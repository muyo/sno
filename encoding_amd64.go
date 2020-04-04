package sno

import "github.com/muyo/sno/internal"

//go:noescape
func encode(src *ID) (dst [SizeEncoded]byte)

//go:noescape
func decode(src []byte) (dst ID)

// One-shot to determine whether we've got SSE2 at all - and the SSE4.2 and BMI2 sets
// that we need for the vectorized codecs.
//
// The fallbacks currently rely on SSE2 - while it's available on just about
// any modern amd64 platform, *just in case* it's not, the check will fail loudly
// and immediately (panic) instead of faulting on the first encode/decode attempt.
var hasVectorSupport = internal.HasVectorSupport()
