package sno

//go:noescape
func encodeVector(src *ID) [SizeEncoded]byte

//go:noescape
func decodeVector(dst *ID, src []byte)

//go:noescape
func cpuId(op uint8) (eax, ebx, ecx, edx uint32)

func init() {
	if cpuHasVectorSupport() {
		encode = encodeVector
		decode = decodeVector
	}
}

func cpuHasVectorSupport() bool {
	mfi, _, _, _ := cpuId(0)

	// Need an mfi of at least 7 since we need to check for BMI2 support as well.
	if mfi < 7 {
		return false
	}

	_, _, c, d := cpuId(1)

	// The check on d is for SSE2 support.
	// The mask on c is:
	// c & 0x00000001 -> SSE3
	// c & 0x00000200 -> SSSE3
	// c & 0x00080000 -> SSE4
	// c & 0x00100000 -> SSE4.2
	if (d&(1<<26)) == 0 || (c&0x00180201) == 0 {
		return false
	}

	// 0x00000008 = BMI1
	// 0x00000100 = BMI2
	_, e, _, _ := cpuId(7)
	if (e & 0x00000108) == 0 {
		return false
	}

	return true
}
