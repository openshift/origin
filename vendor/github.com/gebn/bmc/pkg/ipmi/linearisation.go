package ipmi

import (
	"errors"
	"fmt"
	"math"
)

const (
	LinearisationLinear Linearisation = iota
	LinearisationLn
	LinearisationLog10
	LinearisationLog2
	LinearisationE
	LinearisationExp10
	LinearisationExp2
	LinearisationInverse
	LinearisationSqr
	LinearisationCube
	LinearisationSqrt
	LinearisationCubeRt
	LinearisationNonLinear

	// 0x71 through 0x7f are reserved for non-linear, OEM defined
	// linearisations. It is unclear why these cannot use
	// LinearisationNonLinear, as being non-linear, they do not have a
	// linearisation formula. Waiting for a use case to emerge rather than
	// implementing a questionably useful RegisterLineariser() function.
)

var (
	// ErrNotLinearised is returned if Lineariser() is called on a linear or
	// non-linear linearisation. Linear sensors' values do not require any
	// transformation by virtue of the sensor already being linear. If the sensor
	// is non-linear, the conversion factors returned by Get Sensor Reading
	// Factors are all that are needed to obtain a real value: by being unique
	// to the raw sensor reading, there is no need for a separate linearisation
	// formula.
	//
	// Linearise() could return a no-op lineariser, however the current
	// implementation should never ask for one on a non-linearised sensor, so
	// instead we return an error to flag up a possible bug.
	ErrNotLinearised = errors.New(
		"only linearised sensors have a linearisation formula")

	linearisationDescriptions = map[Linearisation]string{
		LinearisationLinear:    "Linear",
		LinearisationLn:        "ln",
		LinearisationLog10:     "log10",
		LinearisationLog2:      "log2",
		LinearisationE:         "e",
		LinearisationExp10:     "exp10",
		LinearisationExp2:      "exp2",
		LinearisationInverse:   "1/x",
		LinearisationSqr:       "sqr(x)",
		LinearisationCube:      "cube(x)",
		LinearisationSqrt:      "sqrt(x)",
		LinearisationCubeRt:    "x^(1/3)",
		LinearisationNonLinear: "Non-linear",
	}

	// linearisationLinearisers allows us to find out what linearisation formula
	// needs to be applied to the converted output of a linearised sensor, to
	// produce a real value. Note that linear and non-linear linearisations do
	// not appear here as they don't need a linearisation formula.
	linearisationLinearisers = map[Linearisation]Lineariser{
		LinearisationLn:    LineariserFunc(math.Log),
		LinearisationLog10: LineariserFunc(math.Log10),
		LinearisationLog2:  LineariserFunc(math.Log2),
		LinearisationE:     LineariserFunc(math.Exp),
		LinearisationExp10: LineariserFunc(func(f float64) float64 {
			// cannot use math.Pow10 as that takes an int
			return math.Pow(10, f)
		}),
		LinearisationExp2: LineariserFunc(math.Exp2),
		LinearisationInverse: LineariserFunc(func(f float64) float64 {
			return math.Pow(f, -1)
		}),
		LinearisationSqr: LineariserFunc(func(f float64) float64 {
			return math.Pow(f, 2)
		}),
		LinearisationCube: LineariserFunc(func(f float64) float64 {
			return math.Pow(f, 3)
		}),
		LinearisationSqrt: LineariserFunc(math.Sqrt),
		LinearisationCubeRt: LineariserFunc(func(f float64) float64 {
			return math.Pow(f, 1./3)
		}),
	}
)

// Linearisation indicates whether a sensor is linear, linearised, or
// non-linear. Values are specified in the Full Sensor Record wire format table
// in 37-1 and 43-1 of v1.5 and v2.0 respectively.
//
// Linear sensors are the easiest to deal with. The sensor's raw readings are
// converted into real readings (e.g. Celsius) with a linear formula. Accuracy
// and resolution are constant in real terms across the entire range of values
// produced by the sensor.
//
// Linearised are slightly more challenging. The same linear formula is applied
// as for linear sensors, however a final "linearisation formula" is applied to
// obtain the real reading. This transformation is one of 11 defined in the
// spec, e.g. log or sqrt, and obviously does not have to be linear itself. The
// tolerance (the spec misuses accuracy as a synonym) of linearised sensors is
// also constant for all values. This is possible despite the existence of the
// linearisation formula turning raw values into disproportionate real values,
// as tolerance is expressed relative to 0. This assumes the sensor's tolerance
// does not diminish in real, absolute terms at extreme values (positive or
// negative), as there is no way of representing it (you'd have to resort to
// declaring it a non-linear sensor). Note that tolerance can only be expressed
// in half-raw value increments, which is in itself quite coarse. Regarding
// resolution, this will vary with reading due to the linearisation formula. The
// recommended way to calculate it is to retrieve and calculate the real values
// (with the help of Get Sensor Reading Factors as necessary) corresponding to
// the raw values below and above the actual raw value observed. Subtracting the
// real reading for the raw value below the observed raw value from the real
// reading for the observed value gives the negative resolution, and the process
// is equivalent for the positive resolution using the raw value one above.
//
// All consistency bets are off with non-linear sensors. Not only does
// resolution vary by reading (calculated in the same was as for linearised
// sensors), but so does tolerance. Get Sensor Reading Factors must be sent with
// each raw reading; applying the linear formula using the returned conversion
// factors yields the real reading, and can the same factors can be plugged into
// the tolerance and resolution formulae to calculate them.
type Linearisation uint8

// IsLinear returns whether the underlying sensor is linear. Calling
// Lineariser() will return an error, as there is no linearisation formula (it
// is effectively a no-op). Only the linear formula in the spec needs be applied
// to obtain a real reading.
func (l Linearisation) IsLinear() bool {
	return l == LinearisationLinear
}

// IsLinearised returns whether the underlying sensor is linearised, meaning the
// value after conversion needs to be fed through a linearisation formula as a
// final step before being used. A suitable implementation of this function is
// returned by the Lineariser() method.
func (l Linearisation) IsLinearised() bool {
	return l > LinearisationLinear && l < LinearisationNonLinear
}

// IsNonLinear returns whether the underlying sensor is not consistent enough
// for the constraints of linear and linearised. As for linear sensors,
// attempting to retrieve a Lineariser will return an error. Readings from these
// sensors require Get Sensor Reading Factors to convert them into usable
// values.
func (l Linearisation) IsNonLinear() bool {
	return l >= LinearisationNonLinear
}

// Lineariser returns a suitable Lineariser implementation that will turn the
// converted raw value produced by the underlying sensor into a usable value. If
// the sensor is already linear, or non-linear, this will return
// ErrNotLinearised.
func (l Linearisation) Lineariser() (Lineariser, error) {
	if lineariser, ok := linearisationLinearisers[l]; ok {
		return lineariser, nil
	}
	return nil, ErrNotLinearised
}

func (l Linearisation) Description() string {
	if desc, ok := linearisationDescriptions[l]; ok {
		return desc
	}
	if l >= 0x71 && l <= 0x7f {
		return "Non-linear OEM"
	}
	return "Unknown"
}

func (l Linearisation) String() string {
	return fmt.Sprintf("%#v(%v)", uint8(l), l.Description())
}

// Lineariser is implemented by formulae that can linearise a value returned by
// the Get Sensor Reading command that has gone through the linear formula
// containing M, B, K1 and K2, used for all sensors.
type Lineariser interface {

	// Linearise applies a linearisation formula to a converted value, returning
	// the final value in the correct unit. This is the last step in the "Sensor
	// Reading Conversion Formula" described in section 30.3 of IPMI v1.5 and
	// v2.0.
	Linearise(float64) float64
}

// LineariserFunc is the type of the function in the Lineariser interface. It
// allows us to create stateless Lineariser implementations from raw functions,
// including those in the math package.
type LineariserFunc func(float64) float64

// Linearise invokes the wrapped function, passing through the input and result.
func (l LineariserFunc) Linearise(f float64) float64 {
	return l(f)
}
