package ipmi

import (
	"fmt"
)

// SensorDirection indicates whether a sensor is monitoring an input or output
// relative to the entity, e.g. input voltage vs. output voltage. It is
// specified in byte 29 of the Full Sensor Record Table 43-1 in IPMI v2.0.
type SensorDirection uint8

const (
	SensorDirectionUnspecified SensorDirection = iota
	SensorDirectionInput
	SensorDirectionOutput
)

var (
	sensorDirectionDescriptions = map[SensorDirection]string{
		SensorDirectionUnspecified: "Unspecified/not applicable",
		SensorDirectionInput:       "Input",
		SensorDirectionOutput:      "Output",
	}
)

func (s SensorDirection) Description() string {
	if direction, ok := sensorDirectionDescriptions[s]; ok {
		return direction
	}
	return "Unknown"
}

func (s SensorDirection) String() string {
	return fmt.Sprintf("%#v(%v)", uint8(s), s.Description())
}
