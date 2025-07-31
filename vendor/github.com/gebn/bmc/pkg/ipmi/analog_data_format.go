package ipmi

import (
	"fmt"

	"github.com/gebn/bmc/internal/pkg/complement"
)

// AnalogDataFormatParser is implemented by types that can convert a raw
// sensor reading into a native int16. The interface's purpose is to abstract
// over 8-bit values on the wire that could be unsigned, 1's complement or 2's
// complement.
type AnalogDataFormatParser interface {

	// Parse turns an 8-bit raw sensor value into its Go value.  This returns an
	// int16 as this is the "smallest" integral type that can return a superset
	// of these 3 binary formats.
	Parse(byte) int16
}

// AnalogDataFormatParserFunc is a convenience type allowing functions to
// statelessly implement AnalogDataFormatParser.
type AnalogDataFormatParserFunc func(byte) int16

// Parse calls the underlying function with the raw input value, returning the
// result.
func (f AnalogDataFormatParserFunc) Parse(r byte) int16 {
	return f(r)
}

// parseAnalogDataFormatUnsigned converts a byte containing an 8-bit unsigned
// integer into an int16.
func parseAnalogDataFormatUnsigned(r byte) int16 {
	return int16(r)
}

// parseAnalogDataFormatOnesComplement converts a byte containing an 8-bit 1's
// complement integer into an int16.
func parseAnalogDataFormatOnesComplement(r byte) int16 {
	return int16(complement.Ones(r))
}

// parseAnalogDataFormatTwosComplement converts a byte containing an 8-bit 2's
// complement integer into an int16.
func parseAnalogDataFormatTwosComplement(r byte) int16 {
	// converting straight to int16 does not sign-extend
	return int16(int8(r))
}

// AnalogDataFormat represents the binary format of analog sensor readings and
// thresholds. It is specified in byte 21 of the Full Sensor Record table in
// 37.1 and 43.1 of v1.5 and v2.0 respectively. It is a 2-bit uint on the wire.
type AnalogDataFormat uint8

const (
	// AnalogDataFormatUnsigned indicates an unsigned analog sensor. It is also
	// used in the case where the sensor provides neither analog readings nor
	// thresholds.
	AnalogDataFormatUnsigned AnalogDataFormat = iota
	AnalogDataFormatOnesComplement
	AnalogDataFormatTwosComplement

	// AnalogDataFormatNotAnalog indicates the sensor does not have numeric
	// readings, only thresholds.
	AnalogDataFormatNotAnalog
)

var (
	analogDataFormatParsers = map[AnalogDataFormat]AnalogDataFormatParser{
		AnalogDataFormatUnsigned:       AnalogDataFormatParserFunc(parseAnalogDataFormatUnsigned),
		AnalogDataFormatOnesComplement: AnalogDataFormatParserFunc(parseAnalogDataFormatOnesComplement),
		AnalogDataFormatTwosComplement: AnalogDataFormatParserFunc(parseAnalogDataFormatTwosComplement),
	}
	analogDataFormatDescriptions = map[AnalogDataFormat]string{
		AnalogDataFormatUnsigned:       "Unsigned",
		AnalogDataFormatOnesComplement: "1's Complement",
		AnalogDataFormatTwosComplement: "2's Complement",
		AnalogDataFormatNotAnalog:      "No analog readings",
	}
)

// Parser returns an AnalogDataFormatParser instance capable of turning raw
// values from this sensor (including normal/sensor min/max) into native Go
// values. If the format does not have a parser, e.g.
// AnalogDataFormatNotAnalog, this returns an error.
func (f AnalogDataFormat) Parser() (AnalogDataFormatParser, error) {
	if parser, ok := analogDataFormatParsers[f]; ok {
		return parser, nil
	}
	return nil, fmt.Errorf("no analog data format parser found for %v", f)
}

func (f AnalogDataFormat) Description() string {
	if desc, ok := analogDataFormatDescriptions[f]; ok {
		return desc
	}
	return "Unknown"
}

func (f AnalogDataFormat) String() string {
	return fmt.Sprintf("%#v(%v)", uint8(f), f.Description())
}
