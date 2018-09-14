package main

import (
	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	var limit int64 = 50593792
	var swappiness uint64 = 50

	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetLinuxCgroupsPath(cgroups.RelCgroupPath)
	g.SetLinuxResourcesMemoryLimit(limit)
	g.SetLinuxResourcesMemoryReservation(limit)
	g.SetLinuxResourcesMemorySwap(limit)
	g.SetLinuxResourcesMemoryKernel(limit)
	g.SetLinuxResourcesMemoryKernelTCP(limit)
	g.SetLinuxResourcesMemorySwappiness(swappiness)
	g.SetLinuxResourcesMemoryDisableOOMKiller(true)
	err = util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesMemory)
	if err != nil {
		t.Fail(err.Error())
	}
}
