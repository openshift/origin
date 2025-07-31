package ipmi

import (
	"fmt"
)

// SensorType indicates what a sensor measures, e.g. a temperature, chassis
// intrusion, or cooling device. There are some seemingly conflicting values, e.g.
// temperature (0x01) could be that of a processor (0x07), however the types
// after 0x04 are seemingly for discrete sensors, and have an additional
// sub-type in the form of a sensor specific offset. See Table 36-3 and 42-3 in
// v1.5 and v2.0 respectively.
type SensorType uint8

const (
	_ SensorType = iota
	SensorTypeTemperature
	SensorTypeVoltage
	SensorTypeCurrent
	SensorTypeFan
	SensorTypePhysicalSecurity
	SensorTypePlatformSecurity
	SensorTypeProcessor
	SensorTypePowerSupply
	SensorTypePowerUnit
	SensorTypeCoolingDevice
	SensorTypeOtherUnitsBasedSensor
	SensorTypeMemory
	SensorTypeDriveBay

	// non-exhaustive
)

var (
	sensorTypeDescriptions = map[SensorType]string{
		SensorTypeTemperature:           "Temperature",
		SensorTypeVoltage:               "Voltage",
		SensorTypeCurrent:               "Current",
		SensorTypeFan:                   "Fan",
		SensorTypePhysicalSecurity:      "Physical Security",
		SensorTypePlatformSecurity:      "Platform Security",
		SensorTypeProcessor:             "Processor",
		SensorTypePowerSupply:           "Power Supply",
		SensorTypePowerUnit:             "Power Unit",
		SensorTypeCoolingDevice:         "Cooling Device",
		SensorTypeOtherUnitsBasedSensor: "Other Units-based Sensor",
		SensorTypeMemory:                "Memory",
		SensorTypeDriveBay:              "Drive Bay",
	}
)

func (t SensorType) Description() string {
	if desc, ok := sensorTypeDescriptions[t]; ok {
		return desc
	}
	return "Unknown"
}

func (t SensorType) String() string {
	return fmt.Sprintf("%#x(%v)", uint8(t), t.Description())
}
