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
	// Intel(R) Core(TM) i5-9600K CPU got 0, 0, 0, 0
	if mfi < 7 && mfi > 0 {
		if mfi < 1 {
			// We don't even have basic sets.
			panic(cpuLacksSSE2ErrMsg)
		}

		return false
	}

	_, _, ecx, edx := cpuId(1)
	// The fallbacks currently rely on SSE2 - while it's available on just about
	// any modern AMD64 platform, *just in case* it's not, fail loudly and immediately
	// instead of faulting on first encode/decode attempt.
	if (edx & (1 << 26)) == 0 {
		panic(cpuLacksSSE2ErrMsg)
	}

	// ecx & 0x00000001 -> SSE3
	// ecx & 0x00000200 -> SSSE3
	// ecx & 0x00080000 -> SSE4
	// ecx & 0x00100000 -> SSE4.2
	if (ecx & 0x00180201) == 0 {
		return false
	}

	// ebx & 0x00000008 -> BMI1
	// ebx & 0x00000100 -> BMI2
	_, ebx, _, _ := cpuId(7)
	if (ebx & 0x00000108) == 0 {
		return false
	}

	return true
}()
