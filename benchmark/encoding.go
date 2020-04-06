package benchmark

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/celrenheit/sandflake"
	"github.com/gofrs/uuid"
	"github.com/muyo/sno"
	"github.com/oklog/ulid"
	"github.com/rs/xid"
	"github.com/segmentio/ksuid"
)

func benchmarkEncoding(b *testing.B) {
	println("\n-- Encoding ----------------------------------------------------------------------------------\n")
	b.Run("enc", benchmarkEncode)
	println("\n-- Decoding ----------------------------------------------------------------------------------\n")
	b.Run("dec", benchmarkDecode)
}

func benchmarkEncode(b *testing.B) {
	b.Run("sno", benchmarkEncodeSno)
	b.Run("xid", benchmarkEncodeXid)
	b.Run("snowflake", benchmarkEncodeSnowflake)
	b.Run("sandflake", benchmarkEncodeSandflake)
	b.Run("uuid", benchmarkEncodeUUID)
	b.Run("ulid", benchmarkEncodeULID)
	b.Run("ksuid", benchmarkEncodeKSUID)
}

func benchmarkDecode(b *testing.B) {
	b.Run("sno", benchmarkDecodeSno)
	b.Run("xid", benchmarkDecodeXid)
	b.Run("snowflake", benchmarkDecodeSnowflake)
	b.Run("sandflake", benchmarkDecodeSandflake)
	b.Run("uuid", benchmarkDecodeUUID)
	b.Run("ulid", benchmarkDecodeULID)
	b.Run("ksuid", benchmarkDecodeKSUID)
}

func benchmarkEncodeSno(b *testing.B) {
	id := sno.New(255)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeXid(b *testing.B) {
	id := xid.New()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeSnowflake(b *testing.B) {
	n, _ := snowflake.NewNode(255)
	id := n.Generate()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeSandflake(b *testing.B) {
	var g sandflake.Generator
	id := g.Next()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeUUID(b *testing.B) {
	b.Run("v1", benchmarkEncodeUUIDv1)
	b.Run("v4", benchmarkEncodeUUIDv4)
}

func benchmarkEncodeUUIDv1(b *testing.B) {
	id, _ := uuid.NewV1()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeUUIDv4(b *testing.B) {
	id, _ := uuid.NewV4()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeULID(b *testing.B) {
	id, _ := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkEncodeKSUID(b *testing.B) {
	id, _ := ksuid.NewRandom()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = id.String()
		}
	})
}

func benchmarkDecodeSno(b *testing.B) {
	id := sno.New(255).String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = sno.FromEncodedString(id)
		}
	})
}

func benchmarkDecodeXid(b *testing.B) {
	id := xid.New().String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = xid.FromString(id)
		}
	})
}

func benchmarkDecodeSnowflake(b *testing.B) {
	n, _ := snowflake.NewNode(255)
	id := n.Generate().String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = snowflake.ParseString(id)
		}
	})
}

func benchmarkDecodeSandflake(b *testing.B) {
	var g sandflake.Generator
	id := g.Next().String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = sandflake.Parse(id)
		}
	})
}

func benchmarkDecodeUUID(b *testing.B) {
	b.Run("v1", benchmarkDecodeUUIDv1)
	b.Run("v4", benchmarkDecodeUUIDv4)
}

func benchmarkDecodeUUIDv1(b *testing.B) {
	id, _ := uuid.NewV1()
	s := id.String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = uuid.FromString(s)
		}
	})
}

func benchmarkDecodeUUIDv4(b *testing.B) {
	id, _ := uuid.NewV4()
	s := id.String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = uuid.FromString(s)
		}
	})
}

func benchmarkDecodeULID(b *testing.B) {
	id, _ := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	s := id.String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ulid.Parse(s)
		}
	})
}

func benchmarkDecodeKSUID(b *testing.B) {
	id, _ := ksuid.NewRandom()
	s := id.String()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ksuid.Parse(s)
		}
	})
}
