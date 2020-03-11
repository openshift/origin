package h2spec_test

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/origin/test/extended/router/h2spec"
)

func TestParseEmptyData(t *testing.T) {
	_, err := h2spec.DecodeJUnitReport(strings.NewReader(""))
	if err == nil {
		t.Fatalf("expected an error")
	}
	if expected := "EOF"; expected != err.Error() {
		t.Fatalf("expected %q, got %q", expected, err)
	}
}

func TestParseBadData(t *testing.T) {
	_, err := h2spec.DecodeJUnitReport(strings.NewReader("Not an XML document."))
	if err == nil {
		t.Fatal("expected an error")
	}
	if expected := "EOF"; expected != err.Error() {
		t.Fatalf("expected %q, got %q", expected, err)
	}
}

func TestParseEmptySuites(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites/>
`
	testsuites, err := h2spec.DecodeJUnitReport(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error; got %v", err)
	}

	if l := len(testsuites); l != 0 {
		t.Errorf("expected 0, got %v", l)
	}
}

func TestParseOneTestSuiteNoTestCase(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="Suite 1" package="generic/1" id="1" tests="0" skipped="0" failures="0" errors="0"/>
</testsuites>
`
	testsuites, err := h2spec.DecodeJUnitReport(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error; got %v", err)
	}

	if l := len(testsuites); l != 1 {
		t.Fatalf("expected 1, got %v", l)
	}

	expected := h2spec.JUnitTestSuite{
		XMLName:  xml.Name{Local: "testsuite"},
		Name:     "Suite 1",
		Package:  "generic/1",
		ID:       "1",
		Tests:    0,
		Skipped:  0,
		Failures: 0,
		Errors:   0,
	}

	if diff := cmp.Diff(*testsuites[0], expected); diff != "" {
		t.Errorf("%T differ (-got, +want): %s", testsuites[0], diff)
	}
}

func TestParseOneTestSuiteWithTestCases(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="3.5. HTTP/2 Connection Preface" package="http2/3.5" id="3.5" tests="2" skipped="0" failures="0" errors="1">
    <testcase package="http2/3.5" classname="Sends client connection preface" time="0.0002"></testcase>
    <testcase package="http2/3.5" classname="Sends invalid connection preface" time="0.0009">
      <error>GOAWAY Frame (Error Code: PROTOCOL_ERROR)
Connection closed
Error: unexpected EOF</error>
    </testcase>
  </testsuite>
</testsuites>
`
	testsuites, err := h2spec.DecodeJUnitReport(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error; got %v", err)
	}

	if l := len(testsuites); l != 1 {
		t.Fatalf("expected 1, got %v", l)
	}

	expected := h2spec.JUnitTestSuite{
		XMLName: xml.Name{
			Space: "",
			Local: "testsuite",
		},
		Name:     "3.5. HTTP/2 Connection Preface",
		Package:  "http2/3.5",
		ID:       "3.5",
		Tests:    2,
		Skipped:  0,
		Failures: 0,
		Errors:   1,
		TestCases: []*h2spec.JUnitTestCase{{
			XMLName:   xml.Name{Local: "testcase"},
			Package:   "http2/3.5",
			ClassName: "Sends client connection preface",
			Time:      "0.0002",
		}, {
			XMLName:   xml.Name{Local: "testcase"},
			Package:   "http2/3.5",
			ClassName: "Sends invalid connection preface",
			Time:      "0.0009",
			Failure:   nil,
			Skipped:   nil,
			Error: &h2spec.JUnitError{
				XMLName: xml.Name{Local: "error"},
				Content: "GOAWAY Frame (Error Code: PROTOCOL_ERROR)\nConnection closed\nError: unexpected EOF",
			},
		}},
	}

	if diff := cmp.Diff(*testsuites[0], expected); diff != "" {
		t.Errorf("%T differ (-got, +want): %s", *testsuites[0], diff)
	}
}

func TestParseOneTestSuiteShouldError(t *testing.T) {
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="3.5. HTTP/2 Connection Preface" package="http2/3.5" id="%s" tests="2" skipped="0" failures="0" errors="1">
    <testcase package="http2/3.5" classname="Sends client connection preface" time="0.0002"></testcase>
  </testsuite>
</testsuites>
`, "3.5")

	testsuites, err := h2spec.DecodeJUnitReport(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error; got %v", err)
	}

	if l := len(testsuites); l != 1 {
		t.Fatalf("expected 1, got %v", l)
	}

	expected := h2spec.JUnitTestSuite{
		XMLName: xml.Name{
			Space: "",
			Local: "testsuite",
		},
		Name:      "3.5. HTTP/2 Connection Preface",
		Package:   "http2/3.5",
		ID:        "changed from '3.5' to ensure we error",
		Tests:     2,
		Skipped:   0,
		Failures:  0,
		Errors:    0,
		TestCases: nil,
	}

	if diff := cmp.Diff(*testsuites[0], expected); diff == "" {
		t.Errorf("expected a difference in ID field")
	}
}
