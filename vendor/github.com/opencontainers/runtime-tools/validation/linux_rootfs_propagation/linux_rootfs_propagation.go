package main

import (
	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func testLinuxRootPropagation(t *tap.T, propMode string) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetupPrivileged(true)
	g.SetLinuxRootPropagation(propMode)
	g.AddAnnotation("TestName", "check root propagation")
	return util.RuntimeInsideValidate(g, t, nil)
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
		if err := testLinuxRootPropagation(t, c); err != nil {
			t.Fail(err.Error())
		}
	}
}
