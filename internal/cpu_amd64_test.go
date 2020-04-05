package internal

import (
	"testing"
)

func testCPU(t *testing.T) {
	t.Run("features", func(t *testing.T) {
		t.Run("real", testCPUReal)
		t.Run("mocked", testCPUMocked)
	})
}

// First tests are run against the real hardware and actual cpuid instruction.
// While we can't reliably assume the availability of the instruction sets,
// at the very least we may catch anomalies when the highest function parameter
// returned is not sane - or when SSE2 instructions are not available where we
// assume they should be.
func testCPUReal(t *testing.T) {
	t.Run("highest-function-parameter-valid", testCPURealMFIValid)
	t.Run("sse-has-base-set", testCPURealSSEHasBaseSet)
	t.Run("has-vector-support-attempt", testCPURealHasVectorSupportAttempt)
}

func testCPURealMFIValid(t *testing.T) {
	eax, _, _, _ := cpuid(0)
	if eax < 1 {
		t.Errorf("expected a non-zero highest function parameter, got [%d]", eax)
	}
}

func testCPURealSSEHasBaseSet(t *testing.T) {
	_, _, _, edx := cpuid(1)
	if (edx & (1 << 26)) == 0 {
		t.Error("expected the SSE2 instruction set to be available, does not appear to be")
	}
}

func testCPURealHasVectorSupportAttempt(t *testing.T) {
	defer func() {
		catch(t, recover(), "")
	}()

	// Note: We don't care about the result as we can't assume to get a 'true'.
	// We only care for this to not panic.
	HasVectorSupport()
}

// Note: Those tests must not run in parallel to any tests that rely
// on real hardware and the actual cpuid implementation (vide enc/dec),
// as the cpuid function gets swapped out for mocks.
func testCPUMocked(t *testing.T) {
	cpuid = cpu.id

	t.Run("highest-function-parameter-invalid", testCPUHasVectorSupportMFIInvalid)
	t.Run("highest-function-parameter-too-low", testCPUHasVectorSupportMFILow)
	t.Run("sse-lacks-base-set", testCPUHasVectorSupportSSELacksBaseSet)
	t.Run("sse-lacks-extended-sets", testCPUHasVectorSupportSSELacksExtendedSets)
	t.Run("bmi-lacks-sets", testCPUHasVectorSupportBMILacksSets)
	t.Run("passes", testCPUHasVectorPasses)

	// Restore real implementation.
	cpuid = cpuidReal
}

func testCPUHasVectorSupportMFIInvalid(t *testing.T) {
	defer func() {
		catch(t, recover(), cpuLacksSSE2ErrMsg)
	}()

	cpu.reset()
	cpu.eax = 0
	expectVectorSupport(t, false)
}

func testCPUHasVectorSupportMFILow(t *testing.T) {
	defer func() {
		catch(t, recover(), "")
	}()

	cpu.reset()
	cpu.eax = 6
	expectVectorSupport(t, false)
}

func testCPUHasVectorSupportSSELacksBaseSet(t *testing.T) {
	defer func() {
		catch(t, recover(), cpuLacksSSE2ErrMsg)
	}()

	cpu.reset()
	cpu.edx ^= 1 << 26 // SSE2 is featured as 1 << 26, so we simply set everything *but*.
	expectVectorSupport(t, false)
}

func testCPUHasVectorSupportSSELacksExtendedSets(t *testing.T) {
	defer func() {
		catch(t, recover(), "")
	}()

	for _, c := range []struct {
		name string
		mask uint32
	}{
		{"SSE3", ^uint32(0x00000001)},
		{"SSSE3", ^uint32(0x00000200)},
		{"SSE4", ^uint32(0x00080000)},
		{"SSE4.2", ^uint32(0x00100000)},
	} {
		t.Run(c.name, func(t *testing.T) {
			cpu.reset()
			cpu.ecx = c.mask
			expectVectorSupport(t, false)
		})
	}
}

func testCPUHasVectorSupportBMILacksSets(t *testing.T) {
	defer func() {
		catch(t, recover(), "")
	}()

	for _, c := range []struct {
		name string
		mask uint32
	}{
		{"BMI1", ^uint32(0x00000008)},
		{"BMI2", ^uint32(0x00000100)},
	} {
		t.Run(c.name, func(t *testing.T) {
			cpu.reset()
			cpu.ebx = c.mask
			expectVectorSupport(t, false)
		})
	}
}

func testCPUHasVectorPasses(t *testing.T) {
	defer func() {
		catch(t, recover(), "")
	}()

	cpu.reset()
	expectVectorSupport(t, true)
}

var cpu = func() *cpuMock {
	c := &cpuMock{}
	c.reset()

	return c
}()

type cpuMock struct {
	eax, ebx, ecx, edx uint32
}

func (c *cpuMock) reset() {
	c.eax = 7
	c.ebx = 0x00000108
	c.ecx = 0x00180201
	c.edx = 1 << 26
}

func (c *cpuMock) id(_ uint32) (eax, ebx, ecx, edx uint32) {
	return c.eax, c.ebx, c.ecx, c.edx
}

func catch(t *testing.T, err interface{}, expected string) {
	if expected != "" {
		if err == nil {
			t.Fatalf("expected a panic with message [%s]", expected)
		}

		if err != expected {
			t.Errorf("expected a panic with message [%s], got [%s]", expected, err)
		}

		return
	}

	if err != nil {
		t.Fatalf("expected to not panic, panicked with [%s]", err)
	}
}

func expectVectorSupport(t *testing.T, expected bool) {
	if actual := HasVectorSupport(); actual != expected {
		t.Errorf("expected [%t], got [%t]", expected, actual)
	}
}
