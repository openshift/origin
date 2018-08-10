package main

import (
	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func testLinuxRootPropagation(propMode string) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetupPrivileged(true)
	g.SetLinuxRootPropagation(propMode)
	return util.RuntimeInsideValidate(g, nil)
}

func main() {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	cases := []string{
		"shared",
		"slave",
		"private",
		"unbindable",
	}

	for _, c := range cases {
		if err := testLinuxRootPropagation(c); err != nil {
			t.Fail(err.Error())
		}
	}
}
