package gotest

import (
	"regexp"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

// testStartPattern matches the line in verbose `go test` output that marks the declaration of a test.
// The first submatch of this regex is the name of the test
var testStartPattern = regexp.MustCompile(`^=== RUN\s+([^\s]+)$`)

// ExtractRun identifies the start of a test output section.
func ExtractRun(line string) (string, bool) {
	if matches := testStartPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		return matches[1], true
	}
	return "", false
}

// testResultPattern matches the line in verbose `go test` output that marks the result of a test.
// The first submatch of this regex is the result of the test (PASS, FAIL, or SKIP)
// The second submatch of this regex is the name of the test
// The third submatch of this regex is the time taken in seconds for the test to finish
var testResultPattern = regexp.MustCompile(`^(\s*)--- (PASS|FAIL|SKIP):\s+([^\s]+)\s+\((\d+\.\d+)(s| seconds)\)$`)

// ExtractResult extracts the test result from a test output line. Depth is measured as the leading whitespace
// for the line multiplied by four, which is used to identify output from nested Go subtests.
func ExtractResult(line string) (r api.TestResult, name string, depth int, duration string, ok bool) {
	if matches := testResultPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[2]) > 0 {
		switch matches[2] {
		case "PASS":
			r = api.TestResultPass
		case "SKIP":
			r = api.TestResultSkip
		case "FAIL":
			r = api.TestResultFail
		default:
			return "", "", 0, "", false
		}
		name = matches[3]
		duration = matches[4] + "s"
		depth = len(matches[1]) / 4
		ok = true
		return
	}
	return "", "", 0, "", false
}

// testOutputPattern captures a line with leading whitespace.
var testOutputPattern = regexp.MustCompile(`^(\s*)(.*)$`)

// ExtractOutput captures a line of output indented by whitespace and returns
// the output, the indentation depth (4 spaces is the canonical indentation used by go test),
// and whether the match was successful.
func ExtractOutput(line string) (string, int, bool) {
	if matches := testOutputPattern.FindStringSubmatch(line); len(matches) > 1 {
		return matches[2], len(matches[1]) / 4, true
	}
	return "", 0, false
}

// coverageOutputPattern matches coverage output on a single line.
// The first submatch of this regex is the percent coverage
var coverageOutputPattern = regexp.MustCompile(`coverage:\s+(\d+\.\d+)\% of statements`)

// packageResultPattern matches the `go test` output for the end of a package.
// The first submatch of this regex matches the result of the test (ok or FAIL)
// The second submatch of this regex matches the name of the package
// The third submatch of this regex matches the time taken in seconds for tests in the package to finish
// The sixth (optional) submatch of this regex is the percent coverage
var packageResultPattern = regexp.MustCompile(`^(ok|FAIL)\s+(.+)[\s\t]+(\d+\.\d+(s| seconds))([\s\t]+coverage:\s+(\d+\.\d+)\% of statements)?$`)

// ExtractPackage extracts the name of the test suite from a test package line.
func ExtractPackage(line string) (name string, duration string, coverage string, ok bool) {
	if matches := packageResultPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[2]) > 0 {
		return matches[2], matches[3], matches[5], true
	}
	return "", "", "", false
}

// ExtractDuration extracts the package duration from a test output line
func ExtractDuration(line string) (string, bool) {
	if resultMatches := packageResultPattern.FindStringSubmatch(line); len(resultMatches) > 3 && len(resultMatches[3]) > 0 {
		return resultMatches[3], true
	}
	return "", false
}

const (
	coveragePropertyName string = "coverage.statements.pct"
)

// ExtractProperties extracts any metadata properties of the test suite from a test output line
func ExtractProperties(line string) (map[string]string, bool) {
	// the only test suite properties that Go testing can create are coverage values, which can either
	// be present on their own line or in the package result line
	if matches := coverageOutputPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		return map[string]string{
			coveragePropertyName: matches[1],
		}, true
	}

	if resultMatches := packageResultPattern.FindStringSubmatch(line); len(resultMatches) > 6 && len(resultMatches[6]) > 0 {
		return map[string]string{
			coveragePropertyName: resultMatches[6],
		}, true
	}
	return map[string]string{}, false
}
