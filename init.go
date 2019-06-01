package sno

// Note: This init is separated out for the future so that each architecture can make its necessary runtime checks.
// When multiple init() functions are present in a package, lexicographic sort order (of file names) decides
// about the order of execution, so this runs after all the encoding_{arch}.go files, which is the point.
func init() {
	var err error
	if generator, err = NewGenerator(nil, nil); err != nil {
		panic(err)
	}

	// If the package level codecs were supplied, respect it. Otherwise set the fallbacks
	// and build the decoding LUT.
	if encode != nil && decode != nil {
		return
	}

	encode = encodeScalar
	decode = decodeScalar
	dec = &[256]byte{}

	for i := 0; i < len(dec); i++ {
		dec[i] = 0xFF
	}

	for i := 0; i < len(encoding); i++ {
		dec[encoding[i]] = byte(i)
	}
}
