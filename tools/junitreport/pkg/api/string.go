package api

import "fmt"

// This file implements Stringer for the API types for ease of debugging

func (t *TestSuites) String() string {
	return fmt.Sprintf("Test Suites with suites: %s.", t.Suites)
}

func (t *TestSuite) String() string {
	childDescriptions := []string{}
	for _, child := range t.Children {
		childDescriptions = append(childDescriptions, child.String())
	}
	return fmt.Sprintf("Test Suite %q with properties: %s, %d test cases, of which %d failed and %d were skipped: %s, and children: %s.", t.Name, t.Properties, t.NumTests, t.NumFailed, t.NumSkipped, t.TestCases, childDescriptions)
}

func (t *TestCase) String() string {
	var result, message, output string
	result = "passed"
	if t.SkipMessage != nil {
		result = "skipped"
		message = t.SkipMessage.Message
	}
	if t.FailureOutput != nil {
		result = "failed"
		message = t.FailureOutput.Message
		output = t.FailureOutput.Output
	}

	return fmt.Sprintf("Test Case %q %s after %f seconds with message %q and output %q.", t.Name, result, t.Duration, message, output)
}

func (p *TestSuiteProperty) String() string {
	return fmt.Sprintf("%q=%q", p.Name, p.Value)
}
