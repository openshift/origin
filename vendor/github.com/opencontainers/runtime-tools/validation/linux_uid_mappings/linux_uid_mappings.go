package main

import (
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.AddOrReplaceLinuxNamespace("user", "")
	g.AddLinuxUIDMapping(uint32(1000), uint32(0), uint32(2000))
	g.AddLinuxGIDMapping(uint32(1000), uint32(0), uint32(3000))
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
