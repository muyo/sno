package internal

//go:noescape
func Encode(src *[10]byte) (dst [16]byte)

//go:noescape
func Decode(src []byte) (dst [10]byte)

// One-shot to determine whether we've got SSE2 at all - and the SSE4.2 and BMI2 sets
// that we need for the vectorized codecs.
//
// The fallbacks currently rely on SSE2 - while it's available on just about
// any modern amd64 platform, *just in case* it's not, the check will fail loudly
// and immediately (panic) instead of faulting on the first encode/decode attempt.
var hasVectorSupport = checkVectorSupport()
