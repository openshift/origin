package main

import (
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	mount := rspec.Mount{
		Destination: "/tmp",
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options: []string{
			"nosuid",
			"strictatime",
			"mode=755",
			"size=1k",
		},
	}
	g.AddMount(mount)
	err = util.RuntimeInsideValidate(g, nil)
	if err != nil {
		util.Fatal(err)
	}
}
