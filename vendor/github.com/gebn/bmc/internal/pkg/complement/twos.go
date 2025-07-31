package complement

// Twos parses two's complement numbers of up to 16 bits into a native integer.
// The input is two bytes in big-endian order, and the number of bits the binary
// representation is expected to be (0 through 16). More significant bits above
// this must be 0, e.g. Twos([...]byte{0b000000xx, 0bxxxxxxxx}, 10).
func Twos(bigEndian [2]byte, bits uint8) int16 {
	// this abstracts away the endian-ness of the platform; big-endian only
	// refers to the byte order of the input. It is identical to
	// binary.BigEndian.Uint16(), but avoids creating a slice.
	numerical := uint16(bigEndian[1]) | uint16(bigEndian[0])<<8

	// sign extend to 16 bits
	// (https://graphics.stanford.edu/~seander/bithacks.html, "Sign extending
	// from a variable bit-width")
	mask := uint16(1) << (uint16(bits) - 1)
	numerical = (numerical ^ mask) - mask

	// make signed - same underlying bits, just different type
	return int16(numerical)
}
