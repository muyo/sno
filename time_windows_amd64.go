package sno

//go:noescape
func ostime() uint64

// snotimeReal implements snotime.
func snotimeReal() uint64 {
	// Note: Division is left here instead of being impl in asm since the compiler optimizes this
	// into mul+shift, which is easier to read when left in as simple division.
	// This doesn't affect performance. The asm won't get inlined anyway while this function
	// will.
	//
	// 4e4 instead of TimeUnit (4e6) because the time we get from the OS is in units of 100ns.
	return ostime() / 4e4
}
