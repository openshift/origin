package extensions

import (
	"strings"

	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

// appendSuiteNames appends suite names to the end of the test names.
// This will happen unless:
//  1. The spec already has [Suite:xyz] in its name
//  2. The spec has an annotation present in the ExcludedTests
//  3. It is a dynamically selected "External Storage" test
func appendSuiteNames(specs et.ExtensionTestSpecs) {
	// Append suite info to the test names where relevant
	specs.Walk(func(spec *et.ExtensionTestSpec) {
		// Don't add suite info to the name if it is already present
		if strings.Contains(spec.Name, "[Suite:") {
			return
		}
		// If the name contains any of the Excluded patterns, don't add any suite info
		for _, exclusion := range ExcludedTests {
			if strings.Contains(spec.Name, exclusion) {
				return
			}
		}
		// Don't add suites to dynamically determined External Storage tests
		if strings.HasPrefix(spec.Name, "External Storage") {
			return
		}
		isSerial := strings.Contains(spec.Name, "[Serial]") || spec.Labels.Has("[Serial]")
		isConformance := strings.Contains(spec.Name, "[Conformance]") || spec.Labels.Has("[Conformance]")
		var suite string
		switch {
		case isSerial && isConformance:
			suite = " [Suite:openshift/conformance/serial/minimal]"
		case isSerial:
			suite = " [Suite:openshift/conformance/serial]"
		case isConformance:
			suite = " [Suite:openshift/conformance/parallel/minimal]"
		default:
			suite = " [Suite:openshift/conformance/parallel]"
		}
		spec.Name += suite
	})
}
