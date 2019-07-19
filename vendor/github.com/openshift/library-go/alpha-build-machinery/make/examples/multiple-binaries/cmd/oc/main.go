package main

import (
	"fmt"

	"github.com/openshift/library-go/alpha-build-machinery/make/examples/multiple-binaries/pkg/version"
)

func main() {
	fmt.Print(version.String())
}
