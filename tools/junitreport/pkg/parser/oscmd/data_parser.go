package oscmd

import (
	"regexp"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/parser/stack"
)

func newTestDataParser() stack.TestDataParser {
	return &testDataParser{
		// testStartPattern matches the test beginning bookend
		testStartPattern: regexp.MustCompile(`=== BEGIN TEST CASE ===`),

		// testDeclarationPattern matches the test declaration line, the full match is the name of the test being run
		// as we're starting the test declaration line with a Unix path, misprinting a leading newline will cause this
		// pattern to match the entire misprinted line
		testDeclarationPattern: regexp.MustCompile(`.+:[0-9]+: executing '.+' expecting .+`),

		// testConclusionPattern matches the test conclusion line, and contains the following submatches:
		//  - 1: test result
		//  - 2: test duration
		//  - 3: test name
		//  - 5: test result message
		// In order to make this regex sane, we *require* a end-line anchor and therefore make us a little more fragile
		// in the face of broken input
		testConclusionPattern: regexp.MustCompile(`(SUCCESS|FAILURE) after ([0-9]+\.[0-9]+s): (.+:[0-9]+: executing '.*' expecting .*?)(: (.*))?$`),

		// testEndPattern matches the test end bookend
		testEndPattern: regexp.MustCompile(`=== END TEST CASE ===`),
	}
}

type testDataParser struct {
	testStartPattern       *regexp.Regexp
	testDeclarationPattern *regexp.Regexp
	testConclusionPattern  *regexp.Regexp
	testEndPattern         *regexp.Regexp
}

// MarksBeginning determines if the line marks the beginning of a test case
func (p *testDataParser) MarksBeginning(line string) bool {
	return p.testStartPattern.MatchString(line)
}

// ExtractName extracts the name of the test case from test output lines
func (p *testDataParser) ExtractName(line string) (string, bool) {
	// The test declaration pattern is technically a subset of the test conclusion pattern, and will therefore
	// match anything the test declaration pattern matches. The match from the test conclusion pattern is more
	// correct, if it exists, so we check the conclusion pattern first and return if we have a name candidate.
	if matches := p.testConclusionPattern.FindStringSubmatch(line); len(matches) > 3 && len(matches[3]) > 0 {
		return matches[3], true
	}

	if matches := p.testDeclarationPattern.FindStringSubmatch(line); len(matches) > 0 && len(matches[0]) > 0 {
		return matches[0], true
	}

	return "", false
}

// ExtractResult extracts the test result from a test output line
func (p *testDataParser) ExtractResult(line string) (api.TestResult, bool) {
	if matches := p.testConclusionPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		switch matches[1] {
		case "SUCCESS":
			return api.TestResultPass, true
		case "FAILURE":
			return api.TestResultFail, true
		}
	}

	return "", false
}

// ExtractDuration extracts the test duration from a test output line
func (p *testDataParser) ExtractDuration(line string) (string, bool) {
	if matches := p.testConclusionPattern.FindStringSubmatch(line); len(matches) > 2 && len(matches[2]) > 0 {
		return matches[2], true
	}

	return "", false
}

// ExtractMessage extracts a message (e.g. for signalling why a failure or skip occurred) from a test output line
func (p *testDataParser) ExtractMessage(line string) (string, bool) {
	if matches := p.testConclusionPattern.FindStringSubmatch(line); len(matches) > 5 && len(matches[5]) > 0 {
		return matches[5], true
	}

	return "", false
}

// MarksCompletion determines if the line marks the completion of a test case
func (p *testDataParser) MarksCompletion(line string) bool {
	return p.testEndPattern.MatchString(line)
}

func newTestSuiteDataParser() stack.TestSuiteDataParser {
	return &testSuiteDataParser{
		// suiteDeclarationPattern matches the suite declaration line and has the following submatches:
		//  - 1: suite name
		suiteDeclarationPattern: regexp.MustCompile(`=== BEGIN TEST SUITE (.*) ===`),

		// suiteConclusionPattern matches the suite conclusion line
		suiteConclusionPattern: regexp.MustCompile(`=== END TEST SUITE ===`),
	}
}

type testSuiteDataParser struct {
	suiteDeclarationPattern *regexp.Regexp
	suiteConclusionPattern  *regexp.Regexp
}

// MarksBeginning determines if the line marks the beginning of a test suite
func (p *testSuiteDataParser) MarksBeginning(line string) bool {
	return p.suiteDeclarationPattern.MatchString(line)
}

// ExtractName extracts the name of the test suite from a test output line
func (p *testSuiteDataParser) ExtractName(line string) (string, bool) {
	if matches := p.suiteDeclarationPattern.FindStringSubmatch(line); len(matches) > 1 && len(matches[1]) > 0 {
		return matches[1], true
	}

	return "", false
}

// ExtractProperties extracts any metadata properties of the test suite from a test output line
func (p *testSuiteDataParser) ExtractProperties(line string) (map[string]string, bool) {
	// `os::cmd` suites cannot expose properties
	return map[string]string{}, false
}

// MarksCompletion determines if the line marks the completion of a test suite
func (p *testSuiteDataParser) MarksCompletion(line string) bool {
	return p.suiteConclusionPattern.MatchString(line)
}
