package util

import (
	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/cgroups"
)

// ValidateLinuxResourcesMemory validates linux.resources.memory.
func ValidateLinuxResourcesMemory(config *rspec.Spec, t *tap.T, state *rspec.State) error {
	cg, err := cgroups.FindCgroup()
	t.Ok((err == nil), "find memory cgroup")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	lm, err := cg.GetMemoryData(state.Pid, config.Linux.CgroupsPath)
	t.Ok((err == nil), "get memory cgroup data")
	if err != nil {
		t.Diagnostic(err.Error())
		return nil
	}

	t.Ok(*lm.Limit == *config.Linux.Resources.Memory.Limit, "memory limit is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Memory.Limit, *lm.Limit)

	t.Ok(*lm.Reservation == *config.Linux.Resources.Memory.Reservation, "memory reservation is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Memory.Reservation, *lm.Reservation)

	t.Ok(*lm.Swap == *config.Linux.Resources.Memory.Swap, "memory swap is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Memory.Swap, *lm.Reservation)

	t.Ok(*lm.Kernel == *config.Linux.Resources.Memory.Kernel, "memory kernel is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Memory.Kernel, *lm.Kernel)

	t.Ok(*lm.KernelTCP == *config.Linux.Resources.Memory.KernelTCP, "memory kernelTCP is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Memory.KernelTCP, *lm.Kernel)

	t.Ok(*lm.Swappiness == *config.Linux.Resources.Memory.Swappiness, "memory swappiness is set correctly")
	t.Diagnosticf("expect: %d, actual: %d", *config.Linux.Resources.Memory.Swappiness, *lm.Swappiness)

	t.Ok(*lm.DisableOOMKiller == *config.Linux.Resources.Memory.DisableOOMKiller, "memory oom is set correctly")
	t.Diagnosticf("expect: %t, actual: %t", *config.Linux.Resources.Memory.DisableOOMKiller, *lm.DisableOOMKiller)

	return nil
}
