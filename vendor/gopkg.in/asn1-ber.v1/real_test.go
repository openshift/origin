package ber

import (
	"math"
	"testing"
)

var negativeZero = math.Copysign(0, -1)

func TestRealEncoding(t *testing.T) {
	for _, value := range []float64{
		0.15625,
		-0.15625,
		math.Inf(1),
		math.Inf(-1),
		math.NaN(),
		negativeZero,
		0.0,
	} {
		enc := encodeFloat(value)
		dec, err := ParseReal(enc)
		if err != nil {
			t.Errorf("Failed to decode %f (%v): %s", value, enc, err)
		}
		if dec != value {
			if !(math.IsNaN(dec) && math.IsNaN(value)) {
				t.Errorf("decoded value != orig: %f <=> %f", value, dec)
			}
		}
	}
}
