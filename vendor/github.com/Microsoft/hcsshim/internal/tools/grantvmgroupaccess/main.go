package main

import (
	"fmt"
	"os"

	"github.com/Microsoft/go-winio/pkg/security"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: grantvmgroupaccess.exe file")
		os.Exit(-1)
	}
	if err := security.GrantVmGroupAccess(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}
