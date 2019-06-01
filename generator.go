package sno

import (
	"crypto/rand"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Partition represents a fixed identifier of the given generator.
type Partition [2]byte

// Generator is responsible for generating new IDs scoped to a given fixed Partition and
// managing their sequence.
type Generator struct {
	partition Partition // Immutable, assigned at object creation.

	// Time bookkeeping.
	drifts     *uint32 // Uses only the LSB for the tick-tock but serves as a counter as well.
	wallHi     *int64
	monoHi     *int64
	monoOffset int64 // Immutable, assigned at object creation.
	regression *sync.Mutex

	// Sequence bookkeeping.
	seq    *uint32 // Uses only the low 16-bits.
	seqMin uint32  // Immutable, assigned at object creation.
	seqMax uint32  // Immutable, assigned at object creation.

	// Sequence overflow.
	seqOverflowCount  uint32 // Behind seqOverflowCond lock.
	seqOverflowCond   *sync.Cond
	seqOverflowTicker *time.Ticker
	seqOverflowChan   chan<- *SequenceOverflowNotification
}

type SequenceOverflowNotification struct {
	Now   time.Time // Time of tick.
	Count uint32    // Number of currently overflowing generation calls.
	Ticks uint32    // For how many ticks in total we've already been dealing with the *current* overflow.
}

// GeneratorSnapshot represents the internal bookkeeping data of a Generator at some point in time.
type GeneratorSnapshot struct {
	Partition   Partition `json:"partition"`
	Sequence    uint16    `json:"sequence"`
	SequenceMin uint16    `json:"sequenceMin"`
	SequenceMax uint16    `json:"sequenceMax"`

	// All fields are exported leaving you full control, but if you manually set any of the below
	// and use them to seed a generator, you should know exactly what you're doing.
	Drifts     uint32 `json:"drifts"`
	Now        int64  `json:"now"`
	WallHi     int64  `json:"wallHi"`
	MonoHi     int64  `json:"monoHi"`
	MonoOffset int64  `json:"monoOffset"`
}

// NewGenerator returns a new generator based on the optional Snapshot.
func NewGenerator(snapshot *GeneratorSnapshot) (*Generator, error) {
	if snapshot == nil {
		var (
			err error
			p   Partition
		)

		if p, err = genPartition(); err != nil {
			return nil, err
		}

		return &Generator{
			partition:       p,
			seq:             new(uint32),
			seqMin:          0,
			seqMax:          MaxSequence,
			seqOverflowCond: sync.NewCond(&sync.Mutex{}),
			drifts:          new(uint32),
			wallHi:          new(int64),
			monoHi:          new(int64),
			regression:      &sync.Mutex{},
		}, nil
	}

	snap := *snapshot

	// We do a very rudimentary comparison on the wall clocks.
	// snapshot.MonoOffset gets sampled along with snapshot.Now. We apply the difference in wall clock times
	// as if monotonic time also changed. If wallNow is behind snapshot.Now, we effectively extend the time it
	// takes until we reach a new safe boundary (monoHi). Otherwise we decrease it, simply adding passed time.
	if snap.Now != 0 {
		wallNow, monoNow := nanotime()
		snap.MonoOffset += wallNow - snap.Now - monoNow
		snap.MonoHi -= monoNow
	}

	if err := sanitizeSnapshotBounds(&snap); err != nil {
		return nil, err
	}

	// We use only the low 16 bits, but sync (@go 1.12) has no exported atomic call for that
	// (and impl would require porting it to all architectures). At the same time don't want to give
	// the impression that seq is higher than it actually is via a exported type like the snapshots,
	// which is why they're shot as uint16.
	seq := uint32(snap.Sequence)

	return &Generator{
		partition:       snap.Partition,
		seq:             &seq,
		seqMin:          uint32(snap.SequenceMin),
		seqMax:          uint32(snap.SequenceMax),
		seqOverflowCond: sync.NewCond(&sync.Mutex{}),
		drifts:          &snap.Drifts,
		wallHi:          &snap.WallHi,
		monoHi:          &snap.MonoHi,
		monoOffset:      snap.MonoOffset,
		regression:      &sync.Mutex{},
	}, nil
}

// New generates a new ID using the current system time for its timestamp.
func (g *Generator) New(meta byte) (id ID) {
	var (
		// Note: Single load of wallHi for the evaluations is correct. This is finicky, but should this thread stall
		// on the branch evaluations due to solar flares or another obscure reason on CPU level, the tick-tock write to
		// g.wallHi inside the regression branch could happen before the evaluation of both conditions,
		// so if both loaded atomically, different values of wallHi could get compared to a static wallNow.
		wallHi           = atomic.LoadInt64(g.wallHi)
		wallNow, monoNow = nanotime()
	)

	if wallHi == wallNow {
		// Fastest branch if we're still in the same timeframe.
		seq := atomic.AddUint32(g.seq, 1)

		// Upper bound is inclusive.
		if g.seqMax >= seq {
			g.setTimestamp(&id, wallNow)
			g.setPartition(&id, meta)

			id[4] |= byte(atomic.LoadUint32(g.drifts) & 1)

			id[8] = byte(seq >> 8)
			id[9] = byte(seq)

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
			// put us back on the New(). At extreme contention we could end up back here anyways
			// since we're essentially clogging up and *hoping* throughput will reduce instead
			// of staying steady. If we went straight to New() in such a scenario we'd have wasted
			// additional cycles on the syscalls to get the updated time and so forth.
			g.seqOverflowCond.Wait()
		}

		g.seqOverflowCount--
		g.seqOverflowCond.L.Unlock()

		// Reset of seq may also mean a backwards drift, so we run from scratch.
		return New(meta)
	}

	// Time progression branch.
	if wallNow > wallHi && atomic.CompareAndSwapInt64(g.wallHi, wallHi, wallNow) {
		atomic.StoreUint32(g.seq, g.seqMin)

		g.setTimestamp(&id, wallNow)
		g.setPartition(&id, meta)

		id[4] |= byte(atomic.LoadUint32(g.drifts) & 1)

		if g.seqMin > 0 {
			id[8] = byte(g.seqMin >> 8)
			id[9] = byte(g.seqMin)
		}

		return
	}

	// Time regression branch.
	g.regression.Lock()

	// Check-again. It's possible that another thread applied the drift while we were spinning (if we were).
	// This way we're avoiding a lock in the fast path outside. This is also the most likely path
	// when a drift happens and a routine happens to do the first check, then lock, before we apply the tick-tock.
	// The edge case is when when the tick-tock does not get applied, e.g. we had more than one time regression
	// before reaching g.hiMono again, in which case this routine will end up sleeping until it's safe
	// to generate again.
	if wallHi = atomic.LoadInt64(g.wallHi); wallNow >= wallHi {
		g.regression.Unlock()

		return New(meta)
	}

	monoNow += g.monoOffset
	monoHi := *g.monoHi // Safe, only write that ever happens to monoHi is inside this branch and we're in a mutex.

	if monoNow > monoHi {
		// Branch for *the one* routine that gets to apply the drift.
		// wallHi - wallNow represents the size of the drift in wall clock time. monoHi becomes a moment
		// in the future of the monotonic clock.
		// Sequence gets reset on drifts as time changed. Every other contender is in a branch
		// that ends in a rerun so they'll pick up all the changes.
		atomic.StoreInt64(g.monoHi, monoNow+wallHi-wallNow)
		atomic.StoreInt64(g.wallHi, wallNow)
		atomic.StoreUint32(g.seq, g.seqMin)

		g.setTimestamp(&id, wallNow)
		g.setPartition(&id, meta)

		id[4] |= byte(atomic.AddUint32(g.drifts, 1) & 1)

		if g.seqMin > 0 {
			id[8] = byte(g.seqMin >> 8)
			id[9] = byte(g.seqMin)
		}

		g.regression.Unlock()

		return id
	}

	// Branch for all routines that are in an "unsafe" past. After sleeping, we retry from scratch
	// as we could've moved backwards in time *again* during our slumber.
	//
	// TODO(alcore) Up for a bench and profile. Not only is there likely a better way to do this
	// than to put all contenders to sleep, intuition also says this needs profiling under extreme
	// contention to check behaviour when all goroutines get woken up - and to see whether we starve
	// anything out. Arbitrarily splitting between spinning on a mutex on short drifts (define short?)
	// and actually going to sleep to let the runtime and OS handle the resources during long drifts,
	// would be an option.
	g.regression.Unlock()

	time.Sleep(time.Duration(monoHi - monoNow))

	return New(meta)
}

// NewWithTime generates a new ID using the given time for the timestamp.
//
// IDs generated with user-specified timestamps are exempt from the tick-tock mechanism (but retain
// the same data layout). Managing potential collisions in their case is left to the user. This utility
// is primarily meant to enable porting of old IDs to sno and assumed to be ran before an ID scheme goes
// online.
func (g *Generator) NewWithTime(meta byte, t time.Time) ID {
	var id ID
	g.setTimestamp(&id, (t.UnixNano()-epochNsec)/TimeUnit)
	g.setPartition(&id, meta)

	// TODO(alcore) Decouple from time-relative sequence?
	seq := atomic.AddUint32(g.seq, 1)
	id[8] = byte(seq >> 8)
	id[9] = byte(seq)

	return id
}

// Partition returns the fixed identifier of the Generator.
func (g *Generator) Partition() Partition {
	return g.partition
}

// Sequence returns the current sequence the Generator is at.
func (g *Generator) Sequence() uint16 {
	return uint16(atomic.LoadUint32(g.seq))
}

// Snapshot returns a snapshot of the Generator's current bookkeeping data.
func (g *Generator) Snapshot() GeneratorSnapshot {
	wallNow, _ := nanotime()

	return GeneratorSnapshot{
		Now:         wallNow,
		Partition:   g.partition,
		Drifts:      atomic.LoadUint32(g.drifts),
		WallHi:      atomic.LoadInt64(g.wallHi),
		MonoHi:      atomic.LoadInt64(g.monoHi),
		MonoOffset:  g.monoOffset,
		Sequence:    uint16(atomic.LoadUint32(g.seq)),
		SequenceMin: uint16(g.seqMin),
		SequenceMax: uint16(g.seqMax),
	}
}

func (g *Generator) setTimestamp(id *ID, units int64) {
	// Entire timestamp is shifted left by one to make room for the tick-toggle at the LSB.
	// Note that we get the time from New() already as our own time units and in our epoch.
	id[0] = byte(units >> 31)
	id[1] = byte(units >> 23)
	id[2] = byte(units >> 15)
	id[3] = byte(units >> 7)
	id[4] = byte(units << 1)
}

func (g *Generator) setPartition(id *ID, meta byte) {
	id[5] = meta
	id[6] = g.partition[0]
	id[7] = g.partition[1]
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
			if retryNotify || ticks%tickRateDiv == 1 {
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

		// This handles an edge case where we've got calls locked on an overflow and suddenly no more
		// calls to Generate() come in, meaning there's no one to actually reset the sequence.
		var (
			wallNow = (epochNsec + t.UnixNano()) / TimeUnit
			wallHi  = atomic.LoadInt64(g.wallHi)
		)

		if wallNow > wallHi && atomic.CompareAndSwapInt64(g.wallHi, wallHi, wallNow) {
			atomic.StoreUint32(g.seq, g.seqMin)
			g.seqOverflowCond.Broadcast()

			continue // Superfluous but left for readability of flow.
		}
	}
}

func genPartition() (p Partition, err error) {
	if _, err := rand.Read(p[:]); err != nil {
		return p, fmt.Errorf("failed to generate a random partition: %v", err)
	}

	return
}

func sanitizeSnapshotBounds(s *GeneratorSnapshot) error {
	// Allow for the zero value of SequenceMax to pass as the default max if and only if SequenceMin
	// is also zero. If the latter is not the case, we'll flip their order and use min as max before
	// checking other reqs.
	if s.SequenceMax == 0 && s.SequenceMin == 0 {
		s.SequenceMax = MaxSequence
	} else if s.SequenceMin == s.SequenceMax {
		return &InvalidGeneratorBoundsError{
			Cur: s.Sequence,
			Min: s.SequenceMin,
			Max: s.SequenceMax,
			msg: "sequence bounds are identical - cannot create a generator with no sequence pool",
		}
	}

	// Allow bounds to be given in any order since we can work with either as long as they
	// meet all the other requirements.
	if s.SequenceMax < s.SequenceMin {
		s.SequenceMin, s.SequenceMax = s.SequenceMax, s.SequenceMin
	}

	if s.SequenceMax-s.SequenceMin < minSequencePoolSize {
		return &InvalidGeneratorBoundsError{
			Cur: s.Sequence,
			Min: s.SequenceMin,
			Max: s.SequenceMax,
			msg: "a generator requires a sequence pool with a capacity of at least 16",
		}
	}

	// Allow zero value to pass as a default of the lower bound.
	if s.Sequence == 0 {
		s.Sequence = s.SequenceMin
	}

	if s.Sequence > s.SequenceMax || s.Sequence < s.SequenceMin {
		return &InvalidGeneratorBoundsError{
			Cur: s.Sequence,
			Min: s.SequenceMin,
			Max: s.SequenceMax,
			msg: "current sequence overflows bounds",
		}
	}

	return nil
}
