package stack

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
func NewParser(builder builder.TestSuitesBuilder, testParser TestDataParser, suiteParser TestSuiteDataParser, stream bool) parser.TestOutputParser {
	return &testOutputParser{
		builder:     builder,
		testParser:  testParser,
		suiteParser: suiteParser,
		stream:      stream,
	}
}

// testOutputParser uses a stack to parse test output. Critical assumptions that this parser makes are:
//   1 - packages may be nested but tests may not
//   2 - no package declarations will occur within the boundaries of a test
//   3 - all tests and packages are fully bounded by a start and result line
//   4 - if a package or test declaration occurs after the start of a package but before its result,
//       the sub-package's or member test's result line will occur before that of the parent package
//       i.e. any test or package overlap will necessarily mean that one package's lines are a superset
//       of any lines of tests or other packages overlapping with it
//   5 - any text in the input file that doesn't match the parser regex is necessarily the output of the
//       current test being built
type testOutputParser struct {
	builder builder.TestSuitesBuilder

	testParser  TestDataParser
	suiteParser TestSuiteDataParser

	stream bool
}

// Parse parses output syntax of a specific class, the assumptions of which are outlined in the struct definition.
// The specific boundary markers and metadata encodings are free to vary as long as regex can be build to extract them
// from test output.
func (p *testOutputParser) Parse(input *bufio.Scanner) (*api.TestSuites, error) {
	inProgress := NewTestSuiteStack()

	var currentTest *api.TestCase
	var currentResult api.TestResult
	var currentOutput []string
	var currentMessage string

	for input.Scan() {
		line := input.Text()
		isTestOutput := true

		if p.testParser.MarksBeginning(line) {
			currentTest = &api.TestCase{}
			currentResult = api.TestResultFail
			currentOutput = []string{}
			currentMessage = ""
		}

		if name, contained := p.testParser.ExtractName(line); contained {
			currentTest.Name = name
		}

		if result, contained := p.testParser.ExtractResult(line); contained {
			currentResult = result
		}

		if duration, contained := p.testParser.ExtractDuration(line); contained {
			if err := currentTest.SetDuration(duration); err != nil {
				return nil, err
			}
		}

		if message, contained := p.testParser.ExtractMessage(line); contained {
			currentMessage = message
		}

		if p.testParser.MarksCompletion(line) {
			currentOutput = append(currentOutput, line)
			// if we have finished the current test case, we finalize our current test, add it to the package
			// at the head of our in progress package stack, and clear our current test record.
			switch currentResult {
			case api.TestResultSkip:
				currentTest.MarkSkipped(currentMessage)
			case api.TestResultFail:
				output := strings.Join(currentOutput, "\n")
				currentTest.MarkFailed(currentMessage, output)
			}

			if inProgress.Peek() == nil {
				return nil, fmt.Errorf("found test case %q outside of a test suite", currentTest.Name)
			}

			inProgress.Peek().AddTestCase(currentTest)
			currentTest = &api.TestCase{}
		}

		if p.suiteParser.MarksBeginning(line) {
			// if we encounter the beginning of a suite, we create a new suite to be considered and
			// add it to the head of our in progress package stack
			inProgress.Push(&api.TestSuite{})
			isTestOutput = false
		}

		if name, contained := p.suiteParser.ExtractName(line); contained {
			inProgress.Peek().Name = name
			isTestOutput = false
		}

		if properties, contained := p.suiteParser.ExtractProperties(line); contained {
			for propertyName := range properties {
				inProgress.Peek().AddProperty(propertyName, properties[propertyName])
			}
			isTestOutput = false
		}

		if p.suiteParser.MarksCompletion(line) {
			if p.stream {
				fmt.Fprintln(os.Stdout, line)
			}

			// if we encounter the end of a suite, we remove the suite at the head of the in progress stack
			p.builder.AddSuite(inProgress.Pop())
			isTestOutput = false
		}

		// we want to associate every line other than those directly involved with test suites as output of a test case
		if isTestOutput {
			currentOutput = append(currentOutput, line)
		}
	}
	return p.builder.Build(), nil
}
