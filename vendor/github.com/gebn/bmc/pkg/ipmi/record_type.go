package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
)

// RecordType indicates the format of a Sensor Data Record, e.g. a Full Sensor
// Record, Compact Sensor Record, or Entity Association Record. It is a field in
// the SDR header, essential for informing the remote console how to interpret
// the rest of the packet. Although "sensor" is in the name, an SDR does not
// necessarily pertain to a sensor.
type RecordType uint8

const (
	RecordTypeFullSensor                        RecordType = 0x01
	RecordTypeCompactSensor                     RecordType = 0x02
	RecordTypeEventOnly                         RecordType = 0x03
	RecordTypeEntityAssociation                 RecordType = 0x08
	RecordTypeDeviceRelativeEntityAssociation   RecordType = 0x09
	RecordTypeGenericDeviceLocator              RecordType = 0x10
	RecordTypeFRUDeviceLocator                  RecordType = 0x11
	RecordTypeManagementControllerDeviceLocator RecordType = 0x12
	RecordTypeManagementControllerConfirmation  RecordType = 0x13
	RecordTypeBMCMessageChannelInfo             RecordType = 0x14
)

var (
	recordTypeLayerTypes = map[RecordType]gopacket.LayerType{
		RecordTypeFullSensor: LayerTypeFullSensorRecord,
	}
	recordTypeDescriptions = map[RecordType]string{
		RecordTypeFullSensor:                        "Full Sensor Record",
		RecordTypeCompactSensor:                     "Compact Sensor Record",
		RecordTypeEventOnly:                         "Event-only Record",
		RecordTypeEntityAssociation:                 "Entity Association Record",
		RecordTypeDeviceRelativeEntityAssociation:   "Device-relative Entity Association Record",
		RecordTypeGenericDeviceLocator:              "Generic Device Locator Record",
		RecordTypeFRUDeviceLocator:                  "FRU Device Locator Record",
		RecordTypeManagementControllerDeviceLocator: "Management Controller Device Locator Record",
		RecordTypeManagementControllerConfirmation:  "Management Controller Confirmation Record",
		RecordTypeBMCMessageChannelInfo:             "BMC Message Channel Info Record",
	}
)

func (t RecordType) NextLayerType() gopacket.LayerType {
	if layer, ok := recordTypeLayerTypes[t]; ok {
		return layer
	}
	return gopacket.LayerTypePayload
}

func (t RecordType) Description() string {
	if desc, ok := recordTypeDescriptions[t]; ok {
		return desc
	}
	return "Unknown"
}

func (t RecordType) String() string {
	return fmt.Sprintf("%#x(%v)", uint8(t), t.Description())
}
