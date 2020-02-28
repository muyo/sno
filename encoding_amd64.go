package sno

//go:noescape
func encode(src *ID) (dst [SizeEncoded]byte)

//go:noescape
func decode(src []byte) (dst ID)

//go:noescape
func cpuId(op uint8) (eax, ebx, ecx, edx uint32)

const cpuLacksSSE2ErrMsg = "sno: CPU does not seem to support SSE2 instructions required on amd64 platforms"

// One-shot to determine whether we've got SSE2 at all, and the SSE4.2 and BMI2 sets
// that we need for the vectorized codecs.
var hasVectorSupport = func() bool {
	mfi, _, _, _ := cpuId(0)

	// Need an mfi of at least 7 since we need to check for BMI2 support as well.
	if mfi < 7 {
		if mfi < 1 {
			// We don't even have basic sets.
			panic(cpuLacksSSE2ErrMsg)
		}

		return false
	}

	_, _, c, d := cpuId(1)
	// The fallbacks currently rely on SSE2 - while it's available on just about
	// any modern AMD64 platform, *just in case* it's not, fail loudly and immediately
	// instead of faulting on first encode/decode attempt.
	if (d & (1 << 26)) == 0 {
		panic(cpuLacksSSE2ErrMsg)
	}

	// c & 0x00000001 -> SSE3
	// c & 0x00000200 -> SSSE3
	// c & 0x00080000 -> SSE4
	// c & 0x00100000 -> SSE4.2
	if (c & 0x00180201) == 0 {
		return false
	}

	// e & 0x00000008 -> BMI1
	// e & 0x00000100 -> BMI2
	_, e, _, _ := cpuId(7)

	return (e & 0x00000108) != 0
}()
