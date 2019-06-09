// Package sno provides fast generators of compact, sortable, unique IDs with embedded metadata.
package sno

import (
	"crypto/rand"
	"fmt"
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
	partition Partition // Immutable.

	drifts     *uint32     // Uses the LSB for the tick-tock and serves as a counter.
	wallHi     *int64      //
	wallSafe   *int64      //
	regression *sync.Mutex // Regression branch lock.

	seq       *uint32
	seqMin    uint32  // Immutable.
	seqMax    uint32  // Immutable.
	seqStatic *uint32 // See NewWithTime. Not included in snapshots (does not get restored).

	seqOverflowCond   *sync.Cond
	seqOverflowTicker *time.Ticker
	seqOverflowCount  uint32 // Behind seqOverflowCond lock.
	seqOverflowChan   chan<- *SequenceOverflowNotification

	clock func() (wall int64)
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

	if snapshot.WallSafe != 0 {
		wallNow := nanotime()
		// Since we can't currently infer whether it'd be safe to simply resume (e.g. whether
		// the snapshot got taken during a simple drift - a single regression, or while contenders
		// were sleeping during a multi-regression), we take the safe route out and set wallHi so
		// that the multi-regression branch will get triggered until WallSafe.
		if snapshot.WallSafe > wallNow {
			snapshot.WallHi = wallNow
		}
	}

	seq := uint32(snapshot.Sequence)
	// Offset by -1 because we start 0-indexed, while AddUint gets called first in NewWithTime()
	// and unlike New(), NewWithTime() has no time progression branch that would catch the first
	// generation and reset the sequence along with time.
	seqStatic := uint32(snapshot.SequenceMin - 1)

	return &Generator{
		partition:       snapshot.Partition,
		seq:             &seq,
		seqMin:          uint32(snapshot.SequenceMin),
		seqMax:          uint32(snapshot.SequenceMax),
		seqStatic:       &seqStatic,
		seqOverflowCond: sync.NewCond(&sync.Mutex{}),
		seqOverflowChan: c,
		drifts:          &snapshot.Drifts,
		wallHi:          &snapshot.WallHi,
		wallSafe:        &snapshot.WallSafe,
		regression:      &sync.Mutex{},
		clock:           nanotime,
	}, nil
}

func newGeneratorFromDefaults(c chan<- *SequenceOverflowNotification) (*Generator, error) {
	var (
		err error
		p   Partition
	)

	if p, err = genPartition(); err != nil {
		return nil, err
	}

	seqStatic := ^uint32(0) // Offset to -1, see note in newGeneratorFromSnapshot

	return &Generator{
		partition:       p,
		seq:             new(uint32),
		seqMin:          0,
		seqMax:          MaxSequence,
		seqStatic:       &seqStatic,
		seqOverflowCond: sync.NewCond(&sync.Mutex{}),
		seqOverflowChan: c,
		drifts:          new(uint32),
		wallHi:          new(int64),
		wallSafe:        new(int64),
		regression:      &sync.Mutex{},
		clock:           nanotime,
	}, nil
}

// New generates a new ID using the current system time for its timestamp.
func (g *Generator) New(meta byte) (id ID) {
	var (
		// Note: Single load of wallHi for the evaluations is correct (as wallNow is static).
		wallHi  = atomic.LoadInt64(g.wallHi)
		wallNow = g.clock()
	)

	// Fastest branch if we're still in the same timeframe.
	if wallHi == wallNow {
		seq := atomic.AddUint32(g.seq, 1)

		if g.seqMax >= seq {
			g.applyTimestamp(&id, wallNow)
			g.applyTickTock(&id, atomic.LoadUint32(g.drifts))
			g.applyPartition(&id, meta)
			g.applySequence(&id, seq)

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
	if wallNow > wallHi && atomic.CompareAndSwapInt64(g.wallHi, wallHi, wallNow) {
		atomic.StoreUint32(g.seq, g.seqMin)

		g.applyTimestamp(&id, wallNow)
		g.applyTickTock(&id, atomic.LoadUint32(g.drifts))
		g.applyPartition(&id, meta)
		g.applySequence(&id, g.seqMin)

		return
	}

	// Time regression branch.
	g.regression.Lock()

	// Check-again. It's possible that another thread applied the drift while we were spinning (if we were).
	if wallHi = atomic.LoadInt64(g.wallHi); wallNow >= wallHi {
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
		atomic.StoreInt64(g.wallSafe, wallHi)
		atomic.StoreInt64(g.wallHi, wallNow)
		atomic.StoreUint32(g.seq, g.seqMin)

		g.applyTimestamp(&id, wallNow)
		g.applyTickTock(&id, atomic.AddUint32(g.drifts, 1))
		g.applyPartition(&id, meta)
		g.applySequence(&id, g.seqMin)

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

	g.applyTimestamp(&id, (t.UnixNano()-epochNsec)/TimeUnit)
	g.applyPartition(&id, meta)
	g.applySequence(&id, seq)

	return
}

// Partition returns the fixed identifier of the Generator.
func (g *Generator) Partition() Partition {
	return g.partition
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
	if wallNow := g.clock(); wallNow == atomic.LoadInt64(g.wallHi) {
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
	if wallNow := g.clock(); wallNow == atomic.LoadInt64(g.wallHi) {
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
		wallHi  = atomic.LoadInt64(g.wallHi)
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
		Partition:   g.partition,
		SequenceMin: uint16(g.seqMin),
		SequenceMax: uint16(g.seqMax),
		Sequence:    seq,
		Now:         wallNow,
		WallHi:      wallHi,
		WallSafe:    atomic.LoadInt64(g.wallSafe),
		Drifts:      atomic.LoadUint32(g.drifts),
	}
}

func (g *Generator) applyTimestamp(id *ID, units int64) {
	id[0] = byte(units >> 31)
	id[1] = byte(units >> 23)
	id[2] = byte(units >> 15)
	id[3] = byte(units >> 7)
	id[4] = byte(units << 1)
}

func (g *Generator) applyTickTock(id *ID, counter uint32) {
	id[4] |= byte(counter & 1)
}

func (g *Generator) applyPartition(id *ID, meta byte) {
	id[5] = meta
	id[6] = g.partition[0]
	id[7] = g.partition[1]
}

func (g *Generator) applySequence(id *ID, seq uint32) {
	id[8] = byte(seq >> 8)
	id[9] = byte(seq)
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
			wallNow = (t.UnixNano() - epochNsec) / TimeUnit
			wallHi  = atomic.LoadInt64(g.wallHi)
		)

		if wallNow > wallHi {
			atomic.StoreUint32(g.seq, g.seqMin)
			g.seqOverflowCond.Broadcast()

			continue // Left for readability of flow.
		}
	}
}

func genPartition() (p Partition, err error) {
	if _, err := rand.Read(p[:]); err != nil {
		return p, fmt.Errorf("sno: failed to generate a random partition: %v", err)
	}

	return
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
