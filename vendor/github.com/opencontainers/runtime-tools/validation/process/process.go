package main

import (
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetProcessCwd("/test")
	g.AddProcessEnv("testa", "valuea")
	g.AddProcessEnv("testb", "123")

	err = util.RuntimeInsideValidate(g, nil, func(path string) error {
		pathName := filepath.Join(path, "test")
		return os.MkdirAll(pathName, 0700)
	})
	if err != nil {
		util.Fatal(err)
	}
}
