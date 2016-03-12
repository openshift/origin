package gotest

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/builder"
	"github.com/openshift/origin/tools/junitreport/pkg/parser"
)

// NewParser returns a new parser that's capable of parsing Go unit test output
func NewParser(builder builder.TestSuitesBuilder, stream bool) parser.TestOutputParser {
	return &testOutputParser{
		builder:     builder,
		testParser:  newTestDataParser(),
		suiteParser: newTestSuiteDataParser(),
		stream:      stream,
	}
}

type testOutputParser struct {
	builder     builder.TestSuitesBuilder
	testParser  testDataParser
	suiteParser testSuiteDataParser
	stream      bool
}

// Parse parses `go test -v` output into test suites. Test output from `go test -v` is not bookmarked for packages, so
// the parsing strategy is to advance line-by-line, building up a slice of test cases until a package declaration is found,
// at which point all tests cases are added to that package and the process can start again.
func (p *testOutputParser) Parse(input *bufio.Scanner) (*api.TestSuites, error) {
	currentSuite := &api.TestSuite{}
	var currentTest *api.TestCase
	var currentTestResult api.TestResult
	var currentTestOutput []string

	for input.Scan() {
		line := input.Text()
		isTestOutput := true

		if p.testParser.MarksBeginning(line) || p.suiteParser.MarksCompletion(line) {
			if currentTest != nil {
				// we can't mark the test as failed or skipped until we have all of the test output, which we don't know
				// we have until we see the next test or the beginning of suite output, so we add it here
				output := strings.Join(currentTestOutput, "\n")
				switch currentTestResult {
				case api.TestResultSkip:
					currentTest.MarkSkipped(output)
				case api.TestResultFail:
					currentTest.MarkFailed("", output)
				}

				currentSuite.AddTestCase(currentTest)
			}
			currentTest = &api.TestCase{}
			currentTestResult = api.TestResultFail
			currentTestOutput = []string{}
		}

		if name, matched := p.testParser.ExtractName(line); matched {
			currentTest.Name = name
		}

		if result, matched := p.testParser.ExtractResult(line); matched {
			currentTestResult = result
		}

		if duration, matched := p.testParser.ExtractDuration(line); matched {
			if err := currentTest.SetDuration(duration); err != nil {
				return nil, err
			}
		}

		if properties, matched := p.suiteParser.ExtractProperties(line); matched {
			for name := range properties {
				currentSuite.AddProperty(name, properties[name])
			}
			isTestOutput = false
		}

		if name, matched := p.suiteParser.ExtractName(line); matched {
			currentSuite.Name = name
			isTestOutput = false
		}

		if duration, matched := p.suiteParser.ExtractDuration(line); matched {
			if err := currentSuite.SetDuration(duration); err != nil {
				return nil, err
			}
		}

		if p.suiteParser.MarksCompletion(line) {
			if p.stream {
				fmt.Fprintln(os.Stdout, line)
			}

			p.builder.AddSuite(currentSuite)

			currentSuite = &api.TestSuite{}
			currentTest = nil
			isTestOutput = false
		}

		// we want to associate any line not directly related to a test suite with a test case to ensure we capture all output
		if isTestOutput {
			currentTestOutput = append(currentTestOutput, line)
		}
	}

	return p.builder.Build(), nil
}
