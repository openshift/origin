package main

import (
	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/cgroups"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	page := "1GB"
	var limit uint64 = 56892210544640

	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetLinuxCgroupsPath(cgroups.RelCgroupPath)
	g.AddLinuxResourcesHugepageLimit(page, limit)
	err = util.RuntimeOutsideValidate(g, t, func(config *rspec.Spec, t *tap.T, state *rspec.State) error {
		cg, err := cgroups.FindCgroup()
		t.Ok((err == nil), "find hugetlb cgroup")
		if err != nil {
			t.Diagnostic(err.Error())
			return nil
		}

		lhd, err := cg.GetHugepageLimitData(state.Pid, config.Linux.CgroupsPath)
		t.Ok((err == nil), "get hugetlb cgroup data")
		if err != nil {
			t.Diagnostic(err.Error())
			return nil
		}

		found := false
		for _, lhl := range lhd {
			if lhl.Pagesize == page {
				found = true
				t.Ok(lhl.Limit == limit, "hugepage limit is set correctly")
				t.Diagnosticf("expect: %d, actual: %d", limit, lhl.Limit)
			}
		}
		t.Ok(found, "hugepage limit found")

		return nil
	})

	if err != nil {
		t.Fail(err.Error())
	}
}
