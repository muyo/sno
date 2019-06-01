#include "textflag.h"
#include "funcdata.h"

// func cpuId(op uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuId(SB), NOSPLIT, $16-24
    XORQ CX, CX
    MOVB op+0(FP), AX
    CPUID
    MOVL AX, eax+8(FP)
    MOVL BX, ebx+12(FP)
    MOVL CX, ecx+16(FP)
    MOVL DX, edx+20(FP)
    RET


DATA shuffleVec<>+0(SB)/8, $0x0001020304050607
DATA shuffleVec<>+8(SB)/8, $0x08090A0B0C0D0E0F
GLOBL shuffleVec<>(SB), (NOPTR+RODATA), $16

DATA offsetCharset<>+0(SB)/8, $0x3232323232323232 // 50
DATA offsetCharset<>+8(SB)/8, $0x3232323232323232
GLOBL offsetCharset<>(SB), (NOPTR+RODATA), $16

DATA selectLetters<>+0(SB)/8, $0x0707070707070707
DATA selectLetters<>+8(SB)/8, $0x0707070707070707
GLOBL selectLetters<>(SB), (NOPTR+RODATA), $16

DATA subLetters<>+0(SB)/8, $0xD8D8D8D8D8D8D8D8 // 216
DATA subLetters<>+8(SB)/8, $0xD8D8D8D8D8D8D8D8
GLOBL subLetters<>(SB), (NOPTR+RODATA), $16

DATA interleave<>+0(SB)/8, $0x1f1f1f1f1f1f1f1f
GLOBL interleave<>(SB), (NOPTR+RODATA), $8

#define SHUFFLE_VEC X11
#define CHARSET_BEGIN X12
#define CHARSET_LETTERS_OFFSET X13
#define CHARSET_LETTERS_SELECT X14
#define INTERLEAVE R10

// func encodeVector(src *ID) [SizeEncoded]byte
TEXT ·encodeVector(SB), NOSPLIT, $16-24
    MOVOA shuffleVec<>+0(SB),    SHUFFLE_VEC
    MOVOA selectLetters<>+0(SB), CHARSET_LETTERS_SELECT
    MOVOA subLetters<>+0(SB),    CHARSET_LETTERS_OFFSET
    MOVOA offsetCharset<>+0(SB), CHARSET_BEGIN
    MOVQ  interleave<>+0(SB),    INTERLEAVE

    MOVQ  src+0(FP), BX

    // TODO(alcore) The PDEPQ section below can *probably* be optimized with mul instructions
    // instead. A single PSHUFB with a vector like...
    //
    // DATA shuffleVec<>+0(SB)/8, $0x0001020304FFFFFF
    // DATA shuffleVec<>+8(SB)/8, $0x0506070809FFFFFF
    //
    // ... can get the 2 40-bit blocks into big endian order and the following layout:
    //
    // [aaaaabbb|bbcccccd|ddddeeee|efffffgg|ggghhhhh|--------|--------|--------]
    //
    // A needs to be shifted right 59 times, B 46 times, C 33 times and so on (each being a 13-shift,
    // where f, g and h need to be shifted left accordingly) to get a layout like:
    //
    // [---hhhhh|---ggggg|---fffff|---eeeee|---ddddd|---ccccc|---bbbbb|---aaaaa]
    //
    // A single PAND with a 1F mask on the lanes would then take care of squeezing each
    // byte into the 0..31 range.

    MOVQ   0(BX), R8
    BSWAPQ R8
    SHRQ   $24, R8
    PDEPQ  INTERLEAVE, R8, R8

    MOVQ   5(BX), R9
    BSWAPQ R9
    SHRQ   $24, R9
    PDEPQ  INTERLEAVE, R9, R9

    MOVQ   R8, X0
    PINSRQ $1, R9, X0
    PSHUFB SHUFFLE_VEC, X0

    MOVOA   X0, X1
    PADDB   CHARSET_BEGIN, X0               // Add 48, where 48 is the beginning of our alphabet (ASCII '0')
                                            // That takes care of all digits. We need to offset letters, though,
                                            // as they start at char('a'), which is 97 in dec.
    PCMPGTB CHARSET_LETTERS_SELECT, X1      // PCMPGTB will set all bytes with letters to 255.
    PSUBUSB CHARSET_LETTERS_OFFSET, X1      // We need to add 37 to each letter in X0 to move them into the right range.
                                            // Note: Not 47 (50 + 47 = 97), because all our letter are already
                                            // in the range [8..31]. And so we simply do a (unsigned) subtraction
                                            // of 217 and as a result get a mask of 37 in dec where all the letters are.
    PADDB X1, X0                            // Add them together and done.

    MOVOU X0, ret+8(FP)                     // Store.

    RET


// func decodeVector(dst *ID, src []byte)
TEXT ·decodeVector(SB), NOSPLIT, $16-32
    MOVOA shuffleVec<>+0(SB),    SHUFFLE_VEC
    MOVOA selectLetters<>+0(SB), CHARSET_LETTERS_SELECT
    MOVOA subLetters<>+0(SB),    CHARSET_LETTERS_OFFSET
    MOVOA offsetCharset<>+0(SB), CHARSET_BEGIN
    MOVQ  interleave<>+0(SB),    INTERLEAVE

    // The entirety of this function is simply the inverse of encodeVector.
	MOVQ  src+8(FP), BX
	MOVQ  dst+0(FP), DX
    MOVOU (BX), X0

    PSUBB  CHARSET_BEGIN, X0
    MOVOA  X0, X1

    PCMPGTB CHARSET_LETTERS_SELECT, X1
    PSUBUSB CHARSET_LETTERS_OFFSET, X1
    PSUBB   X1, X0

    PSHUFB SHUFFLE_VEC, X0

    MOVQ       X0, R8
    PEXTRQ $1, X0, R9

    PEXTQ  INTERLEAVE, R8, R8
    BSWAPQ R8
    SHRQ   $24, R8

    PEXTQ  INTERLEAVE, R9, R9
    BSWAPQ R9
    SHRQ   $24, R9


    MOVQ R8, 0(DX)
    MOVQ R9, 5(DX)

    RET
