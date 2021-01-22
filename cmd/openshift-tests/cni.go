package main

import (
	"strings"
)

// Determines whether a test should be run for third-party network plugin conformance testing
func inCNISuite(name string) bool {
	if strings.Contains(name, "[Suite:k8s]") && strings.Contains(name, "[sig-network]") {
		// Run all upstream sig-network conformance tests
		if strings.Contains(name, "[Conformance]") {
			return true
		}
		// Run all upstream NetworkPolicy tests except named port tests. (Neither
		// openshift-sdn nor ovn-kubernetes supports named ports in NetworkPolicy,
		// so we don't require third party tests to support them either.)
		if strings.Contains(name, "NetworkPolicy") && !strings.Contains(name, "named port") {
			return true
		}
	}

	return false
}
