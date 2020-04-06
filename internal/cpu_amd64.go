package internal

const cpuLacksSSE2ErrMsg = "sno: CPU does not seem to support SSE2 instructions required on amd64 platforms"

func checkVectorSupport() bool {
	// We need a highest function parameter of at least 7 since we need
	// to check for BMI2 support as well.
	eax, _, _, _ := cpuid(0)
	if eax < 7 {
		if eax < 1 {
			panic(cpuLacksSSE2ErrMsg)
		}

		return false
	}

	_, _, ecx, edx := cpuid(1)
	if (edx & (1 << 26)) == 0 {
		panic(cpuLacksSSE2ErrMsg)
	}

	// c & 0x00000001 -> SSE3
	// c & 0x00000200 -> SSSE3
	// c & 0x00080000 -> SSE4
	// c & 0x00100000 -> SSE4.2
	if (ecx & 0x00180201) != 0x00180201 {
		return false
	}

	// b & 0x00000008 -> BMI1
	// b & 0x00000100 -> BMI2
	_, ebx, _, _ := cpuid(7)

	return (ebx & 0x00000108) == 0x00000108
}

// Gets temporarily swapped out with a mock during tests.
var cpuid = cpuidReal

//go:noescape
func cpuidReal(op uint32) (eax, ebx, ecx, edx uint32)
