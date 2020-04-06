#include "textflag.h"
#include "funcdata.h"

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
DATA interleave<>+8(SB)/8, $0x1f1f1f1f1f1f1f1f
GLOBL interleave<>(SB), (NOPTR+RODATA), $16

// func Encode(src *[10]byte) (dst [16]byte)
TEXT ·Encode(SB), NOSPLIT, $0-24
    MOVQ  src+0(FP), BX

    MOVQ   0(BX), AX
    BSWAPQ AX
    SHRQ   $24, AX

    MOVQ   5(BX), BX
    BSWAPQ BX
    SHRQ   $24, BX

    CMPB  ·hasVectorSupport(SB), $1
    JEQ    encodeVec

    LEAQ   dst+8(FP), DX

    MOVB   AX, 7(DX)
    SHRQ   $5, AX
    MOVB   AX, 6(DX)
    SHRQ   $5, AX
    MOVB   AX, 5(DX)
    SHRQ   $5, AX
    MOVB   AX, 4(DX)
    SHRQ   $5, AX
    MOVB   AX, 3(DX)
    SHRQ   $5, AX
    MOVB   AX, 2(DX)
    SHRQ   $5, AX
    MOVB   AX, 1(DX)
    SHRQ   $5, AX
    MOVB   AX, 0(DX)

    MOVB   BX, 15(DX)
    SHRQ   $5, BX
    MOVB   BX, 14(DX)
    SHRQ   $5, BX
    MOVB   BX, 13(DX)
    SHRQ   $5, BX
    MOVB   BX, 12(DX)
    SHRQ   $5, BX
    MOVB   BX, 11(DX)
    SHRQ   $5, BX
    MOVB   BX, 10(DX)
    SHRQ   $5, BX
    MOVB   BX, 9(DX)
    SHRQ   $5, BX
    MOVB   BX, 8(DX)

    MOVOU  (DX), X0
    PAND   interleave<>+0(SB), X0

    JMP    encodeFinish

encodeVec:
    PDEPQ  interleave<>+0(SB), AX, AX
    PDEPQ  interleave<>+0(SB), BX, BX

    MOVQ   AX, X0
    PINSRQ $1, BX, X0
    PSHUFB shuffleVec<>+0(SB), X0

encodeFinish:
    MOVOA   X0, X1
    PADDB   offsetCharset<>+0(SB), X0   // Add 50, where 50 is the beginning of our alphabet (ASCII '2')
                                        // That takes care of all digits. We need to offset letters, though,
                                        // as they start at char('a'), which is 97 in dec.
    PCMPGTB selectLetters<>+0(SB), X1   // PCMPGTB will set all bytes with letters to 255.
    PSUBUSB subLetters<>+0(SB), X1      // We need to add 39 to each letter in X0 to move them into the right range.
                                        // Note: Not 47 (50 + 47 = 97), as our letters are in the [8..31] range.
                                        // And so we simply do a (unsigned) subtraction of 216 and as a result
                                        // get a mask of 39 (the offset) in dec where all the letters are.
    PADDB X1, X0                        // Add them together and done.

    MOVOU X0, dst+8(FP)

    RET


//func Decode(src []byte) (dst [10]byte)
TEXT ·Decode(SB), NOSPLIT, $0-34
    // The entirety of this function is simply the inverse of encode.
    MOVQ  src+0(FP), BX
    LEAQ  dst+24(FP), DX
    MOVOU (BX), X0

    PSUBB  offsetCharset<>+0(SB), X0
    MOVOA  X0, X1

    PCMPGTB selectLetters<>+0(SB), X1
    PSUBUSB subLetters<>+0(SB), X1
    PSUBB   X1, X0

    CMPB  ·hasVectorSupport(SB), $0
    JEQ   decodeFallback

    PSHUFB shuffleVec<>+0(SB), X0

    MOVQ       X0, R8
    PEXTRQ $1, X0, R9

    PEXTQ  interleave<>+0(SB), R8, R8
    BSWAPQ R8
    SHRQ   $24, R8

    PEXTQ  interleave<>+0(SB), R9, R9
    BSWAPQ R9
    SHRQ   $24, R9

    MOVQ R8, 0(DX)
    MOVQ R9, 5(DX)

    RET

decodeFallback:
    // TODO(alcore) Subject to an optimization pass.
    MOVQ   X0, R8
    PSRLO  $8, X0
    MOVQ   X0, R9

    // Timestamp block - 0
    MOVB R8, BX
    SHLB $3, BX

    SHRQ $8, R8 // 1
    MOVB R8, AX
    SHRB $2, AX
    ORB  AX, BX

    MOVB BX, 0(DX)

    MOVB R8, BX
    SHLB $6, BX

    SHRQ $8, R8 // 2
    MOVB R8, AX
    SHLB $1, AX
    ORB  AX, BX

    SHRQ $8, R8 // 3
    MOVB R8, CX
    SHRB $4, CX
    ORB  CX, BX

    MOVB BX, 1(DX)

    MOVB R8, BX
    SHLB $4, BX

    SHRQ $8, R8 // 4
    MOVB R8, AX
    SHRB $1, AX
    ORB  AX, BX

    MOVB BX, 2(DX)

    MOVB R8, BX
    SHLB $7, BX

    SHRQ $8, R8 // 5
    MOVB R8, CX
    SHLB $2, CX
    ORB  CX, BX

    SHRQ $8, R8 // 6
    MOVB R8, AX
    SHRB $3, AX
    ORB  AX, BX

    MOVB BX, 3(DX)

    MOVB R8, BX
    SHLB $5, BX

    SHRQ $8, R8 // 7
    ORB  R8, BX

    MOVB BX, 4(DX)

    // Payload block - 8
    MOVB R9, BX
    SHLB $3, BX

    SHRQ $8, R9 // 9
    MOVB R9, AX
    SHRB $2, AX
    ORB  AX, BX

    MOVB BX, 5(DX)

    MOVB R9, BX
    SHLB $6, BX

    SHRQ $8, R9 // 10
    MOVB R9, AX
    SHLB $1, AX
    ORB  AX, BX

    SHRQ $8, R9 // 11
    MOVB R9, CX
    SHRB $4, CX
    ORB  CX, BX

    MOVB BX, 6(DX)

    MOVB R9, BX
    SHLB $4, BX

    SHRQ $8, R9 // 12
    MOVB R9, AX
    SHRB $1, AX
    ORB  AX, BX

    MOVB BX, 7(DX)

    MOVB R9, BX
    SHLB $7, BX

    SHRQ $8, R9 // 13
    MOVB R9, CX
    SHLB $2, CX
    ORB  CX, BX

    SHRQ $8, R9 // 14
    MOVB R9, AX
    SHRB $3, AX
    ORB  AX, BX

    MOVB BX, 8(DX)

    MOVB R9, BX
    SHLB $5, BX

    SHRQ $8, R9 // 15
    ORB  R9, BX

    MOVB BX, 9(DX)

    RET
