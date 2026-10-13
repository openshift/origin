package h2spec

// These types have been copied from h2spec.

import (
	"encoding/xml"
)

// JUnitReport represents the JUnit XML format.
type JUnitTestReport struct {
	XMLName    xml.Name          `xml:"testsuites"`
	TestSuites []*JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents the testsuite element of JUnit XML format.
type JUnitTestSuite struct {
	XMLName   xml.Name         `xml:"testsuite"`
	Name      string           `xml:"name,attr"`
	Package   string           `xml:"package,attr"`
	ID        string           `xml:"id,attr"`
	Tests     int              `xml:"tests,attr"`
	Skipped   int              `xml:"skipped,attr"`
	Failures  int              `xml:"failures,attr"`
	Errors    int              `xml:"errors,attr"`
	TestCases []*JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents the testcase element of JUnit XML format.
type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Package   string        `xml:"package,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure"`
	Skipped   *JUnitSkipped `xml:"skipped"`
	Error     *JUnitError   `xml:"error"`
}

// JUnitFailure represents the failure element of JUnit XML format.
type JUnitFailure struct {
	XMLName xml.Name `xml:"failure"`
	Content string   `xml:",innerxml"`
}

// JUnitSkipped represents the skipped element of JUnit XML format.
type JUnitSkipped struct {
	XMLName xml.Name `xml:"skipped"`
	Content string   `xml:",innerxml"`
}

// JUnitSkipped represents the error element of JUnit XML format.
type JUnitError struct {
	XMLName xml.Name `xml:"error"`
	Content string   `xml:",innerxml"`
}
