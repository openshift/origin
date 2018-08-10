package main

import (
	"fmt"
	"os"
	"runtime"

	rspecs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
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
	g.AddProcessRlimits("RLIMIT_TEST", 1024, 1024)
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err == nil {
		util.Fatal(specerror.NewError(specerror.PosixProcRlimitsTypeGenError, fmt.Errorf("The runtime MUST generate an error for any values which cannot be mapped to a relevant kernel interface"), rspecs.Version))
	}
}
