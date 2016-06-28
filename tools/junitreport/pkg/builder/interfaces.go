package builder

import "github.com/openshift/origin/tools/junitreport/pkg/api"

// TestSuitesBuilder knows how to aggregate data to form a collection of test suites.
type TestSuitesBuilder interface {
	// AddSuite adds a test suite to the collection
	AddSuite(suite *api.TestSuite)

	// Build returns the built structure
	Build() *api.TestSuites
}
