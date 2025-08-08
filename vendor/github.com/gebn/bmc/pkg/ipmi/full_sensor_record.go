package ipmi

import (
	"fmt"

	"github.com/gebn/bmc/internal/pkg/complement"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// SensorRecordKey contains the Record Key fields for the Full Sensor Record and
// Compact Sensor Record SDR types.
type SensorRecordKey struct {

	// OwnerAddress uniquely identifies a management controller on the IPMB.
	// This is relevant for device-relative entity instances.
	OwnerAddress Address
	Channel      Channel
	OwnerLUN     LUN

	// Number uniquely identifies the sensor within the context of a given
	// owner. 0xff is used to indicate no more sensors, so there are 255 useful
	// values.
	Number uint8
}

// FullSensorRecord is specified in 37.1 and 43.1 of v1.5 and v2.0 respectively.
// It describes any type of sensor, and is the only record type that can
// describe a sensor generating analogue (i.e. non-enumerated/discrete)
// readings, e.g. a temperature sensor. It is specified as 64 bytes. This layer
// represents the record key and record body sections.
type FullSensorRecord struct {
	layers.BaseLayer
	SensorRecordKey
	ConversionFactors

	// IsContainerEntity indicates whether we should treat the entity as a
	// logical container entity, as opposed to a physical entity. This is used
	// in conjunction with Entity Association records.
	IsContainerEntity bool

	// Entity describes the type of component that the sensor monitors, e.g. a
	// processor. See EntityID for more details.
	Entity EntityID

	// Instance provides a way to distinguish between multiple occurrences of a
	// given entity, e.g. a dual socket system will likely have two processor
	// temperature sensors, each with a different instance. We can enumerate all
	// instances to ensure all processors are covered.
	Instance EntityInstance

	// Ignore indicates whether we should ignore the sensor if its entity is
	// absent or disabled. In general, this can be assumed to be true. The
	// entity's status can be obtained via an Entity Presence sensor.
	Ignore bool

	// SensorType indicates what is being measured. For analogue sensors, this
	// is the dimension, e.g. temperature. For discrete sensors, there are many
	// values to pinpoint exactly what is being exposed.
	SensorType SensorType

	// OutputType contains the Event/Reading Type Code of the underlying sensor.
	OutputType OutputType

	// AnalogDataFormat indicates whether the Reading, NormalMin, NormalMax,
	// SensorMin and SensorMax fields are unsigned, 1's complement or 2's
	// complement. This field will be AnalogDataFormatNotAnalog if the sensor
	// specifies thresholds but not numeric readings. Note it will be
	// AnalogDataFormatUnsigned if the sensor provides neither thresholds nor
	// analog readings. Identifying whether a sensor providing analog reading is
	// more an art than a science; in practice, this field alone is good enough.
	AnalogDataFormat AnalogDataFormat

	// RateUnit gives the time period throughput-based quantities are provided
	// over, e.g. airflow per second, minute, day etc.
	RateUnit RateUnit

	// IsPercentage indicates whether the reading is a percentage.
	IsPercentage bool

	// BaseUnit gives the primary unit of the sensor's reading, e.g. Celsius or
	// Fahrenheit for a temperature sensor.
	BaseUnit SensorUnit

	// ModifierUnit is contained in the Sensor Units 3 field. Note this is
	// distinct from the identically-named 2-bit field in Sensor Units 1. 0x0
	// means unused.
	ModifierUnit SensorUnit

	// Linearisation indicates whether the sensor is linear, linearised or
	// non-linear. This controls post-processing after applying the linear
	// conversion formula to the raw reading.
	Linearisation Linearisation

	// Tolerance gives the absolute accuracy of the sensor in +/- half raw
	// counts. This is a 6-bit uint on the wire.
	Tolerance uint8

	// Accuracy gives the sensor accuracy in 0.01% increments when raised to
	// AccuracyExp. This is a 10-bit int on the wire.
	Accuracy int16

	// AccuracyExp is the quantity Accuracy is raised to the power of to give
	// the final accuracy.
	AccuracyExp uint8

	// Direction indicates whether the sensor is monitoring input or output of
	// the entity.
	Direction SensorDirection

	// NominalReadingSpecified indicates whether the NominalReading field should
	// be interpreted.
	NominalReadingSpecified bool

	// NormalMinSpecified indicates whether the NormalMin field should be
	// interpreted.
	NormalMinSpecified bool

	// NormalMaxSpecified indicates whether the NormalMax field should be
	// interpreted.
	NormalMaxSpecified bool

	// NominalReading contains a sample value for the sensor. Note: this is
	// *not* the current reading. This is in the format specified by
	// AnalogDataFormat. It should be ignored if the characteristic flags
	// indicate the nominal reading is not specified.
	NominalReading uint8

	// NormalMin prints the lower threshold for normal reading range. It is in
	// the format specified by AnalogDataFormat. It should be ignored if the
	// characteristic flags indicate the normal min reading is not specified.
	NormalMin uint8

	// NormalMax prints the upper threshold for normal reading range. It is in
	// the format specified by AnalogDataFormat. It should be ignored if the
	// characteristic flags indicate the normal min reading is not specified.
	NormalMax uint8

	// SensorMin caps the lowest reading the sensor can provide. Readings lower
	// than this should be given as this value (this is not enforced).
	SensorMin uint8

	// SensorMax caps the highest reading the sensor can provide. Readings
	// higher than this should be given as this value (this is not enforced).
	SensorMax uint8

	// Identity is a descriptive string for the sensor. This can be up to 16
	// bytes long, which translates into 16-32 characters depending on the
	// format used. There are no conventions around this, and it is provided for
	// informational purposes only. Contrary to the name, attempting to identify
	// sensors based on this value is doomed to fail.
	Identity string

	// TODO ignored many fields
}

func (*FullSensorRecord) LayerType() gopacket.LayerType {
	return LayerTypeFullSensorRecord
}

func (r *FullSensorRecord) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

func (*FullSensorRecord) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (r *FullSensorRecord) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 43 {
		df.SetTruncated()
		return fmt.Errorf("Full Sensor Records are at least 43 bytes long, got %v",
			len(data))
	}

	// to go from the offsets here to the byte numbers in the specification, add
	// 6, e.g. data[8] -> byte 14 in the table.

	r.OwnerAddress = Address(data[0])
	r.Channel = Channel(data[1] >> 4)
	r.OwnerLUN = LUN(data[1] & 0x3)
	r.Number = uint8(data[2])

	r.Entity = EntityID(data[3])
	r.IsContainerEntity = data[4]&(1<<7) != 0
	r.Instance = EntityInstance(data[4] & 0x7f)

	r.Ignore = data[6]&(1<<7) != 0

	r.SensorType = SensorType(data[7])
	r.OutputType = OutputType(data[8])

	r.AnalogDataFormat = AnalogDataFormat(data[15] >> 6)
	r.RateUnit = RateUnit((data[15] & 0x38) >> 3)
	// modifier unit when needed
	r.IsPercentage = data[15]&1 != 0

	r.BaseUnit = SensorUnit(data[16])
	r.ModifierUnit = SensorUnit(data[17])

	r.Linearisation = Linearisation(data[18] & 0x7f)

	buf := [...]byte{data[20] >> 6, data[19]}
	r.M = complement.Twos(buf, 10)
	r.Tolerance = uint8(data[20] & 0x3f)
	buf[1] = data[21]
	buf[0] = data[22] >> 6
	r.B = complement.Twos(buf, 10)
	buf[1] = data[22]&0x3f | ((data[23] & 0xf0) << 2)
	buf[0] = (data[23] & 0xf0) >> 6
	r.Accuracy = complement.Twos(buf, 10)
	r.AccuracyExp = uint8(data[23]&0xc) >> 2
	r.Direction = SensorDirection(data[23] & 0x3)
	buf[0] = 0
	buf[1] = data[24] >> 4
	r.RExp = int8(complement.Twos(buf, 4))
	buf[1] = data[24] & 0xf
	r.BExp = int8(complement.Twos(buf, 4))

	r.NominalReadingSpecified = data[25]&1 != 0
	r.NormalMaxSpecified = data[25]&(1<<1) != 0
	r.NormalMinSpecified = data[25]&(1<<2) != 0

	r.NominalReading = uint8(data[26])
	r.NormalMax = uint8(data[27])
	r.NormalMin = uint8(data[28])

	r.SensorMax = uint8(data[29])
	r.SensorMin = uint8(data[30])

	encoding := StringEncoding(data[42] >> 6)
	decoder, err := encoding.Decoder()
	if err != nil {
		// unsupported encoding; fail loudly so we can fix this
		return err
	}
	characters := int(data[42] & 0x1f)
	identity, consumed, err := decoder.Decode(data[43:], characters)
	if err != nil {
		// invalid bytes
		return err
	}
	r.Identity = identity
	r.BaseLayer.Contents = data[:43+consumed]
	r.BaseLayer.Payload = data[43+consumed:]
	return nil
}
