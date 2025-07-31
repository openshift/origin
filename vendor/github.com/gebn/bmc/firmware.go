package bmc

import (
	"encoding/binary"
	"fmt"

	"github.com/gebn/bmc/pkg/iana"
	"github.com/gebn/bmc/pkg/ipmi"
)

// FirmwareVersion builds a firmware version string using all information
// available in the Get Device ID response. Many manufacturers use the 4
// auxiliary firmware revision bytes to add additional components to the
// version, e.g. "3.15.17.15" in the case of Dell. Using the major and minor
// firmware version fields alone would only yield 3.15. This function aims to
// capture as much relevant data as possible, falling back on "major.minor".
// Where possible, it returns data in the same format (e.g. 0 padding) as in the
// BMC web interface.
func FirmwareVersion(r *ipmi.GetDeviceIDRsp) string {
	switch r.Manufacturer {
	case iana.EnterpriseIntel:
		// 0 and 1 are major and minor boot firmware revision, BCD encoded
		return fmt.Sprintf("%02d.%02d.%d",
			r.MajorFirmwareRevision, r.MinorFirmwareRevision,
			binary.LittleEndian.Uint16(r.AuxiliaryFirmwareRevision[2:]))
	case iana.EnterpriseDell:
		// 0 is always 0x00, unused
		return fmt.Sprintf("%d.%d.%d.%db%02d",
			r.MajorFirmwareRevision, r.MinorFirmwareRevision,
			r.AuxiliaryFirmwareRevision[2],
			r.AuxiliaryFirmwareRevision[3],
			r.AuxiliaryFirmwareRevision[1])
	case iana.EnterpriseQuanta:
		// 1, 2, 3 are always 0x00, unused
		return fmt.Sprintf("%d.%d.%02d",
			r.MajorFirmwareRevision, r.MinorFirmwareRevision,
			r.AuxiliaryFirmwareRevision[0])
	case iana.EnterpriseSuperMicro:
		// does not use the aux revision bytes, but formats to two digits
		return fmt.Sprintf("%02d.%02d",
			r.MajorFirmwareRevision, r.MinorFirmwareRevision)
	default:
		return fmt.Sprintf("%d.%d",
			r.MajorFirmwareRevision, r.MinorFirmwareRevision)
	}
}
