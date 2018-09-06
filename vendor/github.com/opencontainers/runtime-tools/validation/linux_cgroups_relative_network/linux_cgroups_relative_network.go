package main

import (
	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	var id, prio uint32 = 255, 10
	ifName := "lo"

	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetLinuxCgroupsPath(cgroups.RelCgroupPath)
	g.SetLinuxResourcesNetworkClassID(id)
	g.AddLinuxResourcesNetworkPriorities(ifName, prio)
	err = util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesNetwork)
	if err != nil {
		t.Fail(err.Error())
	}
}
