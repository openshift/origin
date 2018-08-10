package main

import (
	"fmt"
	"runtime"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func testHugetlbCgroups() error {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	// limit =~ 100 * page size
	// NOTE: on some systems, pagesize "1GB" doesn't seem to work.
	// Ideally we should auto-detect the value.
	cases := []struct {
		page  string
		limit uint64
	}{
		{"2MB", 100 * 2 * 1024 * 1024},
		{"1GB", 100 * 1024 * 1024 * 1024},
		{"2MB", 100 * 2 * 1024 * 1024},
		{"1GB", 100 * 1024 * 1024 * 1024},
	}

	for _, c := range cases {
		g, err := util.GetDefaultGenerator()
		if err != nil {
			return err
		}
		g.SetLinuxCgroupsPath(cgroups.AbsCgroupPath)
		g.AddLinuxResourcesHugepageLimit(c.page, c.limit)
		err = util.RuntimeOutsideValidate(g, t, func(config *rspec.Spec, t *tap.T, state *rspec.State) error {
			cg, err := cgroups.FindCgroup()
			if err != nil {
				return err
			}
			lhd, err := cg.GetHugepageLimitData(state.Pid, config.Linux.CgroupsPath)
			if err != nil {
				return err
			}
			for _, lhl := range lhd {
				if lhl.Pagesize != c.page {
					continue
				}
				t.Ok(lhl.Limit == c.limit, "hugepage limit is set correctly")
				t.Diagnosticf("expect: %d, actual: %d", c.limit, lhl.Limit)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func testWrongHugetlb() error {
	// We deliberately set the page size to a wrong value, "3MB", to see
	// if the container really returns an error.
	page := "3MB"
	var limit uint64 = 100 * 3 * 1024 * 1024

	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	g.SetLinuxCgroupsPath(cgroups.AbsCgroupPath)
	g.AddLinuxResourcesHugepageLimit(page, limit)

	err = util.RuntimeOutsideValidate(g, t, func(config *rspec.Spec, t *tap.T, state *rspec.State) error {
		return nil
	})
	t.Ok(err != nil, "hugepage invalid pagesize results in an errror")
	if err == nil {
		t.Diagnosticf("expect: err != nil, actual: err == nil")
	}
	return err
}

func main() {
	if "linux" != runtime.GOOS {
		util.Fatal(fmt.Errorf("linux-specific cgroup test"))
	}

	if err := testHugetlbCgroups(); err != nil {
		util.Fatal(err)
	}

	if err := testWrongHugetlb(); err == nil {
		util.Fatal(err)
	}
}
