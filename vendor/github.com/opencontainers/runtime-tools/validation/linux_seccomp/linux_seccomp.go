package main

import (
	"github.com/opencontainers/runtime-tools/generate/seccomp"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	syscallArgs := seccomp.SyscallOpts{
		Action:  "errno",
		Syscall: "getcwd",
	}
	g.SetDefaultSeccompAction("allow")
	g.SetSyscallAction(syscallArgs)
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
