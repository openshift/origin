package gotest

import (
	"regexp"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

func newTestDataParser() testDataParser {
	return testDataParser{
		// testStartPattern matches the line in verbose `go test` output that marks the declaration of a test.
		// The first submatch of this regex is the name of the test
		testStartPattern: regexp.MustCompile(`=== RUN\s+(.+)$`),

		// testResultPattern matches the line in verbose `go test` output that marks the result of a test.
		// The first submatch of this regex is the result of the test (PASS, FAIL, or SKIP)
		// The second submatch of this regex is the name of the test
		// The third submatch of this regex is the time taken in seconds for the test to finish
		testResultPattern: regexp.MustCompile(`--- (PASS|FAIL|SKIP):\s+(.+)\s+\((\d+\.\d+)(s| seconds)\)`),
	}
}

type testDataParser struct {
	testStartPattern  *regexp.Regexp
	testResultPattern *regexp.Regexp
}

// MarksBeginning determines if the line marks the beginning of a test case
func (p *testDataParser) MarksBeginning(line string) bool {
	return p.testStartPattern.MatchString(line)
}

// ExtractName extracts the name of the test case from test output line
func (p *testDataParser) ExtractName(line string) (string, bool) {
	if matches := p.testStartPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		return matches[1], true
	}

	if matches := p.testResultPattern.FindStringSubmatch(line); len(matches) > 2 && len(matches[2]) > 0 {
		return matches[2], true
	}

	return "", false
}

// ExtractResult extracts the test result from a test output line
func (p *testDataParser) ExtractResult(line string) (api.TestResult, bool) {
	if matches := p.testResultPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		switch matches[1] {
		case "PASS":
			return api.TestResultPass, true
		case "SKIP":
			return api.TestResultSkip, true
		case "FAIL":
			return api.TestResultFail, true
		}
	}
	return "", false
}

// ExtractDuration extracts the test duration from a test output line
func (p *testDataParser) ExtractDuration(line string) (string, bool) {
	if matches := p.testResultPattern.FindStringSubmatch(line); len(matches) > 3 && len(matches[3]) > 0 {
		return matches[3] + "s", true
	}
	return "", false
}

func newTestSuiteDataParser() testSuiteDataParser {
	return testSuiteDataParser{
		// coverageOutputPattern matches coverage output on a single line.
		// The first submatch of this regex is the percent coverage
		coverageOutputPattern: regexp.MustCompile(`coverage:\s+(\d+\.\d+)\% of statements`),

		// packageResultPattern matches the `go test` output for the end of a package.
		// The first submatch of this regex matches the result of the test (ok or FAIL)
		// The second submatch of this regex matches the name of the package
		// The third submatch of this regex matches the time taken in seconds for tests in the package to finish
		// The sixth (optional) submatch of this regex is the percent coverage
		packageResultPattern: regexp.MustCompile(`(ok|FAIL)\s+(.+)[\s\t]+(\d+\.\d+(s| seconds))([\s\t]+coverage:\s+(\d+\.\d+)\% of statements)?`),
	}
}

type testSuiteDataParser struct {
	coverageOutputPattern *regexp.Regexp
	packageResultPattern  *regexp.Regexp
}

// ExtractName extracts the name of the test suite from a test output line
func (p *testSuiteDataParser) ExtractName(line string) (string, bool) {
	if matches := p.packageResultPattern.FindStringSubmatch(line); len(matches) > 2 && len(matches[2]) > 0 {
		return matches[2], true
	}
	return "", false
}

// ExtractDuration extracts the package duration from a test output line
func (p *testSuiteDataParser) ExtractDuration(line string) (string, bool) {
	if resultMatches := p.packageResultPattern.FindStringSubmatch(line); len(resultMatches) > 3 && len(resultMatches[3]) > 0 {
		return resultMatches[3], true
	}
	return "", false
}

const (
	coveragePropertyName string = "coverage.statements.pct"
)

// ExtractProperties extracts any metadata properties of the test suite from a test output line
func (p *testSuiteDataParser) ExtractProperties(line string) (map[string]string, bool) {
	// the only test suite properties that Go testing can create are coverage values, which can either
	// be present on their own line or in the package result line
	if matches := p.coverageOutputPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		return map[string]string{
			coveragePropertyName: matches[1],
		}, true
	}

	if resultMatches := p.packageResultPattern.FindStringSubmatch(line); len(resultMatches) > 6 && len(resultMatches[6]) > 0 {
		return map[string]string{
			coveragePropertyName: resultMatches[6],
		}, true
	}
	return map[string]string{}, false
}

// MarksCompletion determines if the line marks the completion of a test suite
func (p *testSuiteDataParser) MarksCompletion(line string) bool {
	return p.packageResultPattern.MatchString(line)
}
