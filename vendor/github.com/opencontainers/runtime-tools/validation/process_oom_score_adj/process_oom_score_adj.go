package main

import (
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetProcessOOMScoreAdj(500)
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
