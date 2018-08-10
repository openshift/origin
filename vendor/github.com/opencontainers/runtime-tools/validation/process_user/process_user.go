package main

import (
	"runtime"

	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}

	switch runtime.GOOS {
	case "linux", "solaris":
		g.SetProcessUID(10)
		g.SetProcessGID(10)
		g.AddProcessAdditionalGid(5)
	case "windows":
		g.SetProcessUsername("test")
	default:
	}

	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
