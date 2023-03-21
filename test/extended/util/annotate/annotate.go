package main

import (
	"strings"

	"k8s.io/kubernetes/openshift-hack/e2e/annotate"

	// this ensures that all origin tests are picked by ginkgo as defined
	// in test/extended/include.go
	_ "github.com/openshift/origin/test/extended"
)

func main() {
	annotate.Run(testMaps, func(name string) bool {
		return strings.Contains(name, "[Suite:k8s]")
	})
}
