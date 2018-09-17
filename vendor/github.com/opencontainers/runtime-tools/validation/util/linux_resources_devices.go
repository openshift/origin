package util

import (
	"fmt"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/specerror"
)

// ValidateLinuxResourcesDevices validates linux.resources.devices.
func ValidateLinuxResourcesDevices(config *rspec.Spec, t *tap.T, state *rspec.State) error {
	cg, err := cgroups.FindCgroup()
	t.Ok((err == nil), "find devices")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	lnd, err := cg.GetDevicesData(state.Pid, config.Linux.CgroupsPath)
	t.Ok((err == nil), "get devices data")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	for i, device := range config.Linux.Resources.Devices {
		if device.Allow == true {
			found := false
			if lnd[i-1].Type == device.Type && *lnd[i-1].Major == *device.Major && *lnd[i-1].Minor == *device.Minor && lnd[i-1].Access == device.Access {
				found = true
			}
			t.Ok(found, fmt.Sprintf("devices %s %d:%d %s is set correctly", device.Type, *device.Major, *device.Minor, device.Access))
			t.Diagnosticf("expect: %s %d:%d %s, actual: %s %d:%d %s",
				device.Type, *device.Major, *device.Minor, device.Access, lnd[i-1].Type, *lnd[i-1].Major, *lnd[i-1].Minor, lnd[i-1].Access)
			if !found {
				err := specerror.NewError(specerror.DevicesApplyInOrder, fmt.Errorf("The runtime MUST apply entries in the listed order"), rspec.Version)
				t.Diagnostic(err.Error())
				return nil
			}
		}
	}

	return nil
}
