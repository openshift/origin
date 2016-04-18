package gotest

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

func TestMarksTestBeginning(t *testing.T) {
	var testCases = []struct {
		name     string
		testLine string
	}{
		{
			name:     "basic",
			testLine: "=== RUN TestName",
		},
		{
			name:     "numeric",
			testLine: "=== RUN 1234",
		},
		{
			name:     "url",
			testLine: "=== RUN github.com/maintainer/repository/package/file",
		},
		{
			name:     "failed print",
			testLine: "some other text=== RUN github.com/maintainer/repository/package/file",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		if !parser.MarksBeginning(testCase.testLine) {
			t.Errorf("%s: did not correctly determine that line %q marked test beginning", testCase.name, testCase.testLine)
		}
	}
}

func TestExtractTestName(t *testing.T) {
	var testCases = []struct {
		name         string
		testLine     string
		expectedName string
	}{
		{
			name:         "basic start",
			testLine:     "=== RUN TestName",
			expectedName: "TestName",
		},
		{
			name:         "numeric",
			testLine:     "=== RUN 1234",
			expectedName: "1234",
		},
		{
			name:         "url",
			testLine:     "=== RUN github.com/maintainer/repository/package/file",
			expectedName: "github.com/maintainer/repository/package/file",
		},
		{
			name:         "basic end",
			testLine:     "--- PASS: Test (0.10 seconds)",
			expectedName: "Test",
		},
		{
			name:         "go1.5.1 timing",
			testLine:     "--- PASS: TestTwo (0.03s)",
			expectedName: "TestTwo",
		},
		{
			name:         "skip",
			testLine:     "--- SKIP: Test (0.10 seconds)",
			expectedName: "Test",
		},
		{
			name:         "fail",
			testLine:     "--- FAIL: Test (0.10 seconds)",
			expectedName: "Test",
		},
		{
			name:         "failed print",
			testLine:     "some other text--- FAIL: Test (0.10 seconds)",
			expectedName: "Test",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractName(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract name from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedName != actual {
			t.Errorf("%s: did not correctly extract name from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedName, actual)
		}
	}
}

func TestExtractResult(t *testing.T) {
	var testCases = []struct {
		name           string
		testLine       string
		expectedResult api.TestResult
	}{
		{
			name:           "basic",
			testLine:       "--- PASS: Test (0.10 seconds)",
			expectedResult: api.TestResultPass,
		},
		{
			name:           "go1.5.1 timing",
			testLine:       "--- PASS: TestTwo (0.03s)",
			expectedResult: api.TestResultPass,
		},
		{
			name:           "skip",
			testLine:       "--- SKIP: Test (0.10 seconds)",
			expectedResult: api.TestResultSkip,
		},
		{
			name:           "fail",
			testLine:       "--- FAIL: Test (0.10 seconds)",
			expectedResult: api.TestResultFail,
		},
		{
			name:           "failed print",
			testLine:       "some other text--- FAIL: Test (0.10 seconds)",
			expectedResult: api.TestResultFail,
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractResult(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract result from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedResult != actual {
			t.Errorf("%s: did not correctly extract result from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedResult, actual)
		}
	}
}

func TestExtractDuration(t *testing.T) {
	var testCases = []struct {
		name             string
		testLine         string
		expectedDuration string
	}{
		{
			name:             "basic",
			testLine:         "--- PASS: Test (0.10 seconds)",
			expectedDuration: "0.10s", // we make the conversion to time.Duration-parseable units internally
		},
		{
			name:             "go1.5.1 timing",
			testLine:         "--- PASS: TestTwo (0.03s)",
			expectedDuration: "0.03s",
		},
		{
			name:             "failed print",
			testLine:         "some other text--- PASS: TestTwo (0.03s)",
			expectedDuration: "0.03s",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractDuration(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract duration from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedDuration != actual {
			t.Errorf("%s: did not correctly extract duration from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedDuration, actual)
		}
	}
}

func TestExtractSuiteName(t *testing.T) {
	var testCases = []struct {
		name         string
		testLine     string
		expectedName string
	}{
		{
			name: "basic",
			testLine: "ok  	package/name 0.160s",
			expectedName: "package/name",
		},
		{
			name: "go 1.5.1",
			testLine: "ok  	package/name	0.160s",
			expectedName: "package/name",
		},
		{
			name: "numeric",
			testLine: "ok  	1234 0.160s",
			expectedName: "1234",
		},
		{
			name: "url",
			testLine: "ok  	github.com/maintainer/repository/package/file 0.160s",
			expectedName: "github.com/maintainer/repository/package/file",
		},
		{
			name: "with coverage",
			testLine: `ok  	package/name 0.400s  coverage: 10.0% of statements`,
			expectedName: "package/name",
		},
		{
			name: "failed print",
			testLine: `some other textok  	package/name 0.400s  coverage: 10.0% of statements`,
			expectedName: "package/name",
		},
	}

	parser := newTestSuiteDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractName(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract name from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedName != actual {
			t.Errorf("%s: did not correctly extract suite name from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedName, actual)
		}
	}
}

func TestSuiteProperties(t *testing.T) {
	var testCases = []struct {
		name               string
		testLine           string
		expectedProperties map[string]string
	}{
		{
			name:               "basic",
			testLine:           `coverage: 10.0% of statements`,
			expectedProperties: map[string]string{coveragePropertyName: "10.0"},
		},
		{
			name: "with package declaration",
			testLine: `ok  	package/name 0.400s  coverage: 10.0% of statements`,
			expectedProperties: map[string]string{coveragePropertyName: "10.0"},
		},
		{
			name:               "failed print",
			testLine:           `some other textcoverage: 10.0% of statements`,
			expectedProperties: map[string]string{coveragePropertyName: "10.0"},
		},
	}

	parser := newTestSuiteDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractProperties(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract properties from line %q", testCase.name, testCase.testLine)
		}
		if !reflect.DeepEqual(testCase.expectedProperties, actual) {
			t.Errorf("%s: did not correctly extract properties from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedProperties, actual)
		}
	}
}

func TestMarksCompletion(t *testing.T) {
	var testCases = []struct {
		name     string
		testLine string
	}{
		{
			name: "basic",
			testLine: "ok  	package/name 0.160s",
		},
		{
			name: "numeric",
			testLine: "ok  	1234 0.160s",
		},
		{
			name: "url",
			testLine: "ok  	github.com/maintainer/repository/package/file 0.160s",
		},
		{
			name: "with coverage",
			testLine: `ok  	package/name 0.400s  coverage: 10.0% of statements`,
		},
		{
			name: "failed print",
			testLine: `some other textok  	package/name 0.400s  coverage: 10.0% of statements`,
		},
	}

	parser := newTestSuiteDataParser()
	for _, testCase := range testCases {
		if !parser.MarksCompletion(testCase.testLine) {
			t.Errorf("%s: did not correctly determine that line %q marked the end of a suite", testCase.name, testCase.testLine)
		}
	}
}
