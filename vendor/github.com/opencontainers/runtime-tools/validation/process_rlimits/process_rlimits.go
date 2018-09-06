package main

import (
	"os"
	"runtime"

	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	if "linux" != runtime.GOOS && "solaris" != runtime.GOOS {
		util.Skip("POSIX-specific process.rlimits test", map[string]string{"OS": runtime.GOOS})
		os.Exit(0)
	}

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}

	var gigaBytes uint64 = 1024 * 1024 * 1024
	g.AddProcessRlimits("RLIMIT_AS", 2*gigaBytes, 1*gigaBytes)
	g.AddProcessRlimits("RLIMIT_CORE", 4*gigaBytes, 3*gigaBytes)
	g.AddProcessRlimits("RLIMIT_DATA", 6*gigaBytes, 5*gigaBytes)
	g.AddProcessRlimits("RLIMIT_FSIZE", 8*gigaBytes, 7*gigaBytes)
	g.AddProcessRlimits("RLIMIT_STACK", 10*gigaBytes, 9*gigaBytes)

	g.AddProcessRlimits("RLIMIT_CPU", 120, 60)       // seconds
	g.AddProcessRlimits("RLIMIT_NOFILE", 4000, 3000) // number of files
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
