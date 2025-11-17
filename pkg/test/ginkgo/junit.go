package ginkgo

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/version"
)

// populateOTEMetadata adds OTE metadata attributes to a JUnit test case if available
func populateOTEMetadata(testCase *junitapi.JUnitTestCase, extensionResult *extensions.ExtensionTestResult) {
	if extensionResult == nil {
		return
	}

	// Test source information
	testCase.SourceImage = extensionResult.Source.SourceImage
	testCase.SourceBinary = extensionResult.Source.SourceBinary
	if extensionResult.Source.Source != nil {
		testCase.SourceURL = extensionResult.Source.SourceURL
		testCase.SourceCommit = extensionResult.Source.Commit
	}

	// Set lifecycle attribute
	testCase.Lifecycle = string(extensionResult.Lifecycle)

	// Set start time attribute if available
	if extensionResult.StartTime != nil {
		testCase.StartTime = time.Time(*extensionResult.StartTime).UTC().Format(time.RFC3339)
	}

	// Set end time attribute if available
	if extensionResult.EndTime != nil {
		testCase.EndTime = time.Time(*extensionResult.EndTime).UTC().Format(time.RFC3339)
	}
}

func generateJUnitTestSuiteResults(
	name string,
	duration time.Duration,
	tests []*testCase,
	syntheticTestResults ...*junitapi.JUnitTestCase) *junitapi.JUnitTestSuite {

	s := &junitapi.JUnitTestSuite{
		Name:     name,
		Duration: duration.Seconds(),
		Properties: []*junitapi.TestSuiteProperty{
			{
				Name:  "TestVersion",
				Value: version.Get().String(),
			},
		},
	}
	for _, test := range tests {
		switch {
		case test.skipped:
			s.NumTests++
			s.NumSkipped++
			testCase := &junitapi.JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.testOutputBytes),
				Duration:  test.duration.Seconds(),
				SkipMessage: &junitapi.SkipMessage{
					Message: lastLinesUntil(string(test.testOutputBytes), 100, "skip ["),
				},
			}
			populateOTEMetadata(testCase, test.extensionTestResult)
			s.TestCases = append(s.TestCases, testCase)
		case test.failed:
			s.NumTests++
			s.NumFailed++
			testCase := &junitapi.JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.testOutputBytes),
				Duration:  test.duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: lastLinesUntil(string(test.testOutputBytes), 100, "fail ["),
				},
			}
			populateOTEMetadata(testCase, test.extensionTestResult)
			s.TestCases = append(s.TestCases, testCase)
		case test.flake:
			s.NumTests++
			s.NumFailed++
			failedTestCase := &junitapi.JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.testOutputBytes),
				Duration:  test.duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: lastLinesUntil(string(test.testOutputBytes), 100, "flake:"),
				},
			}
			populateOTEMetadata(failedTestCase, test.extensionTestResult)
			s.TestCases = append(s.TestCases, failedTestCase)

			// also add the successful junit result:
			s.NumTests++
			successTestCase := &junitapi.JUnitTestCase{
				Name:     test.name,
				Duration: test.duration.Seconds(),
			}
			populateOTEMetadata(successTestCase, test.extensionTestResult)
			s.TestCases = append(s.TestCases, successTestCase)
		case test.success:
			s.NumTests++
			testCase := &junitapi.JUnitTestCase{
				Name:     test.name,
				Duration: test.duration.Seconds(),
			}
			populateOTEMetadata(testCase, test.extensionTestResult)
			s.TestCases = append(s.TestCases, testCase)
		}
	}
	for _, result := range syntheticTestResults {
		switch {
		case result.SkipMessage != nil:
			s.NumSkipped++
		case result.FailureOutput != nil:
			s.NumFailed++
		}
		s.NumTests++
		s.TestCases = append(s.TestCases, result)
	}
	return s
}

func writeJUnitReport(s *junitapi.JUnitTestSuite, filePrefix, fileSuffix, dir string, errOut io.Writer) error {
	out, err := xml.MarshalIndent(s, "", "    ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.xml", filePrefix, fileSuffix))
	fmt.Fprintf(errOut, "Writing JUnit report to %s\n\n", path)
	return ioutil.WriteFile(path, test.StripANSI(out), 0640)
}

func lastLinesUntil(output string, max int, until ...string) string {
	output = strings.TrimSpace(output)
	index := len(output) - 1
	if index < 0 || max == 0 {
		return output
	}
	for max > 0 {
		next := strings.LastIndex(output[:index], "\n")
		if next <= 0 {
			return strings.TrimSpace(output)
		}
		// empty lines don't count
		line := strings.TrimSpace(output[next+1 : index])
		if len(line) > 0 {
			max--
		}
		index = next
		if stringStartsWithAny(line, until) {
			break
		}
	}
	return strings.TrimSpace(output[index:])
}

func stringStartsWithAny(s string, contains []string) bool {
	for _, match := range contains {
		if strings.HasPrefix(s, match) {
			return true
		}
	}
	return false
}
