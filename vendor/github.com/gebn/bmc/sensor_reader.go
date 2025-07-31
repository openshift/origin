package bmc

import (
	"context"
	"errors"
	"fmt"

	"github.com/gebn/bmc/pkg/ipmi"
)

var (
	// ErrSensorReadingUnavailable is returned when the BMC sets the
	// "reading/state unavailable" flag in the Get Sensor Reading response. This
	// indicates the reading should be ignored, so is returned as an error.
	ErrSensorReadingUnavailable = errors.New("sensor reading is not available")

	// ErrSensorScanningDisabled is returned when the BMC sets the "sensor
	// scanning disabled" flag in the Get Sensor Reading response. This suggests
	// the machine is powered down and the reading should be ignored, so is
	// returned as an error.
	ErrSensorScanningDisabled = errors.New("sensor is disabled")
)

// SensorReader is implemented by types that can read the current value of a
// sensor. This abstracts over linear, linearised and non-linear sensors,
// issuing the relevant commands behind the scenes. It is rather high-level,
// however it is difficult to, say, implement only conversion given a raw value,
// as this sometimes requires conversion factors - given we would need a session
// there, we may as well retrieve the current reading in the first place.
type SensorReader interface {

	// Read returns the current value of the sensor with any conversion factors
	// and linearisation applied. It will return ErrSensorReadingUnavailable if
	// the BMC indicates the sensor is not yet ready.
	Read(context.Context, Session) (float64, error)
}

// NewSensorReader returns an appropriate SensorReader implementation for a
// given SDR.
func NewSensorReader(r *ipmi.FullSensorRecord) (SensorReader, error) {
	// TODO non-linear
	switch {
	case r.Linearisation.IsLinear():
		return newLinearSensorReader(r)
	case r.Linearisation.IsLinearised():
		return newLinearisedSensorReader(r)
	default:
		return nil, fmt.Errorf("unsupported sensor linearisation: %v",
			r.Linearisation)
	}
}

// linearSensorReader implements a reader for linear sensors. These sensors have
// a no-op linearisation, with only analog data format parsing and conversion
// factors to apply.
type linearSensorReader struct {
	readingCmd ipmi.GetSensorReadingCmd
	parser     ipmi.AnalogDataFormatParser
	factors    ipmi.ConversionFactors
}

func newLinearSensorReader(r *ipmi.FullSensorRecord) (*linearSensorReader, error) {
	parser, err := r.AnalogDataFormat.Parser()
	if err != nil {
		// sensor does not provide analog readings
		return nil, err
	}
	return &linearSensorReader{
		readingCmd: ipmi.GetSensorReadingCmd{
			Req: ipmi.GetSensorReadingReq{
				Number: r.Number,
			},
			OwnerLUN: r.OwnerLUN,
		},
		factors: r.ConversionFactors,
		parser:  parser,
	}, nil
}

func (r *linearSensorReader) Read(ctx context.Context, s Session) (float64, error) {
	if err := ValidateResponse(s.SendCommand(ctx, &r.readingCmd)); err != nil {
		// some BMCs return an empty response when the component is not present
		return 0, err
	}
	if r.readingCmd.Rsp.ReadingUnavailable {
		return 0, ErrSensorReadingUnavailable
	}
	if !r.readingCmd.Rsp.ScanningEnabled {
		return 0, ErrSensorScanningDisabled
	}
	parsed := r.parser.Parse(r.readingCmd.Rsp.Reading)
	return r.factors.ConvertReading(parsed), nil
}

// linearisedSensorReader implements a reader for linearised sensors. These are
// conceptually linear sensors with a final linearisation step, and so is
// implemented as a wrapper around linearSensorReader.
type linearisedSensorReader struct {
	linearReader *linearSensorReader
	lineariser   ipmi.Lineariser
}

func newLinearisedSensorReader(r *ipmi.FullSensorRecord) (*linearisedSensorReader, error) {
	reader, err := newLinearSensorReader(r)
	if err != nil {
		return nil, err
	}
	lineariser, err := r.Linearisation.Lineariser()
	if err != nil {
		return nil, err
	}
	return &linearisedSensorReader{
		linearReader: reader,
		lineariser:   lineariser,
	}, nil
}

func (r *linearisedSensorReader) Read(ctx context.Context, s Session) (float64, error) {
	reading, err := r.linearReader.Read(ctx, s)
	if err != nil {
		return 0, err
	}
	return r.lineariser.Linearise(reading), nil
}
