package junitapi

import (
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

// ToExtensionTestResults converts JUnit test cases to ExtensionTestResults
// so they can be included in the extension test result JSON/HTML output.
func ToExtensionTestResults(junits []*JUnitTestCase) extensiontests.ExtensionTestResults {
	var results extensiontests.ExtensionTestResults

	for _, junit := range junits {
		if junit == nil {
			continue
		}

		result := &extensiontests.ExtensionTestResult{
			Name:      junit.Name,
			Lifecycle: extensiontests.LifecycleBlocking,
			Duration:  int64(junit.Duration * 1000), // Convert seconds to milliseconds
		}

		// Determine the result status
		switch {
		case junit.SkipMessage != nil:
			result.Result = extensiontests.ResultSkipped
			result.Output = junit.SystemOut
			if junit.SkipMessage.Message != "" {
				result.Output = junit.SkipMessage.Message + "\n\n" + result.Output
			}
		case junit.FailureOutput != nil:
			result.Result = extensiontests.ResultFailed
			result.Output = junit.SystemOut
			result.Error = junit.FailureOutput.Output
		default:
			result.Result = extensiontests.ResultPassed
			result.Output = junit.SystemOut
		}

		// Parse lifecycle if present in the JUnit metadata
		if junit.Lifecycle != "" {
			result.Lifecycle = extensiontests.Lifecycle(junit.Lifecycle)
		}

		// Parse start time if present
		if junit.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, junit.StartTime); err == nil {
				result.StartTime = dbtime.Ptr(t)
			}
		}

		// Parse end time if present
		if junit.EndTime != "" {
			if t, err := time.Parse(time.RFC3339, junit.EndTime); err == nil {
				result.EndTime = dbtime.Ptr(t)
			}
		}

		results = append(results, result)
	}

	return results
}
