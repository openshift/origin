package util

import (
	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/cgroups"
)

// ValidateLinuxResourcesPids validates linux.resources.pids.
func ValidateLinuxResourcesPids(config *rspec.Spec, t *tap.T, state *rspec.State) error {
	cg, err := cgroups.FindCgroup()
	t.Ok((err == nil), "find pids cgroup")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	lpd, err := cg.GetPidsData(state.Pid, config.Linux.CgroupsPath)
	t.Ok((err == nil), "get pids cgroup data")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	t.Ok(lpd.Limit == config.Linux.Resources.Pids.Limit, "pids limit is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", config.Linux.Resources.Pids.Limit, lpd.Limit)

	return nil
}
