package ipmi

import (
	"fmt"
	"math"
)

// This file implements functions to decode the final "ID String" field in full
// and compact sensor records.

var (
	// bcdPlus defines the mappings of BCD plus nibbles to runes, specified in
	// 37.15 and 43.15 of v1.5 and v2.0 respectively. An N byte string consists
	// of 2N characters.
	bcdPlusRunes = [16]rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		' ', '-', '.', ':', ',', '_'}
)

// StringDecoder is implemented by things that know how to parse the final ID
// String field of full and compact SDRs.
type StringDecoder interface {

	// Decode parses the first c characters (0 <= c <= 30) in b in the expected
	// format (N.B. this could be a varying number of bytes depending on the
	// encoding), returning the resulting string and number of bytes consumed,
	// or an error if the data is too short or invalid.
	//
	// c was implemented as an int rather than uint8 to reduce the number of
	// conversions required.
	Decode(b []byte, c int) (string, int, error)
}

// StringDecoderFunc eases implementation of stateless StringDecoders.
type StringDecoderFunc func([]byte, int) (string, int, error)

// Decode calls the contained function on the inputs, passing through the
// returned values verbatim.
func (f StringDecoderFunc) Decode(b []byte, c int) (string, int, error) {
	return f(b, c)
}

// StringEncoding describes the most significant two bits of the SDR Type/Length
// Byte, specified in 37.15 and 43.15 of v1.5 and v2.0 respectively.
type StringEncoding uint8

const (
	// StringEncodingUnicode, contrary to the name, typically suggests an
	// unspecified encoding. IPMItool displays a hex representation of the
	// underlying bytes, while OpenIPMI interprets it identically to
	// StringEncoding8BitAsciiLatin1. Given Unicode is only a character set and
	// the spec does not suggest any encoding, there is no right answer. The
	// resulting variety of implementations means use of this value by a BMC
	// should be regarded as a bug.
	StringEncodingUnicode StringEncoding = iota
	StringEncodingBCDPlus
	StringEncodingPacked6BitAscii
	StringEncoding8BitAsciiLatin1
)

var (
	stringEncodingDescriptions = map[StringEncoding]string{
		StringEncodingUnicode:         "Unicode",
		StringEncodingBCDPlus:         "BCD plus",
		StringEncodingPacked6BitAscii: "6-bit ASCII, packed",
		StringEncoding8BitAsciiLatin1: "8-bit ASCII + Latin 1",
	}
	// stringEncodingDecoders maps encodings to functions that are capable of
	// turning them into Go strings. These functions are not implemented inline
	// to ease readability and testability
	stringEncodingDecoders = map[StringEncoding]StringDecoder{
		// despite the ambiguity of StringEncodingUnicode, we follow OpenIPMI
		// and decode it as 8-bit ASCII
		StringEncodingUnicode:         StringDecoderFunc(decode8BitAsciiLatin1),
		StringEncodingBCDPlus:         StringDecoderFunc(decodeBCDPlus),
		StringEncodingPacked6BitAscii: StringDecoderFunc(decodePacked6BitAscii),
		StringEncoding8BitAsciiLatin1: StringDecoderFunc(decode8BitAsciiLatin1),
	}
)

func (e StringEncoding) Decoder() (StringDecoder, error) {
	if decoder, ok := stringEncodingDecoders[e]; ok {
		return decoder, nil
	}
	return nil, fmt.Errorf("no decoder found for encoding %v", e)
}

func (e StringEncoding) Description() string {
	if desc, ok := stringEncodingDescriptions[e]; ok {
		return desc
	}
	return "Unknown"
}

func (e StringEncoding) String() string {
	return fmt.Sprintf("%#v(%v)", uint8(e), e.Description())
}

func decodeBCDPlus(b []byte, c int) (string, int, error) {
	// each byte contains 2 characters (1 per nibble), so the number of
	// bytes we expect equals half the number of characters, rounded up
	bytes := int(math.Ceil(float64(c) / 2))
	if len(b) < bytes {
		return "", 0, fmt.Errorf("expected %v bytes, got %v", bytes, len(b))
	}

	runes := make([]rune, c)
	for i := 0; i < c; i++ {
		shift := uint8(0)
		if i%2 == 0 {
			// character is in the most significant 4 bits; need to
			// shift down
			shift = 4
		}
		runes[i] = bcdPlusRunes[(b[i/2]>>shift)&0xf]
	}
	return string(runes), bytes, nil
}

func decodePacked6BitAscii(b []byte, c int) (string, int, error) {
	// the minimum number of bytes required to represent c characters; c does
	// not have to be a multiple of 4
	bytes := c - (c / 4)
	if len(b) < bytes {
		return "", 0, fmt.Errorf("expected %v bytes, got %v", bytes, len(b))
	}

	runes := make([]rune, c)
	acc := uint8(0)
	for i := 0; i < c; i++ {
		// offset is the start offset for the first ASCII bits of the char at i;
		// this formula required a bit of experimentation in Excel. N.B. cannot
		// remove math.Floor() as need to round towards 0, not just strip the
		// fractional component.
		offset := (i - 1) - int(math.Floor(float64(i-1)/4))

		// the switch extracts the appropriate 6 bits into acc (most significant
		// two bits will always be 0)
		switch i % 4 {
		case 0:
			// least significant 6 bits at offset
			acc = b[offset] & 0x3f
		case 1:
			// least sig 2 bits: most sig 2 bits at offset
			// most sig 4 bits: least sig 4 bits at offset + 1
			acc = b[offset] >> 6
			acc |= (b[offset+1] & 0xf) << 2
		case 2:
			// least sig 4 bits: most sig 4 bits at offset
			// most sig 2 bits: least sig 2 bits at offset + 1
			acc = b[offset] >> 4
			acc |= (b[offset+1] & 0x3) << 4
		case 3:
			// most sig 6 bits at offset
			acc = b[offset] >> 2
		}

		// observe character corresponding to code
		runes[i] = rune(acc + 0x20)
	}
	return string(runes), bytes, nil
}

func decode8BitAsciiLatin1(b []byte, c int) (string, int, error) {
	if len(b) < 2 {
		// it is unclear why this limitation exists, but it's plain to
		// see in the specification
		return "", 0, fmt.Errorf("at least 2 bytes of data must be present; got %v bytes", len(b))
	}

	// bounds check to ensure the slicing below does not panic
	if len(b) < c {
		return "", 0, fmt.Errorf("expected %v bytes, got %v", c, len(b))
	}

	// can convert straight into a string as the encoding's range is
	// identical to UTF-8
	return string(b[:c]), c, nil
}
