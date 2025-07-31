package ipmi

import (
	"fmt"
)

// OutputType represents an Event/Reading Type Code, specified in Table 36-2 and
// 42-2 of IPMI v1.5 and v2.0 respectively. Appeal: if you write a
// specification, please do not put slashes in names. Event/Reading Type Codes
// indicate the type of reading a sensor provides. It is mainly useful for
// discrete sensors (analogue sensors are threshold-based).
type OutputType uint8

const (
	_ OutputType = iota // unspecified

	// OutputTypeThreshold indicates an analogue sensor whose values are
	// bucketed into states (e.g. Lower Non-critical, Upper Non-recoverable)
	// that are used in events it generates.
	OutputTypeThreshold

	// many not implemented; we'll save this complexity for when we actually
	// need it. Sensor classes in particular are all special cases of each other
	// and so are meaningless due to the overlap.
)

var (
	outputTypeDescriptions = map[OutputType]string{
		OutputTypeThreshold: "Threshold",
	}
)

func (o OutputType) Description() string {
	if desc, ok := outputTypeDescriptions[o]; ok {
		return desc
	}
	return "Unknown"
}

func (o OutputType) String() string {
	return fmt.Sprintf("%#v(%v)", uint8(o), o.Description())
}
