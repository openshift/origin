package main

import (
	"fmt"
	"net"
	"runtime"

	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func testNetworkCgroups() error {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	loIfName := ""
	ethIfName := ""

	// we try to get the first 2 interfaces on the system, to assign
	// the first one to loIfName, the second to ethIfName.
	l, err := net.Interfaces()
	if err != nil {
		// fall back to the default list only with lo
		loIfName = "lo"
		ethIfName = "eth0"
	} else {
		loIfName = l[0].Name
		ethIfName = l[1].Name
	}

	cases := []struct {
		classid    uint32
		prio       uint32
		ifName     string
		withNetNs  bool
		withUserNs bool
	}{
		{255, 10, loIfName, true, true},
		{255, 10, loIfName, true, false},
		{255, 10, loIfName, false, true},
		{255, 10, loIfName, false, false},
		{255, 10, ethIfName, true, true},
		{255, 10, ethIfName, true, false},
		{255, 10, ethIfName, false, true},
		{255, 10, ethIfName, false, false},
		{255, 30, loIfName, true, true},
		{255, 30, loIfName, true, false},
		{255, 30, loIfName, false, true},
		{255, 30, loIfName, false, false},
		{255, 30, ethIfName, true, true},
		{255, 30, ethIfName, true, false},
		{255, 30, ethIfName, false, true},
		{255, 30, ethIfName, false, false},
		{550, 10, loIfName, true, true},
		{550, 10, loIfName, true, false},
		{550, 10, loIfName, false, true},
		{550, 10, loIfName, false, false},
		{550, 10, ethIfName, true, true},
		{550, 10, ethIfName, true, false},
		{550, 10, ethIfName, false, true},
		{550, 10, ethIfName, false, false},
		{550, 30, loIfName, true, true},
		{550, 30, loIfName, true, false},
		{550, 30, loIfName, false, true},
		{550, 30, loIfName, false, false},
		{550, 30, ethIfName, true, true},
		{550, 30, ethIfName, true, false},
		{550, 30, ethIfName, false, true},
		{550, 30, ethIfName, false, false},
	}

	for _, c := range cases {
		g, err := util.GetDefaultGenerator()
		if err != nil {
			return err
		}

		g.SetLinuxCgroupsPath(cgroups.AbsCgroupPath)
		g.SetLinuxResourcesNetworkClassID(c.classid)
		g.AddLinuxResourcesNetworkPriorities(c.ifName, c.prio)

		if !c.withNetNs {
			g.RemoveLinuxNamespace("network")
		}
		if !c.withUserNs {
			g.RemoveLinuxNamespace("user")
		}

		err = util.RuntimeOutsideValidate(g, t, util.ValidateLinuxResourcesNetwork)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if "linux" != runtime.GOOS {
		util.Fatal(fmt.Errorf("linux-specific cgroup test"))
	}

	if err := testNetworkCgroups(); err != nil {
		util.Fatal(err)
	}
}
