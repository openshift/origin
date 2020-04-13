package ginkgo

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/version"
)

// The below types are directly marshalled into XML. The types correspond to jUnit
// XML schema, but do not contain all valid fields. For instance, the class name
// field for test cases is omitted, as this concept does not directly apply to Go.
// For XML specifications see http://help.catchsoftware.com/display/ET/JUnit+Format
// or view the XSD included in this package as 'junit.xsd'

// TestSuites represents a flat collection of jUnit test suites.
type JUnitTestSuites struct {
	XMLName xml.Name `xml:"testsuites"`

	// Suites are the jUnit test suites held in this collection
	Suites []*JUnitTestSuite `xml:"testsuite"`
}

// TestSuite represents a single jUnit test suite, potentially holding child suites.
type JUnitTestSuite struct {
	XMLName xml.Name `xml:"testsuite"`

	// Name is the name of the test suite
	Name string `xml:"name,attr"`

	// NumTests records the number of tests in the TestSuite
	NumTests uint `xml:"tests,attr"`

	// NumSkipped records the number of skipped tests in the suite
	NumSkipped uint `xml:"skipped,attr"`

	// NumFailed records the number of failed tests in the suite
	NumFailed uint `xml:"failures,attr"`

	// Duration is the time taken in seconds to run all tests in the suite
	Duration float64 `xml:"time,attr"`

	// Properties holds other properties of the test suite as a mapping of name to value
	Properties []*TestSuiteProperty `xml:"properties,omitempty"`

	// TestCases are the test cases contained in the test suite
	TestCases []*JUnitTestCase `xml:"testcase"`

	// Children holds nested test suites
	Children []*JUnitTestSuite `xml:"testsuite"`
}

// TestSuiteProperty contains a mapping of a property name to a value
type TestSuiteProperty struct {
	XMLName xml.Name `xml:"property"`

	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// JUnitTestCase represents a jUnit test case
type JUnitTestCase struct {
	XMLName xml.Name `xml:"testcase"`

	// Name is the name of the test case
	Name string `xml:"name,attr"`

	// Classname is an attribute set by the package type and is required
	Classname string `xml:"classname,attr,omitempty"`

	// Duration is the time taken in seconds to run the test
	Duration float64 `xml:"time,attr"`

	// SkipMessage holds the reason why the test was skipped
	SkipMessage *SkipMessage `xml:"skipped"`

	// FailureOutput holds the output from a failing test
	FailureOutput *FailureOutput `xml:"failure"`

	// SystemOut is output written to stdout during the execution of this test case
	SystemOut string `xml:"system-out,omitempty"`

	// SystemErr is output written to stderr during the execution of this test case
	SystemErr string `xml:"system-err,omitempty"`
}

// SkipMessage holds a message explaining why a test was skipped
type SkipMessage struct {
	XMLName xml.Name `xml:"skipped"`

	// Message explains why the test was skipped
	Message string `xml:"message,attr,omitempty"`
}

// FailureOutput holds the output from a failing test
type FailureOutput struct {
	XMLName xml.Name `xml:"failure"`

	// Message holds the failure message from the test
	Message string `xml:"message,attr"`

	// Output holds verbose failure output from the test
	Output string `xml:",chardata"`
}

// TestResult is the result of a test case
type TestResult string

const (
	TestResultPass TestResult = "pass"
	TestResultSkip TestResult = "skip"
	TestResultFail TestResult = "fail"
)

func writeJUnitReport(filePrefix, name string, tests []*testCase, dir string, duration time.Duration, errOut io.Writer, additionalResults ...*JUnitTestCase) error {
	s := &JUnitTestSuite{
		Name:     name,
		Duration: duration.Seconds(),
		Properties: []*TestSuiteProperty{
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
			s.TestCases = append(s.TestCases, &JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.out),
				Duration:  test.duration.Seconds(),
				SkipMessage: &SkipMessage{
					Message: lastLinesUntil(string(test.out), 100, "skip ["),
				},
			})
		case test.failed:
			s.NumTests++
			s.NumFailed++
			s.TestCases = append(s.TestCases, &JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.out),
				Duration:  test.duration.Seconds(),
				FailureOutput: &FailureOutput{
					Output: lastLinesUntil(string(test.out), 100, "fail ["),
				},
			})
		case test.success:
			s.NumTests++
			s.TestCases = append(s.TestCases, &JUnitTestCase{
				Name:     test.name,
				Duration: test.duration.Seconds(),
			})
		}
	}
	for _, result := range additionalResults {
		switch {
		case result.SkipMessage != nil:
			s.NumSkipped++
		case result.FailureOutput != nil:
			s.NumFailed++
		}
		s.NumTests++
		s.TestCases = append(s.TestCases, result)
	}
	out, err := xml.Marshal(s)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.xml", filePrefix, time.Now().UTC().Format("20060102-150405")))
	fmt.Fprintf(errOut, "Writing JUnit report to %s\n\n", path)
	return ioutil.WriteFile(path, out, 0640)
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
