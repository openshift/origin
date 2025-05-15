package main

import (
	"strings"

	"k8s.io/kubernetes/openshift-hack/e2e/annotate"

	_ "github.com/openshift/origin/test/extended"
)

func main() {
	annotate.Run(testMaps, func(name string) bool {
		return strings.Contains(name, "[Suite:k8s]")
	})
}
