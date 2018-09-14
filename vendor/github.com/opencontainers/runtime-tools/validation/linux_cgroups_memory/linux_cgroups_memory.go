package main

import (
	"fmt"
	"runtime"

	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	if "linux" != runtime.GOOS {
		util.Fatal(fmt.Errorf("linux-specific cgroup test"))
	}

	t := tap.New()
	t.Header(0)

	cases := []struct {
		limit      int64
		swappiness uint64
	}{
		{50593792, 10},
		{50593792, 50},
		{50593792, 100},
		{151781376, 10},
		{151781376, 50},
		{151781376, 100},
	}

	for _, c := range cases {
		g, err := util.GetDefaultGenerator()
		if err != nil {
			util.Fatal(err)
		}
		g.SetLinuxCgroupsPath(cgroups.AbsCgroupPath)
		g.SetLinuxResourcesMemoryLimit(c.limit)
		g.SetLinuxResourcesMemoryReservation(c.limit)
		g.SetLinuxResourcesMemorySwap(c.limit)
		g.SetLinuxResourcesMemoryKernel(c.limit)
		g.SetLinuxResourcesMemoryKernelTCP(c.limit)
		g.SetLinuxResourcesMemorySwappiness(c.swappiness)
		g.SetLinuxResourcesMemoryDisableOOMKiller(true)
		err = util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesMemory)
		if err != nil {
			t.Fail(err.Error())
		}
	}

	t.AutoPlan()
}
