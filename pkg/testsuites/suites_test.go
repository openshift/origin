package testsuites

import (
	"fmt"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

// TestSuiteQualifiersValidCEL validates that all CEL expressions in suite qualifiers
// are syntactically valid
func TestSuiteQualifiersValidCEL(t *testing.T) {
	dummyTest := &extensiontests.ExtensionTestSpec{
		Name:   "[sig-test] Test a thing [Suite:openshift/conformance/parallel] [Early]",
		Source: "openshift:payload:origin",
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

func TestThirdPartySuiteMatchesHyperkubeTests(t *testing.T) {
	var thirdPartySuite *ginkgo.TestSuite
	for i := range staticSuites {
		if staticSuites[i].Name == "openshift/network/third-party" {
			thirdPartySuite = &staticSuites[i]
			break
		}
	}
	if thirdPartySuite == nil {
		t.Fatal("openshift/network/third-party suite not found")
	}

	candidateTests := extensiontests.ExtensionTestSpecs{
		{
			Name:   "[sig-network] NetworkPolicy API should support creating NetworkPolicy API operations [Conformance]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] Networking should provide Internet connection for containers [Feature:Networking-IPv4] [Conformance]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] NetworkPolicy should enforce policy based on Ports [Feature:IPv6DualStack]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] NetworkPolicy named port should not match",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-storage] some storage test [Conformance]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] some origin network test [Conformance]",
			Source: "openshift:payload:origin",
		},
	}

	filtered, err := candidateTests.Filter(thirdPartySuite.Qualifiers)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	expectedNames := map[string]bool{
		"[sig-network] NetworkPolicy API should support creating NetworkPolicy API operations [Conformance]":                 true,
		"[sig-network] Networking should provide Internet connection for containers [Feature:Networking-IPv4] [Conformance]": true,
		"[sig-network] NetworkPolicy should enforce policy based on Ports [Feature:IPv6DualStack]":                           true,
	}

	if len(filtered) != len(expectedNames) {
		t.Errorf("expected %d tests, got %d", len(expectedNames), len(filtered))
		for _, s := range filtered {
			t.Logf("  matched: %s", s.Name)
		}
	}
	for _, s := range filtered {
		if !expectedNames[s.Name] {
			t.Errorf("unexpected test matched: %s", s.Name)
		}
	}
}
