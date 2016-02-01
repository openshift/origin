package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/recycle"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
	basename := os.Args[0]
	args := os.Args[1:]
	if len(args) == 0 || len(args[0]) == 0 {
		fmt.Printf("Usage: %s DIR\n", basename)
		os.Exit(1)
	}

	if err := recycle.Recycle(args[0]); err != nil {
		fmt.Printf("Scrub failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Scrub OK")
	os.Exit(0)
}
