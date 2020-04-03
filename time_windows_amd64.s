#include "textflag.h"
#include "funcdata.h"

// Uses the same approach as Go's runtime to get the current OS time as documented on:
// https://www.dcl.hpi.uni-potsdam.de/research/WRK/2007/08/getting-os-information-the-kuser_shared_data-structure
// https://github.com/golang/go/blob/450d0b2f30e820f402a638799de0b886c1da8dbe/src/runtime/sys_windows_amd64.s#L499
//
// However, we skip a few things the runtime does to provide the facility to time.Now():
// - There is no fallback to QPC, which means this won't work on Wine except the most recent versions;
// - We offset the time straight into the sno epoch instead of into Unix first;
// - We do not perform a unit conversion from 100nsec (as returned by the OS) into 1nsec. Instead we
//   return this as is and the unit conversion is done in the wrapping snotime() function, where the
//   division gets optimized by the compiler;
// - There is no split into seconds and fractional nsecs, since - unlike time.Now() - this is the opposite
//   of what we want;
//
// All in all this lets us shave off about a dozen instructions - including a fairly expensive back-and-forth
// conversion between time units.
//
// func ostime() uint64
TEXT ·ostime(SB), NOSPLIT, $0-8
    MOVQ $2147352596, DI    // 0x7ffe0014 -> 2147352596
time:
    MOVL 4(DI), AX          // time_hi1
    MOVL 0(DI), BX          // time_lo
    MOVL 8(DI), CX          // time_hi2
    CMPL AX, CX
    JNE  time

    SHLQ $32, AX
    ORQ  BX, AX

    // Windows time as stored within _KUSER_SHARED_DATA starts at Jan 1st 1601.
    // The offset in the Windows units (100ns) to Unix epoch is a SUBQ by 116 444 736 000 000 000.
    //
    // Our internal epoch is:
    //          1 262 304 000    seconds on top of Unix.
    // 12 623 040 000 000 000‬    in units of 100nsec (secs * 1e7)
    //
    // As such we SUBQ 116444736000000000 (Windows to Unix diff) + 12623040000000000‬ (Sno to Unix diff)
    // 116 444 736 000 000 000
    //  12 623 040 000 000 000‬
    // ----
    // 129 067 776 000 000 000

    MOVQ $129067776000000000, DI
    SUBQ DI, AX

    MOVQ AX, ret+0(FP)
    RET
