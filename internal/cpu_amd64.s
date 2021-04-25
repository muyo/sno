#include "textflag.h"
#include "funcdata.h"

// func cpuidReal(op uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuidReal(SB), NOSPLIT, $0-24
    MOVL op+0(FP), AX
    XORQ CX, CX
    CPUID
    MOVL AX, eax+8(FP)
    MOVL BX, ebx+12(FP)
    MOVL CX, ecx+16(FP)
    MOVL DX, edx+20(FP)
    RET
