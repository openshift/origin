package main

import (
	"os"
	"runtime"

	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	if "windows" == runtime.GOOS {
		util.Skip("non-Windows root.readonly test", map[string]string{"OS": runtime.GOOS})
		os.Exit(0)
	}

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetRootReadonly(true)
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
