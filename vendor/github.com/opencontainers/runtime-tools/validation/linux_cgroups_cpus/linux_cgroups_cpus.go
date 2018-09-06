package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

const (
	defaultRealtimePeriod  uint64 = 1000000
	defaultRealtimeRuntime int64  = 950000
)

func testCPUCgroups() error {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	CPUrange := fmt.Sprintf("0-%d", runtime.NumCPU()-1)

	// Test with different combinations of values.
	// NOTE: most systems have only one memory node (mems=="0"), so we cannot
	// simply test with multiple values of mems.
	cases := []struct {
		shares uint64
		period uint64
		quota  int64
		cpus   string
		mems   string
	}{
		{1024, 100000, 50000, "0", "0"},
		{1024, 100000, 50000, CPUrange, "0"},
		{1024, 100000, 200000, "0", "0"},
		{1024, 100000, 200000, CPUrange, "0"},
		{1024, 500000, 50000, "0", "0"},
		{1024, 500000, 50000, CPUrange, "0"},
		{1024, 500000, 200000, "0", "0"},
		{1024, 500000, 200000, CPUrange, "0"},
		{2048, 100000, 50000, "0", "0"},
		{2048, 100000, 50000, CPUrange, "0"},
		{2048, 100000, 200000, "0", "0"},
		{2048, 100000, 200000, CPUrange, "0"},
		{2048, 500000, 50000, "0", "0"},
		{2048, 500000, 50000, CPUrange, "0"},
		{2048, 500000, 200000, "0", "0"},
		{2048, 500000, 200000, CPUrange, "0"},
	}

	for _, c := range cases {
		g, err := util.GetDefaultGenerator()
		if err != nil {
			return fmt.Errorf("cannot get default config from generator: %v", err)
		}

		g.SetLinuxCgroupsPath(cgroups.AbsCgroupPath)

		if c.shares > 0 {
			g.SetLinuxResourcesCPUShares(c.shares)
		}

		if c.period > 0 {
			g.SetLinuxResourcesCPUPeriod(c.period)
		}

		if c.quota > 0 {
			g.SetLinuxResourcesCPUQuota(c.quota)
		}

		if c.cpus != "" {
			g.SetLinuxResourcesCPUCpus(c.cpus)
		}

		if c.mems != "" {
			g.SetLinuxResourcesCPUMems(c.mems)
		}

		// NOTE: On most systems where CONFIG_RT_GROUP & CONFIG_RT_GROUP_SCHED are not enabled,
		// the following tests will fail, because sysfs knobs like
		// /sys/fs/cgroup/cpu,cpuacct/cpu.rt_{period,runtime}_us do not exist.
		// So we need to check if the sysfs knobs exist before setting the variables.
		if _, err := os.Stat(filepath.Join(util.CPUCgroupPrefix, "cpu.rt_period_us")); !os.IsNotExist(err) {
			g.SetLinuxResourcesCPURealtimePeriod(defaultRealtimePeriod)
		}

		if _, err := os.Stat(filepath.Join(util.CPUCgroupPrefix, "cpu.rt_runtime_us")); !os.IsNotExist(err) {
			g.SetLinuxResourcesCPURealtimeRuntime(defaultRealtimeRuntime)
		}

		if err := util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesCPU); err != nil {
			return fmt.Errorf("cannot validate CPU cgroups: %v", err)
		}
	}

	return nil
}

func testEmptyCPU() error {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	g, err := util.GetDefaultGenerator()
	if err != nil {
		return fmt.Errorf("cannot get default config from generator: %v", err)
	}
	g.InitConfigLinuxResourcesCPU()
	g.SetLinuxCgroupsPath(cgroups.AbsCgroupPath)

	if err := util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesCPUEmpty); err != nil {
		return fmt.Errorf("cannot validate empty CPU cgroups: %v", err)
	}

	return nil
}

func main() {
	if "linux" != runtime.GOOS {
		util.Fatal(fmt.Errorf("linux-specific cgroup test"))
	}

	if err := testCPUCgroups(); err != nil {
		util.Fatal(err)
	}

	if err := testEmptyCPU(); err != nil {
		util.Fatal(err)
	}
}
