package main

import (
	"fmt"
	"runtime"

	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func testHostname(t *tap.T, hostname string) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	g.SetHostname(hostname)
	g.AddAnnotation("TestName", fmt.Sprintf("check hostname %q", hostname))
	err = util.RuntimeInsideValidate(g, t, nil)
	t.Ok(err == nil, "hostname is set correctly")
	if err != nil {
		t.Diagnosticf("expect: err == nil, actual: err != nil")
	}

	return nil
}

func main() {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	if "linux" != runtime.GOOS {
		t.Skip(1, fmt.Sprintf("linux-specific namespace test"))
	}

	hostnames := []string{
		"",
		"hostname-specific",
	}

	for _, h := range hostnames {
		if err := testHostname(t, h); err != nil {
			t.Fail(err.Error())
		}
	}
}
