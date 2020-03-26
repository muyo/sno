// Package sno provides fast generators of compact, sortable, unique IDs with embedded metadata.
package sno

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"
)

// GeneratorSnapshot represents the bookkeeping data of a Generator at some point in time.
//
// Snapshots serve both as configuration and a means of restoring generators across restarts,
// to ensure newly generated IDs don't overwrite IDs generated before going offline.
type GeneratorSnapshot struct {
	// The Partition the generator is scoped to. A zero value ({0, 0}) is valid and will be used.
	Partition Partition `json:"partition"`

	// Sequence pool bounds (inclusive). Can be given in either order - lower value will become lower bound.
	// When SequenceMax is 0 and SequenceMin != 65535, SequenceMax will be set to 65535.
	SequenceMin uint16 `json:"sequenceMin"`
	SequenceMax uint16 `json:"sequenceMax"`

	// Current sequence number. When 0, it will be set to SequenceMin. May overflow SequenceMax,
	// but not underflow SequenceMin.
	Sequence uint32 `json:"sequence"`

	Now      int64  `json:"now"`      // Wall time the snapshot was taken at in sno time units and in our epoch.
	WallHi   int64  `json:"wallHi"`   //
	WallSafe int64  `json:"wallSafe"` //
	Drifts   uint32 `json:"drifts"`   // Count of wall clock regressions the generator tick-tocked at.
}

// Partition represents the fixed identifier of a Generator.
//
// If you'd rather define Partitions as integers instead of as byte arrays, then:
//	var p sno.Partition
//	p.PutUint16(65535)
type Partition [2]byte

// AsUint16 returns the Partition as a uint16.
func (p Partition) AsUint16() uint16 {
	return uint16(p[0])<<8 | uint16(p[1])
}

// PutUint16 sets Partition to the given uint16 in big-endian order.
func (p *Partition) PutUint16(u uint16) {
	p[0] = byte(u >> 8)
	p[1] = byte(u)
}

// SequenceOverflowNotification contains information pertaining to the current state of a Generator
// while it is overflowing.
type SequenceOverflowNotification struct {
	Now   time.Time // Time of tick.
	Count uint32    // Number of currently overflowing generation calls.
	Ticks uint32    // Total count of ticks while dealing with the *current* overflow.
}

// Generator is responsible for generating new IDs scoped to a given fixed Partition and
// managing their sequence.
type Generator struct {
	partition uint32 // Immutable.

	drifts     *uint32     // Uses the LSB for the tick-tock and serves as a counter.
	wallHi     *uint64     //
	wallSafe   *uint64     //
	regression *sync.Mutex // Regression branch lock.

	seq       *uint32
	seqMin    uint32  // Immutable.
	seqMax    uint32  // Immutable.
	seqStatic *uint32 // See NewWithTime. Not included in snapshots (does not get restored).

	seqOverflowCond   *sync.Cond
	seqOverflowTicker *time.Ticker
	seqOverflowCount  uint32 // Behind seqOverflowCond lock.
	seqOverflowChan   chan<- *SequenceOverflowNotification

	clock func() uint64
}

// NewGenerator returns a new generator based on the optional Snapshot.
func NewGenerator(snapshot *GeneratorSnapshot, c chan<- *SequenceOverflowNotification) (*Generator, error) {
	if snapshot != nil {
		return newGeneratorFromSnapshot(*snapshot, c)
	}

	return newGeneratorFromDefaults(c)
}

func newGeneratorFromSnapshot(snapshot GeneratorSnapshot, c chan<- *SequenceOverflowNotification) (*Generator, error) {
	if err := sanitizeSnapshotBounds(&snapshot); err != nil {
		return nil, err
	}

	var (
		wallHi   = uint64(snapshot.WallHi)
		wallSafe = uint64(snapshot.WallSafe)
	)

	if wallSafe != 0 {
		wallNow := nanotime()
		// Since we can't currently infer whether it'd be safe to simply resume (e.g. whether
		// the snapshot got taken during a simple drift - a single regression, or while contenders
		// were sleeping during a multi-regression), we take the safe route out and set wallHi so
		// that the multi-regression branch will get triggered until WallSafe.
		if wallSafe > wallNow {
			wallHi = wallNow
		}
	}

	seq := snapshot.Sequence
	// Offset by -1 because we start 0-indexed, while AddUint gets called first in NewWithTime()
	// and unlike New(), NewWithTime() has no time progression branch that would catch the first
	// generation and reset the sequence along with time.
	seqStatic := uint32(snapshot.SequenceMin - 1)

	return &Generator{
		partition:       partitionToInternalRepr(snapshot.Partition),
		seq:             &seq,
		seqMin:          uint32(snapshot.SequenceMin),
		seqMax:          uint32(snapshot.SequenceMax),
		seqStatic:       &seqStatic,
		seqOverflowCond: sync.NewCond(&sync.Mutex{}),
		seqOverflowChan: c,
		drifts:          &snapshot.Drifts,
		wallHi:          &wallHi,
		wallSafe:        &wallSafe,
		regression:      &sync.Mutex{},
		clock:           nanotime,
	}, nil
}

func newGeneratorFromDefaults(c chan<- *SequenceOverflowNotification) (*Generator, error) {
	// Realistically safe, but has an edge case resulting in PartitionPoolExhaustedError.
	partition, err := genPartition()
	if err != nil {
		return nil, err
	}

	// Offset to -1 (see note in newGeneratorFromSnapshot).
	seqStatic := ^uint32(0)

	return &Generator{
		partition:       partition,
		seq:             new(uint32),
		seqMin:          0,
		seqMax:          MaxSequence,
		seqStatic:       &seqStatic,
		seqOverflowCond: sync.NewCond(&sync.Mutex{}),
		seqOverflowChan: c,
		drifts:          new(uint32),
		wallHi:          new(uint64),
		wallSafe:        new(uint64),
		regression:      &sync.Mutex{},
		clock:           nanotime,
	}, nil
}

// New generates a new ID using the current system time for its timestamp.
func (g *Generator) New(meta byte) (id ID) {
	var (
		// Note: Single load of wallHi for the evaluations is correct (as wallNow is static).
		wallHi  = atomic.LoadUint64(g.wallHi)
		wallNow = g.clock()
	)

	// Fastest branch if we're still in the same timeframe.
	if wallHi == wallNow {
		seq := atomic.AddUint32(g.seq, 1)

		if g.seqMax >= seq {
			g.applyTimestamp(&id, wallNow, atomic.LoadUint32(g.drifts)&1)
			g.applyPayload(&id, meta, seq)

			return
		}

		// This is to be considered an edge case if seqMax actually gets exceeded, but since bounds
		// can be set arbitrarily, in a small pool (or in stress tests) this can happen.
		// We don't *really* handle this gracefully - we currently clog up and wait until the sequence
		// gets reset by a time change *hoping* we'll finally get our turn. If requests to generate
		// don't decrease enough, eventually this will starve out resources.
		//
		// The reason we don't simply plug the broadcast into the time progression branch is precisely
		// because that one is going to be the most common branch for many uses realistically (1 or 0 ID per 4msec)
		// while this one is for scales on another level. At the same time if we *ever* hit this case, we need
		// a periodic flush anyways, because even a single threaded process can easily exhaust the max default
		// sequence pool, let alone a smaller one, meaning it could potentially deadlock if all routines get
		// locked in on a sequence overflow and no new routine comes to their rescue at a higher time to reset
		// the sequence and notify them.
		g.seqOverflowCond.L.Lock()
		g.seqOverflowCount++

		if g.seqOverflowTicker == nil {
			g.seqOverflowTicker = time.NewTicker(tickRate)
			go g.seqOverflowLoop()
		}

		for atomic.LoadUint32(g.seq) > g.seqMax {
			// We spin pessimistically here instead of a straight lock -> wait -> unlock because that'd
			// put us back on the New(). At extreme contention we could end up back here anyways.
			g.seqOverflowCond.Wait()
		}

		g.seqOverflowCount--
		g.seqOverflowCond.L.Unlock()

		return g.New(meta)
	}

	// Time progression branch.
	if wallNow > wallHi && atomic.CompareAndSwapUint64(g.wallHi, wallHi, wallNow) {
		atomic.StoreUint32(g.seq, g.seqMin)

		g.applyTimestamp(&id, wallNow, atomic.LoadUint32(g.drifts)&1)
		g.applyPayload(&id, meta, g.seqMin)

		return
	}

	// Time regression branch.
	g.regression.Lock()

	// Check-again. It's possible that another thread applied the drift while we were spinning (if we were).
	if wallHi = atomic.LoadUint64(g.wallHi); wallNow >= wallHi {
		g.regression.Unlock()

		return g.New(meta)
	}

	wallSafe := *g.wallSafe
	if wallNow > wallSafe {
		// Branch for the one routine that gets to apply the drift.
		// wallHi is bidirectional (gets updated whenever the wall clock time progresses - or when a drift
		// gets applied, which is when it regresses). In contrast, wallSafe only ever gets updated when
		// a drift gets applied and always gets set to the highest time recorded, meaning it
		// increases monotonically.
		atomic.StoreUint64(g.wallSafe, wallHi)
		atomic.StoreUint64(g.wallHi, wallNow)
		atomic.StoreUint32(g.seq, g.seqMin)

		g.applyTimestamp(&id, wallNow, atomic.AddUint32(g.drifts, 1)&1)
		g.applyPayload(&id, meta, g.seqMin)

		g.regression.Unlock()

		return id
	}

	// Branch for all routines that are in an "unsafe" past (e.g. multiple time regressions happened
	// before we reached wallSafe again).
	g.regression.Unlock()

	time.Sleep(time.Duration(wallSafe - wallNow))

	return g.New(meta)
}

// NewWithTime generates a new ID using the given time for the timestamp.
//
// IDs generated with user-specified timestamps are exempt from the tick-tock mechanism and
// use a sequence separate from New() - one that is independent from time, as time provided to
// this method can be arbitrary. The sequence increases strictly monotonically up to hitting
// the generator's SequenceMax, after which it rolls over silently back to SequenceMin.
//
// That means bounds are respected, but unlike New(), NewWithTime() will not block the caller
// when the (separate) sequence rolls over as the Generator would be unable to determine when
// to resume processing within the constraints of this method.
//
// Managing potential collisions due to the arbitrary time is left to the user.
//
// This utility is primarily meant to enable porting of old IDs to sno and assumed to be ran
// before an ID scheme goes online.
func (g *Generator) NewWithTime(meta byte, t time.Time) (id ID) {
	var seq = atomic.AddUint32(g.seqStatic, 1)

	if seq > g.seqMax {
		if !atomic.CompareAndSwapUint32(g.seqStatic, seq, g.seqMin) {
			return g.NewWithTime(meta, t)
		}

		seq = g.seqMin
	}

	g.applyTimestamp(&id, uint64(t.UnixNano()-epochNsec)/TimeUnit, 0)
	g.applyPayload(&id, meta, seq)

	return
}

// Partition returns the fixed identifier of the Generator.
func (g *Generator) Partition() Partition {
	return partitionToPublicRepr(g.partition)
}

// Sequence returns the current sequence the Generator is at.
//
// This does *not* mean that if one were to call New() right now, the generated ID
// will necessarily get this sequence, as other things may happen before.
//
// If the next call to New() would result in a reset of the sequence, SequenceMin
// is returned instead of the current internal sequence.
//
// If the generator is currently overflowing, the sequence returned will be higher than
// the generator's SequenceMax (thus a uint32 return type), meaning it can be used to
// determine the current overflow via:
//	overflow := int(uint32(generator.SequenceMax()) - generator.Sequence())
func (g *Generator) Sequence() uint32 {
	if wallNow := g.clock(); wallNow == atomic.LoadUint64(g.wallHi) {
		return atomic.LoadUint32(g.seq)
	}

	return g.seqMin
}

// SequenceMin returns the lower bound of the sequence pool of this generator.
func (g *Generator) SequenceMin() uint16 {
	return uint16(g.seqMin)
}

// SequenceMax returns the upper bound of the sequence pool of this generator.
func (g *Generator) SequenceMax() uint16 {
	return uint16(g.seqMax)
}

// Len returns the number of IDs generated in the current timeframe.
func (g *Generator) Len() int {
	if wallNow := g.clock(); wallNow == atomic.LoadUint64(g.wallHi) {
		if seq := atomic.LoadUint32(g.seq); g.seqMax > seq {
			return int(seq-g.seqMin) + 1
		}

		return g.Cap()
	}

	return 0
}

// Cap returns the total capacity of the Generator.
//
// To get its current capacity (e.g. number of possible additional IDs in the current
// timeframe), simply:
// 	spare := generator.Cap() - generator.Len()
// The result will always be non-negative.
func (g *Generator) Cap() int {
	return int(g.seqMax-g.seqMin) + 1
}

// Snapshot returns a copy of the Generator's current bookkeeping data.
func (g *Generator) Snapshot() GeneratorSnapshot {
	var (
		wallNow = g.clock()
		wallHi  = atomic.LoadUint64(g.wallHi)
		seq     uint32
	)

	// Be consistent with g.Sequence() and return seqMin if the next call to New()
	// would reset the sequence.
	if wallNow == wallHi {
		seq = atomic.LoadUint32(g.seq)
	} else {
		seq = g.seqMin
	}

	return GeneratorSnapshot{
		Partition:   partitionToPublicRepr(g.partition),
		SequenceMin: uint16(g.seqMin),
		SequenceMax: uint16(g.seqMax),
		Sequence:    seq,
		Now:         int64(wallNow),
		WallHi:      int64(wallHi),
		WallSafe:    int64(atomic.LoadUint64(g.wallSafe)),
		Drifts:      atomic.LoadUint32(g.drifts),
	}
}

func (g *Generator) applyTimestamp(id *ID, units uint64, tick uint32) {
	// Equivalent to...
	//
	//	id[0] = byte(units >> 31)
	//	id[1] = byte(units >> 23)
	//	id[2] = byte(units >> 15)
	//	id[3] = byte(units >> 7)
	//	id[4] = byte(units << 1) | byte(tick)
	//
	// ... and slightly wasteful as we're storing 3 bytes that will get overwritten
	// via applyPartition but unlike the code above, the calls to binary.BigEndian.PutUintXX()
	// are compiler assisted and boil down to essentially a load + shift + bswap (+ a nop due
	// to midstack inlining), which we prefer over the roughly 16 instructions otherwise.
	// If applyTimestamp() was implemented straight in assembly, we'd not get it inline.
	binary.BigEndian.PutUint64(id[:], units<<25|uint64(tick)<<24)
}

func (g *Generator) applyPayload(id *ID, meta byte, seq uint32) {
	id[5] = meta
	binary.BigEndian.PutUint32(id[6:], g.partition|seq)
}

func (g *Generator) seqOverflowLoop() {
	var (
		retryNotify bool
		ticks       uint32
	)

	for t := range g.seqOverflowTicker.C {
		g.seqOverflowCond.L.Lock()

		if g.seqOverflowChan != nil {
			// We only ever count ticks when we've got a notification channel up.
			// Even if we're at a count of 0 but on our first tick, it means the generator declogged already,
			// but we still notify that it happened.
			ticks++
			if retryNotify || g.seqOverflowCount == 0 || ticks%4 == 1 {
				select {
				case g.seqOverflowChan <- &SequenceOverflowNotification{
					Now:   t,
					Ticks: ticks,
					Count: g.seqOverflowCount,
				}:
					retryNotify = false

				default:
					// Simply drop the message for now but try again the next tick already
					// instead of waiting for the full interval.
					retryNotify = true
				}
			}
		}

		if g.seqOverflowCount == 0 {
			g.seqOverflowTicker.Stop()
			g.seqOverflowTicker = nil
			g.seqOverflowCond.L.Unlock()

			return
		}

		// At this point we can unlock already because we don't touch any shared data anymore.
		// The broadcasts further don't require us to hold the lock.
		g.seqOverflowCond.L.Unlock()

		// Under normal behaviour high load would trigger an overflow and load would remain roughly
		// steady, so a seq reset will simply get triggered by a time change happening in New().
		// The actual callers are in a pessimistic loop and will check the condition themselves again.
		if g.seqMax >= atomic.LoadUint32(g.seq) {
			g.seqOverflowCond.Broadcast()

			continue
		}

		// Handles an edge case where we've got calls locked on an overflow and suddenly no more
		// calls to New() come in, meaning there's no one to actually reset the sequence.
		var (
			wallNow = uint64(t.UnixNano()-epochNsec) / TimeUnit
			wallHi  = atomic.LoadUint64(g.wallHi)
		)

		if wallNow > wallHi {
			atomic.StoreUint32(g.seq, g.seqMin)
			g.seqOverflowCond.Broadcast()

			continue // Left for readability of flow.
		}
	}
}

// genPartition generates a Partition in its internal representation from a time based seed.
//
// While this alone would be enough if we only used this once (for the global generator),
// generators created with the default configuration also use generated partitions - a case
// for which we want to avoid collisions, at the very least within our process.
//
// Considering we only have a tiny period of 2**16 available, and that predictability of
// the partitions is a non-factor, using even a 16-bit Xorshift PRNG would be overkill.
//
// If we used a PRNG without adjustment, we'd have the following pitfalls:
// - we'd need to maintain its state and synchronize access to it. As it can't run atomically,
//   this would require maintaining a global lock separately;
// - if we wanted to avoid that and simply use a different seed
// - our space is limited to barely 65535 partitions, making collisions quite likely
//   and we have no way of determining them without maintaining yet additional state,
//   at the very least as a bit set (potentially growing to 8192 bytes for the entire
//   space). It'd also need to be synchronized. With collisions becoming more and
//   and more likely as we hand out partitions, we'd need a means of determining free
//   partitions in the set to be efficient.
//
// And others. At which point the complexity becomes unreasonable for what we're aiming
// to do, so instead of all of that, we go back to the fact that predictability is a non-factor
// and our goal being only the prevention of collisions, so we simply simply start off with
// a time based seed... which we then atomically increment.
//
// This way access is safely synchronized and we're guaranteed to get 65535 partitions
// without collisions in-process with just a tiny bit of code in comparison.
//
// Should we ever exceed that number, we however panic. If your usage pattern is weird enough
// to hit this edge case, please consider managing the partition space yourself and starting
// the Generators using configuration snapshots, instead.
//
// Note: This being entirely predictable has the upside that the order of creation and the count
// of in-process generators created without snapshots can be simply inferred by comparing their
// partitions (including comparing to the global generator, which starts at 0 - eg. at the seed).
func genPartition() (uint32, error) {
	n := atomic.AddUint32(&gen, 1)

	if n > maxPartition {
		return 0, &PartitionPoolExhaustedError{}
	}

	// Convert to our internal representation leaving 2 bytes empty
	// for the sequence to simply get ORed at runtime.
	return uint32(seed+uint16(n)) << 16, nil
}

var (
	// Counter starts at -1 since genPartition() will increase it on each call, including
	// the first. This means the global generator gets an N of 0 and always has a Partition = seed.
	gen  = ^uint32(0)
	seed = func() uint16 {
		_, wallNsec, _ := now()

		// Note: it's fine if this ends up being a 0, as that is still a valid partition.
		return uint16((wallNsec >> 16) ^ wallNsec)
	}()
)

func partitionToInternalRepr(p Partition) uint32 {
	return uint32(p[0])<<24 | uint32(p[1])<<16
}

func partitionToPublicRepr(p uint32) Partition {
	return Partition{byte(p >> 24), byte(p >> 16)}
}

func sanitizeSnapshotBounds(s *GeneratorSnapshot) error {
	// Zero value of SequenceMax will pass as the default max if and only if SequenceMin is not already
	// default max (as the range can be defined in either order).
	if s.SequenceMax == 0 && s.SequenceMin != MaxSequence {
		s.SequenceMax = MaxSequence
	}

	if s.SequenceMin == s.SequenceMax {
		return invalidSequenceBounds(s, errSequenceBoundsIdenticalMsg)
	}

	// Allow bounds to be given in any order.
	if s.SequenceMax < s.SequenceMin {
		s.SequenceMin, s.SequenceMax = s.SequenceMax, s.SequenceMin
	}

	if s.SequenceMax-s.SequenceMin-1 < minSequencePoolSize {
		return invalidSequenceBounds(s, errSequencePoolTooSmallMsg)
	}

	// Allow zero value to pass as a default of the lower bound.
	if s.Sequence == 0 {
		s.Sequence = uint32(s.SequenceMin)
	}

	if s.Sequence < uint32(s.SequenceMin) {
		return invalidSequenceBounds(s, errSequenceUnderflowsBound)
	}

	return nil
}

func invalidSequenceBounds(s *GeneratorSnapshot, msg string) *InvalidSequenceBoundsError {
	return &InvalidSequenceBoundsError{
		Cur: s.Sequence,
		Min: s.SequenceMin,
		Max: s.SequenceMax,
		Msg: msg,
	}
}
