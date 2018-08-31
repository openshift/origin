package main

import (
	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	var weight uint16 = 500
	var leafWeight uint16 = 300
	var major, minor int64 = 8, 0
	var rate uint64 = 102400

	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetLinuxCgroupsPath(cgroups.RelCgroupPath)
	g.SetLinuxResourcesBlockIOWeight(weight)
	g.SetLinuxResourcesBlockIOLeafWeight(leafWeight)
	g.AddLinuxResourcesBlockIOWeightDevice(major, minor, weight)
	g.AddLinuxResourcesBlockIOLeafWeightDevice(major, minor, leafWeight)
	g.AddLinuxResourcesBlockIOThrottleReadBpsDevice(major, minor, rate)
	g.AddLinuxResourcesBlockIOThrottleWriteBpsDevice(major, minor, rate)
	g.AddLinuxResourcesBlockIOThrottleReadIOPSDevice(major, minor, rate)
	g.AddLinuxResourcesBlockIOThrottleWriteIOPSDevice(major, minor, rate)
	err = util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesBlockIO)
	if err != nil {
		t.Fail(err.Error())
	}
}
