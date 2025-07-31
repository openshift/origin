package ipmi

import (
	"fmt"
)

// EntityID identifies the kind of hardware that a sensor or device is
// associated with, e.g. it distinguishes a processor from a power supply from a
// fan. EntityID codes can be found in 37.14 and 43.14 of IPMI v1.5 and 2.0
// respectively.
//
// A separate "instance" field discriminates between multiple occurrences of a
// given entity, e.g. multi-core CPUs and redundant power supplies. All sensors
// pertaining to a given piece of hardware will have the same entity and
// instance.
type EntityID uint8

const (
	EntityIDUnspecified EntityID = iota
	EntityIDOther
	_
	EntityIDProcessor
	EntityIDDisk
	EntityIDPeripheralBay
	EntityIDSystemManagementModule
	EntityIDSystemBoard
	EntityIDMemoryModule
	EntityIDProcessorModule
	EntityIDPowerSupply
	EntityIDAddInCard
	EntityIDFrontPanelBoard
	EntityIDBackPanelBoard
	EntityIDPowerSystemBoard
	EntityIDDriveBackplane

	EntityIDSystemChassis EntityID = 0x17
	EntityIDCoolingDevice EntityID = 0x1d
	EntityIDMemoryDevice  EntityID = 0x20

	EntityIDAirInlet EntityID = 0x37

	// EntityIDDCMIAirInlet allows associating temperature sensors to the
	// airflow at an air inlet. This is effectively deprecated, used by DCMI
	// v1.0 and v1.1. EntityIDAirInlet should be preferred.
	EntityIDDCMIAirInlet EntityID = 0x40

	// EntityIDDCMIProcessor is effectively deprecated, used by DCMI v1.0 and
	// v1.1. EntityIDProcessor should be preferred.
	EntityIDDCMIProcessor EntityID = 0x41

	// EntityIDDCMISystemBoard is effectively deprecated, used by DCMI v1.0 and
	// v1.1. EntityIDSystemBoard should be preferred.
	EntityIDDCMISystemBoard EntityID = 0x42
)

var (
	entityIdDescriptions = map[EntityID]string{
		EntityIDUnspecified:            "Unspecified",
		EntityIDOther:                  "Other",
		EntityIDProcessor:              "Processor",
		EntityIDDisk:                   "Disk (Bay)",
		EntityIDPeripheralBay:          "Peripheral Bay",
		EntityIDSystemManagementModule: "System Management Module",
		EntityIDSystemBoard:            "System Board",
		EntityIDMemoryModule:           "Memory Module",
		EntityIDProcessorModule:        "Processor Module",
		EntityIDPowerSupply:            "Power Supply",
		EntityIDAddInCard:              "Add-in Card",
		EntityIDFrontPanelBoard:        "Front Panel Board",
		EntityIDBackPanelBoard:         "Back Panel Board",
		EntityIDPowerSystemBoard:       "Power System Board",
		EntityIDDriveBackplane:         "Drive Backplane",
		EntityIDSystemChassis:          "System Chassis",
		EntityIDCoolingDevice:          "Cooling Device",
		EntityIDMemoryDevice:           "Memory Device",
		EntityIDAirInlet:               "Air Inlet",
		EntityIDDCMIAirInlet:           "Air Inlet (DCMI)",
		EntityIDDCMIProcessor:          "Processor (DCMI)",
		EntityIDDCMISystemBoard:        "System Board (DCMI)",
	}
)

func (e EntityID) Description() string {
	if desc, ok := entityIdDescriptions[e]; ok {
		return desc
	}
	return "Unknown"
}

func (e EntityID) String() string {
	return fmt.Sprintf("%#x(%v)", uint8(e), e.Description())
}
