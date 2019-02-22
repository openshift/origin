package main

import (
	"fmt"
	"os"

	"github.com/openshift/library-go/cmd/crd-schema-gen/generator"
)

func main() {
	if err := generator.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
