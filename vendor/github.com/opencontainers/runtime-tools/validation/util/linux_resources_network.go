package util

import (
	"fmt"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/cgroups"
)

// ValidateLinuxResourcesNetwork validates linux.resources.network.
func ValidateLinuxResourcesNetwork(config *rspec.Spec, t *tap.T, state *rspec.State) error {
	cg, err := cgroups.FindCgroup()
	t.Ok((err == nil), "find network cgroup")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	lnd, err := cg.GetNetworkData(state.Pid, config.Linux.CgroupsPath)
	t.Ok((err == nil), "get network cgroup data")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	t.Ok(*lnd.ClassID == *config.Linux.Resources.Network.ClassID, "network ID set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Network.ClassID, *lnd.ClassID)

	for _, priority := range config.Linux.Resources.Network.Priorities {
		found := false
		for _, lip := range lnd.Priorities {
			if lip.Name == priority.Name {
				found = true
				t.Ok(lip.Priority == priority.Priority, fmt.Sprintf("network priority for %s is set correctly", priority.Name))
				t.Diagnosticf("expect: %d, actual: %d", priority.Priority, lip.Priority)
			}
		}
		t.Ok(found, fmt.Sprintf("network priority for %s found", priority.Name))
	}

	return nil
}
