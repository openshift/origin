package extensions

import (
	"fmt"
	"strings"

	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

func addLabelsToSpecs(specs et.ExtensionTestSpecs) {
	var namesByLabel = map[string][]string{
		// tests that must be run without competition
		"[Serial]": {
			"[Disruptive]",
			"[sig-network][Feature:EgressIP]",
		},
		// tests that can't be run in parallel with a copy of itself
		"[Serial:Self]": {
			"[sig-network] HostPort validates that there is no conflict between pods with same hostPort but different hostIP and protocol",
		},
	}

	for label, names := range namesByLabel {
		var selectFunctions []et.SelectFunction
		for _, name := range names {
			selectFunctions = append(selectFunctions, et.NameContains(name))
		}

		// Add the label AND append it to the name (if it isn't present already)
		matching := specs.SelectAny(selectFunctions)
		for _, spec := range matching {
			if !strings.Contains(spec.Name, label) {
				spec.Name = fmt.Sprintf("%s %s", spec.Name, label)
			}
			spec.Labels.Insert(label)
		}
	}
}
