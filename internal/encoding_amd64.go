package internal

// Encode returns the sno32-encoded representation of src as an array of 16 bytes.
//go:noescape
func Encode(src *[10]byte) (dst [16]byte)

// Decode returns the binary representation of a sno32-encoded src as an array of bytes.
//
// Src does not get validated and must have a length of 16 - otherwise Decode will panic.
//go:noescape
func Decode(src []byte) (dst [10]byte)

// One-shot to determine whether we've got SSE2 at all - and the SSE4.2 and BMI2 sets
// that we need for the vectorized codecs.
//
// The fallbacks currently rely on SSE2 - while it's available on just about
// any modern amd64 platform, *just in case* it's not, the check will fail loudly
// and immediately (panic) instead of faulting on the first encode/decode attempt.
var hasVectorSupport = checkVectorSupport()
