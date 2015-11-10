package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"

	"github.com/jstemmer/go-junit-report/parser"
)

// JUnitTestSuites is a collection of JUnit test suites.
type JUnitTestSuites struct {
	XMLName xml.Name `xml:"testsuites"`
	Suites  []JUnitTestSuite
}

// JUnitTestSuite is a single JUnit test suite which may contain many
// testcases.
type JUnitTestSuite struct {
	XMLName    xml.Name        `xml:"testsuite"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Time       string          `xml:"time,attr"`
	Name       string          `xml:"name,attr"`
	Properties []JUnitProperty `xml:"properties>property,omitempty"`
	TestCases  []JUnitTestCase

	Children []JUnitTestSuite
}

// JUnitTestCase is a single test case with its result.
type JUnitTestCase struct {
	XMLName     xml.Name          `xml:"testcase"`
	Classname   string            `xml:"classname,attr"`
	Name        string            `xml:"name,attr"`
	Time        string            `xml:"time,attr"`
	SkipMessage *JUnitSkipMessage `xml:"skipped,omitempty"`
	Failure     *JUnitFailure     `xml:"failure,omitempty"`
}

// JUnitSkipMessage contains the reason why a testcase was skipped.
type JUnitSkipMessage struct {
	Message string `xml:"message,attr"`
}

// JUnitProperty represents a key/value pair used to define properties.
type JUnitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// JUnitFailure contains data related to a failed test.
type JUnitFailure struct {
	Message  string `xml:"message,attr"`
	Type     string `xml:"type,attr"`
	Contents string `xml:",chardata"`
}

// JUnitReportXML writes a JUnit xml representation of the given report to w
// in the format described at http://windyroad.org/dl/Open%20Source/JUnit.xsd
func JUnitReportXML(report *parser.Report, noXMLHeader bool, w io.Writer) error {
	suites := JUnitTestSuites{}

	// convert Report to JUnit test suites
	for _, pkg := range report.Packages {
		suites.Suites = append(suites.Suites, convertToExternalRepresentation(pkg))
	}

	// to xml
	bytes, err := xml.MarshalIndent(suites, "", "\t")
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(w)

	if !noXMLHeader {
		writer.WriteString(xml.Header)
	}

	writer.Write(bytes)
	writer.WriteByte('\n')
	writer.Flush()

	return nil
}

func convertToExternalRepresentation(input *parser.Package) JUnitTestSuite {
	ts := JUnitTestSuite{
		Tests:      len(input.Tests),
		Failures:   0,
		Time:       formatTime(input.Time),
		Name:       input.Name,
		Properties: []JUnitProperty{},
		TestCases:  []JUnitTestCase{},
		Children:   []JUnitTestSuite{},
	}

	classname := input.Name
	if idx := strings.LastIndex(classname, "/"); idx > -1 && idx < len(input.Name) {
		classname = input.Name[idx+1:]
	}

	// properties
	ts.Properties = append(ts.Properties, JUnitProperty{"go.version", runtime.Version()})
	if input.CoveragePct != "" {
		ts.Properties = append(ts.Properties, JUnitProperty{"coverage.statements.pct", input.CoveragePct})
	}

	// individual test cases
	for _, test := range input.Tests {
		testCase := JUnitTestCase{
			Classname: classname,
			Name:      test.Name,
			Time:      formatTime(test.Time),
			Failure:   nil,
		}

		if test.Result == parser.FAIL {
			ts.Failures++
			testCase.Failure = &JUnitFailure{
				Message:  "Failed",
				Type:     "",
				Contents: strings.Join(test.Output, "\n"),
			}
		}

		if test.Result == parser.SKIP {
			testCase.SkipMessage = &JUnitSkipMessage{strings.Join(test.Output, "\n")}
		}

		ts.TestCases = append(ts.TestCases, testCase)
	}

	// child packages
	for _, child := range input.Children {
		ts.Children = append(ts.Children, convertToExternalRepresentation(child))
	}

	// update metrics from children
	for _, child := range ts.Children {
		ts.Tests = ts.Tests + child.Tests
		ts.Failures = ts.Failures + child.Failures
		ts.Time = strconv.FormatFloat(atof(ts.Time)+atof(child.Time), 'f', 3, 64)
	}
	return ts
}

func countFailures(tests []parser.Test) (result int) {
	for _, test := range tests {
		if test.Result == parser.FAIL {
			result++
		}
	}
	return
}

func formatTime(time int) string {
	return fmt.Sprintf("%.3f", float64(time)/1000.0)
}

// atof converts a string representing a float64 to the float
func atof(value string) float64 {
	// we discard the error as we know our time values are floats
	number, _ := strconv.ParseFloat(value, 64)
	return number
}
