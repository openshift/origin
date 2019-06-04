package util

import (
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func DumpNodeStates(oc *CLI) {
	e2e.Logf("Dumping node states")
	out, err := oc.AsAdmin().Run("get").Args("nodes", "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Unable to retrieve node states: %v", err)
		return
	}
	e2e.Logf(out)
}
