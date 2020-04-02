package sno

import "sync/atomic"

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
// - our space is limited to barely 65535 partitions, making collisions quite likely
//   and we have no way of determining them without maintaining yet additional state,
//   at the very least as a bit set (potentially growing to 8192 bytes for the entire
//   space). It'd also need to be synchronized. With collisions becoming more and
//   and more likely as we hand out partitions, we'd need a means of determining free
//   partitions in the set to be efficient.
//
// And others. At which point the complexity becomes unreasonable for what we're aiming
// to do, so instead of all of that, we go back to the fact that predictability is a non-factor
// and our goal being only the prevention of collisions, we simply start off with
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
// partitions (including comparing to the global generator, which starts at 0 - i.e. at the seed).
func genPartition() (uint32, error) {
	n := atomic.AddUint32(&partitions, 1)

	if n > MaxPartition {
		return 0, &PartitionPoolExhaustedError{}
	}

	// Convert to our internal representation leaving 2 bytes empty
	// for the sequence to simply get ORed at runtime.
	return uint32(seed+uint16(n)) << 16, nil
}

var (
	// Counter starts at -1 since genPartition() will increase it on each call, including
	// the first. This means the global generator gets an N of 0 and always has a Partition = seed.
	partitions = ^uint32(0)
	seed       = func() uint16 {
		t := snotime()

		return uint16((t >> 32) ^ t)
	}()
)

func partitionToInternalRepr(p Partition) uint32 {
	return uint32(p[0])<<24 | uint32(p[1])<<16
}

func partitionToPublicRepr(p uint32) Partition {
	return Partition{byte(p >> 24), byte(p >> 16)}
}
