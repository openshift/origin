package stack

import "github.com/openshift/origin/tools/junitreport/pkg/api"

// TestDataParser knows how to take raw test data and extract the useful information from it
type TestDataParser interface {
	// MarksBeginning determines if the line marks the beginning of a test case
	MarksBeginning(line string) bool

	// ExtractName extracts the name of the test case from test output lines
	ExtractName(line string) (name string, succeeded bool)

	// ExtractResult extracts the test result from a test output line
	ExtractResult(line string) (result api.TestResult, succeeded bool)

	// ExtractDuration extracts the test duration from a test output line
	ExtractDuration(line string) (duration string, succeeded bool)

	// ExtractMessage extracts a message (e.g. for signalling why a failure or skip occurred) from a test output line
	ExtractMessage(line string) (message string, succeeded bool)

	// MarksCompletion determines if the line marks the completion of a test case
	MarksCompletion(line string) bool
}

// TestSuiteDataParser knows how to take raw test suite data and extract the useful information from it
type TestSuiteDataParser interface {
	// MarksBeginning determines if the line marks the beginning of a test suite
	MarksBeginning(line string) bool

	// ExtractName extracts the name of the test suite from a test output line
	ExtractName(line string) (name string, succeeded bool)

	// ExtractProperties extracts any metadata properties of the test suite from a test output line
	ExtractProperties(line string) (properties map[string]string, succeeded bool)

	// MarksCompletion determines if the line marks the completion of a test suite
	MarksCompletion(line string) bool
}
