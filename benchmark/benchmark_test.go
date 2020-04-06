package benchmark

import (
	"testing"
)

func Benchmark(b *testing.B) {
	b.Run("generation", benchmarkGeneration)
	b.Run("encoding", benchmarkEncoding)
}
