package benchmark

import (
	crand "crypto/rand"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/celrenheit/sandflake"
	"github.com/gofrs/uuid"
	"github.com/lucsky/cuid"
	"github.com/muyo/sno"
	"github.com/oklog/ulid"
	"github.com/rs/xid"
	"github.com/segmentio/ksuid"
	"github.com/sony/sonyflake"
)

func benchmarkGeneration(b *testing.B) {
	println("\n-- Generation (sequential) -------------------------------------------------------------------\n")
	b.Run("s", benchmarkGenerateSequential)
	println("\n-- Generation (parallel) ---------------------------------------------------------------------\n")
	b.Run("p", benchmarkGenerateParallel)
}

func benchmarkGenerateSequential(b *testing.B) {
	b.Run("sno", benchmarkGenerateSequentialSno)             // Bounded
	b.Run("xid", benchmarkGenerateSequentialXid)             // Unbounded
	b.Run("snowflake", benchmarkGenerateSequentialSnowflake) // Bounded
	b.Run("sonyflake", benchmarkGenerateSequentialSonyflake) // Bounded
	b.Run("sandflake", benchmarkGenerateSequentialSandflake) // Unbounded
	b.Run("cuid", benchmarkGenerateSequentialCuid)           // Unbounded
	b.Run("uuid", benchmarkGenerateSequentialUUID)           // Unbounded
	b.Run("ulid", benchmarkGenerateSequentialULID)           // Unbounded
	b.Run("ksuid", benchmarkGenerateSequentialKSUID)         // Unbounded
}

func benchmarkGenerateParallel(b *testing.B) {
	b.Run("sno", benchmarkGenerateParallelSno)             // Bounded
	b.Run("xid", benchmarkGenerateParallelXid)             // Unbounded
	b.Run("snowflake", benchmarkGenerateParallelSnowflake) // Bounded
	b.Run("sonyflake", benchmarkGenerateParallelSonyflake) // Bounded
	b.Run("sandflake", benchmarkGenerateParallelSandflake) // Unbounded
	b.Run("cuid", benchmarkGenerateParallelCuid)           // Unbounded
	b.Run("uuid", benchmarkGenerateParallelUUID)           // Unbounded
	b.Run("ulid", benchmarkGenerateParallelULID)           // Unbounded
	b.Run("ksuid", benchmarkGenerateParallelKSUID)         // Unbounded
}

func benchmarkGenerateSequentialSno(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = sno.New(255)
	}
}

func benchmarkGenerateSequentialXid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = xid.New()
	}
}

func benchmarkGenerateSequentialSnowflake(b *testing.B) {
	n, _ := snowflake.NewNode(255)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = n.Generate()
	}
}

func benchmarkGenerateSequentialSonyflake(b *testing.B) {
	g := sonyflake.NewSonyflake(sonyflake.Settings{})
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = g.NextID()
	}
}

func benchmarkGenerateSequentialSandflake(b *testing.B) {
	var g sandflake.Generator
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = g.Next()
	}
}

func benchmarkGenerateSequentialCuid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = cuid.New()
	}
}

func benchmarkGenerateSequentialUUID(b *testing.B) {
	b.Run("v1", benchmarkGenerateSequentialUUIDv1)
	b.Run("v4", benchmarkGenerateSequentialUUIDv4)
}

func benchmarkGenerateSequentialUUIDv1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = uuid.NewV1()
	}
}

func benchmarkGenerateSequentialUUIDv4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = uuid.NewV4()
	}
}

// A note about the included ULID runs.
//
// ULIDs generators expect time to be passed in as a timestamp with msec precision. All of the other
// libraries being tested handle time sourcing themselves, which is reflected in their results.
// Therefore the time fetching (via time.Now()) including the unit conversion (via ulid.Timestamp())
// is included in each iteration. If the time had been fetched outside the benchmark loop, the results
// would be roughly 7nsec/op lower (@go 1.14.1, Windows 10, i7 4770k 4.4GHz).
//
// The ULID package benchmarks itself when no entropy source is provided, which in a run resulted
// at 29.8ns/op (relative to unbounded Sno at 8.8ns/op, for reference). However, this test is
// excluded in this benchmark. While it may measure ULID's raw overhead, it does not measure
// a end-user usable case since ULIDs without entropy are essentially a 48bit timestamp and...
// 10 zero bytes, which defeats the purpose of the spec.
func benchmarkGenerateSequentialULID(b *testing.B) {
	b.Run("crypto", benchmarkGenerateSequentialULIDCrypto)
	b.Run("math", benchmarkSequentialNewULIDMath)
}

func benchmarkGenerateSequentialULIDCrypto(b *testing.B) {
	rng := crand.Reader
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ulid.New(ulid.Timestamp(time.Now()), rng)
	}
}

func benchmarkSequentialNewULIDMath(b *testing.B) {
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ulid.New(ulid.Timestamp(time.Now()), rng)
	}
}

func benchmarkGenerateSequentialKSUID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ksuid.NewRandom()
	}
}

func benchmarkGenerateParallelSno(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = sno.New(255)
		}
	})
}

func benchmarkGenerateParallelXid(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = xid.New()
		}
	})
}

func benchmarkGenerateParallelSnowflake(b *testing.B) {
	n, _ := snowflake.NewNode(255)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = n.Generate()
		}
	})
}

func benchmarkGenerateParallelSonyflake(b *testing.B) {
	g := sonyflake.NewSonyflake(sonyflake.Settings{})
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = g.NextID()
		}
	})
}

func benchmarkGenerateParallelSandflake(b *testing.B) {
	var g sandflake.Generator
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.Next()
		}
	})
}

func benchmarkGenerateParallelCuid(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cuid.New()
		}
	})
}

func benchmarkGenerateParallelUUID(b *testing.B) {
	b.Run("v1", benchmarkGenerateParallelUUIDv1)
	b.Run("v4", benchmarkGenerateParallelUUIDv4)
}

func benchmarkGenerateParallelUUIDv1(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = uuid.NewV1()
		}
	})
}

func benchmarkGenerateParallelUUIDv4(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = uuid.NewV4()
		}
	})
}

func benchmarkGenerateParallelULID(b *testing.B) {
	b.Run("crypto", benchmarkGenerateParallelULIDCrypto)
	b.Run("math", benchmarkGenerateParallelULIDMath)
}

func benchmarkGenerateParallelULIDCrypto(b *testing.B) {
	rng := crand.Reader
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ulid.New(ulid.Timestamp(time.Now()), rng)
		}
	})
}

func benchmarkGenerateParallelULIDMath(b *testing.B) {
	// Note: Requires manual locking for this run to complete.
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	mu := sync.Mutex{}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			_, _ = ulid.New(ulid.Timestamp(time.Now()), rng)
			mu.Unlock()
		}
	})
}

func benchmarkGenerateParallelKSUID(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ksuid.NewRandom()
		}
	})
}
