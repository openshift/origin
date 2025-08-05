package extensions

import (
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

		//TODO: once annotation logic has been removed, it might also be necessary to annotate the test name with the label as well
		specs.SelectAny(selectFunctions).AddLabel(label)
	}
}
