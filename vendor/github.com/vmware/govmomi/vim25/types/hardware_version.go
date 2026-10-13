// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"regexp"
	"strconv"
)

// HardwareVersion is a VMX hardware version.
type HardwareVersion uint8

const (
	invalidHardwareVersion HardwareVersion = 0
)

const (
	VMX3 HardwareVersion = iota + 3
	VMX4

	vmx5 // invalid

	VMX6
	VMX7
	VMX8
	VMX9
	VMX10
	VMX11
	VMX12
	VMX13
	VMX14
	VMX15
	VMX16
	VMX17
	VMX18
	VMX19
	VMX20
	VMX21
)

const (
	// MinValidHardwareVersion is the minimum, valid hardware version supported
	// by VMware hypervisors in the wild.
	MinValidHardwareVersion = VMX3

	// MaxValidHardwareVersion is the maximum, valid hardware version supported
	// by VMware hypervisors in the wild.
	MaxValidHardwareVersion = VMX21
)

// IsSupported returns true if the hardware version is known to and supported by
// GoVmomi's generated types.
func (hv HardwareVersion) IsSupported() bool {
	return hv.IsValid() &&
		hv != vmx5 &&
		hv >= MinValidHardwareVersion &&
		hv <= MaxValidHardwareVersion
}

// IsValid returns true if the hardware version is not valid.
// Unlike IsSupported, this function returns true as long as the hardware
// version is greater than 0.
// For example, the result of parsing "abc" or "vmx-abc" is an invalid hardware
// version, whereas the result of parsing "vmx-99" is valid, just not supported.
func (hv HardwareVersion) IsValid() bool {
	return hv != invalidHardwareVersion
}

func (hv HardwareVersion) String() string {
	if hv.IsValid() {
		return fmt.Sprintf("vmx-%d", hv)
	}
	return ""
}

func (hv HardwareVersion) MarshalText() ([]byte, error) {
	return []byte(hv.String()), nil
}

func (hv *HardwareVersion) UnmarshalText(text []byte) error {
	v, err := ParseHardwareVersion(string(text))
	if err != nil {
		return err
	}
	*hv = v
	return nil
}

var (
	vmxRe        = regexp.MustCompile(`(?i)^vmx-(\d+)$`)
	vmxNumOnlyRe = regexp.MustCompile(`^(\d+)$`)
)

// MustParseHardwareVersion parses the provided string into a hardware version.
func MustParseHardwareVersion(s string) HardwareVersion {
	v, err := ParseHardwareVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseHardwareVersion parses the provided string into a hardware version.
// Supported formats include vmx-123 or 123. Please note that the parser will
// only return an error if the supplied version does not match the supported
// formats.
// Once parsed, use the function IsSupported to determine if the hardware
// version falls into the range of versions known to GoVmomi.
func ParseHardwareVersion(s string) (HardwareVersion, error) {
	if m := vmxRe.FindStringSubmatch(s); len(m) > 0 {
		u, err := strconv.ParseUint(m[1], 10, 8)
		if err != nil {
			return invalidHardwareVersion, fmt.Errorf(
				"failed to parse %s from %q as uint8: %w", m[1], s, err)
		}
		return HardwareVersion(u), nil
	} else if m := vmxNumOnlyRe.FindStringSubmatch(s); len(m) > 0 {
		u, err := strconv.ParseUint(m[1], 10, 8)
		if err != nil {
			return invalidHardwareVersion, fmt.Errorf(
				"failed to parse %s as uint8: %w", m[1], err)
		}
		return HardwareVersion(u), nil
	}
	return invalidHardwareVersion, fmt.Errorf("invalid version: %q", s)
}

var hardwareVersions []HardwareVersion

func init() {
	for i := MinValidHardwareVersion; i <= MaxValidHardwareVersion; i++ {
		if i.IsSupported() {
			hardwareVersions = append(hardwareVersions, i)
		}
	}
}

// GetHardwareVersions returns a list of hardware versions.
func GetHardwareVersions() []HardwareVersion {
	dst := make([]HardwareVersion, len(hardwareVersions))
	copy(dst, hardwareVersions)
	return dst
}
