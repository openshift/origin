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
	if "linux" != runtime.GOOS {
		util.Skip("linux-specific process.capabilities test", map[string]string{"OS": runtime.GOOS})
		os.Exit(0)
	}

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.AddProcessCapabilityBounding("CAP_TEST")
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err == nil {
		util.Fatal(specerror.NewError(specerror.LinuxProcCapError, fmt.Errorf("Any value which cannot be mapped to a relevant kernel interface MUST cause an error"), rspecs.Version))
	}
}
