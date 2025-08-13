package complement

// Ones parses an 8-bit one's complement number into a native integer. The
// returned value has range -127 through 127 (one's complement has both positive
// and negative 0).
func Ones(b byte) int8 {
	if b&0x80 != 0 {
		b++
	}
	return int8(b)
}
