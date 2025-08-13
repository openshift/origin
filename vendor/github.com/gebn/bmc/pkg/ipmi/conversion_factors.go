package ipmi

import (
	"math"
)

// ConversionFactors contains inputs to the linear formula in 30.3 and 36.3 of
// v1.5 and v2.0 respectively. This struct exists as conversion factors can come
// from two sources: full sensor records, and the Get Sensor Reading Factors
// command response. In practice, we get them from the former for linear and
// linearised sensors, as these have constant factors. We need to obtain them
// from the Get Sensor Reading Factors command for non-linear sensors, as they
// vary by reading here. Both FullSensorRecord and GetSensorReadingFactorsRsp
// embed this type.
//
// Note that we split application of the formula into "conversion" and
// "linearisation". Conversion happens first, and is the linear formula applied
// to the raw value. The linearisation step, which is a no-op for linear and
// non-linear sensors, applies one of the formulae in the specification to the
// result of the conversion. This struct only deals with conversion; see
// Lineariser for linearisation.
type ConversionFactors struct {

	// M is the constant multiplier. This is a 10-bit 2's complement number on
	// the wire.
	M int16

	// B is the additive offset. This is a 10-bit 2's complement number on the
	// wire.
	B int16

	// BExp is the exponent, controlling the location of the decimal point in B.
	// This is also referred to as K1 in the spec, and is a 4-bit 2's complement
	// number on the wire.
	BExp int8

	// RExp is the result exponent, controlling the location of the decimal
	// point in the result of the linear formula and hence input to the
	// linearisation function. This is also referred to as K2 in the spec, and
	// is a 4-bit 2's complement number on the wire.
	RExp int8
}

// ConvertReading applies the linear formula to a raw sensor reading, without
// the linearisation formula. It is independent of unit. This method takes an
// int16 rather than uint8 as raw values can be in 1 or 2's complement, or
// unsigned, so it must accept from -128 (lowest 2's complement) to 255 (highest
// unsigned). The conversion from the raw format to a native int must be done
// before calling this method.
func (f *ConversionFactors) ConvertReading(raw int16) float64 {
	mX := int64(f.M) * int64(raw)
	b10k1 := float64(f.B) * math.Pow10(int(f.BExp))
	return (float64(mX) + b10k1) * math.Pow10(int(f.RExp))
}
