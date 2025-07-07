package testsuites

import (
	"fmt"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

// TestSuiteQualifiersValidCEL validates that all CEL expressions in suite qualifiers
// are syntactically valid
func TestSuiteQualifiersValidCEL(t *testing.T) {
	dummyTest := &extensiontests.ExtensionTestSpec{
		Name: "[sig-test] Test a thing [Suite:openshift/conformance/parallel] [Early]",
	}
	dummySpecs := extensiontests.ExtensionTestSpecs{dummyTest}

	t.Run("standard suites", func(t *testing.T) {
		for _, suite := range staticSuites {
			t.Run(suite.Name, func(t *testing.T) {
				for i, qualifier := range suite.Qualifiers {
					t.Run(fmt.Sprintf("qualifier_%d", i), func(t *testing.T) {
						// Attempt to filter using the qualifier - this will validate the CEL expression
						_, err := dummySpecs.Filter([]string{qualifier})
						if err != nil {
							t.Errorf("Invalid CEL expression in suite %q, qualifier %d: %q\nError: %v",
								suite.Name, i, qualifier, err)
						}
					})
				}
			})
		}
	})

	t.Run("upgrade suites", func(t *testing.T) {
		for _, suite := range upgradeSuites {
			t.Run(suite.Name, func(t *testing.T) {
				for i, qualifier := range suite.Qualifiers {
					t.Run(fmt.Sprintf("qualifier_%d", i), func(t *testing.T) {
						// Attempt to filter using the qualifier - this will validate the CEL expression
						_, err := dummySpecs.Filter([]string{qualifier})
						if err != nil {
							t.Errorf("Invalid CEL expression in upgrade suite %q, qualifier %d: %q\nError: %v",
								suite.Name, i, qualifier, err)
						}
					})
				}
			})
		}
	})
}
