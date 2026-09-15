package gotest

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/builder"
	"github.com/openshift/origin/tools/junitreport/pkg/parser"
)

// NewParser returns a new parser that's capable of parsing Go unit test output
func NewParser(builder builder.TestSuitesBuilder, stream bool) parser.TestOutputParser {
	return &testOutputParser{
		builder: builder,
		stream:  stream,
	}
}

type testOutputParser struct {
	builder builder.TestSuitesBuilder
	stream  bool
}

const (
	stateBegin = iota
	stateOutput
	stateResults
	stateComplete
)

func log(format string, args ...interface{}) {
	//fmt.Printf(format, args...)
}

func joinLinesPreservingTrailingNewline(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// Parse parses `go test -v` output into test suites. Test output from `go test -v` is not bookmarked for packages, so
// the parsing strategy is to advance line-by-line, building up a slice of test cases until a package declaration is found,
// at which point all tests cases are added to that package and the process can start again.
func (p *testOutputParser) Parse(input *bufio.Scanner) (*api.TestSuites, error) {
	suites := &api.TestSuites{}

	var testNameStack []string
	var tests map[string]*api.TestCase
	var output map[string][]string
	var messages map[string][]string
	var currentSuite *api.TestSuite
	var state int
	var count int
	var orderedTests []string

	for input.Scan() {
		line := input.Text()
		count++

		log("Line %03d: %d: %s\n", count, state, line)

		switch state {

		case stateBegin:
			// this is the first state
			name, ok := ExtractRun(line)
			if !ok {
				// A test that defines a test.M handler can write output prior to test execution. We will drop this because
				// we have no place to put it, although the first test case *could* use it in the future.
				log("  ignored output outside of suite\n")
				continue
			}
			log("  found run command %s\n", name)

			currentSuite = &api.TestSuite{}
			tests = make(map[string]*api.TestCase)
			output = make(map[string][]string)
			messages = make(map[string][]string)

			orderedTests = []string{name}
			testNameStack = []string{name}
			tests[testNameStack[0]] = &api.TestCase{
				Name: name,
			}

			state = stateOutput

		case stateOutput:
			// open a new test for gathering output
			if name, ok := ExtractRun(line); ok {
				log("  found run command %s\n", name)
				test, ok := tests[name]
				if !ok {
					test = &api.TestCase{
						Name: name,
					}
					tests[name] = test
				}
				orderedTests = append(orderedTests, name)
				testNameStack = []string{name}
				continue
			}

			// transition to result mode ONLY if it matches a result at the top level
			if result, name, depth, duration, ok := ExtractResult(line); ok && tests[name] != nil && depth == 0 {
				test := tests[name]
				log("  found result %s %s %s\n", result, name, duration)
				switch result {
				case api.TestResultPass:
				case api.TestResultFail:
					test.FailureOutput = &api.FailureOutput{}
				case api.TestResultSkip:
					test.SkipMessage = &api.SkipMessage{}
				}
				if err := test.SetDuration(duration); err != nil {
					return nil, fmt.Errorf("unexpected duration on line %d: %s", count, duration)
				}
				testNameStack = []string{name}
				state = stateResults
				continue
			}

			// in output mode, turn output lines into output on the particular test
			if _, _, ok := ExtractOutput(line); ok {
				log("  found output\n")
				output[testNameStack[0]] = append(output[testNameStack[0]], line)
				continue
			}
			log("  fallthrough\n")

		case stateResults:
			output, depth, ok := ExtractOutput(line)
			if !ok {
				return nil, fmt.Errorf("unexpected output on line %d, can't grab results", count)
			}

			// we're back to the root, we expect either a new RUN, a test suite end, or this is just an
			// output line that was chopped up
			if depth == 0 {
				if name, ok := ExtractRun(line); ok {
					log("  found run %s\n", name)
					// starting a new set of runs
					orderedTests = append(orderedTests, name)
					testNameStack = []string{name}
					tests[testNameStack[0]] = &api.TestCase{
						Name: name,
					}
					state = stateOutput
					continue
				}
				switch {
				case line == "PASS", line == "FAIL":
					log("  found end of suite\n")
					// at the end of the suite
					state = stateComplete
				default:
					// a broken output line that was not indented
					log("  found message\n")
					name := testNameStack[len(testNameStack)-1]
					test := tests[name]
					switch {
					case test.FailureOutput != nil, test.SkipMessage != nil:
						messages[name] = append(messages[name], output)
					}
				}
				continue
			}

			// if this is a result AND we have already declared this as a test, parse it
			if result, name, _, duration, ok := ExtractResult(output); ok && tests[name] != nil {
				log("  found result %s %s (%d)\n", result, name, depth)
				test := tests[name]
				switch result {
				case api.TestResultPass:
				case api.TestResultFail:
					test.FailureOutput = &api.FailureOutput{}
				case api.TestResultSkip:
					test.SkipMessage = &api.SkipMessage{}
				}
				if err := test.SetDuration(duration); err != nil {
					return nil, fmt.Errorf("unexpected duration on line %d: %s", count, duration)
				}
				switch {
				case depth >= len(testNameStack):
					// we found a new, more deeply nested test
					testNameStack = append(testNameStack, name)
				default:
					if depth < len(testNameStack)-1 {
						// the current result is less indented than our current test, so remove the deepest
						// items from the stack
						testNameStack = testNameStack[:depth]
					}
					testNameStack[len(testNameStack)-1] = name
				}
				continue
			}

			// treat as regular output at the appropriate depth
			log("  found message line %d %v\n", depth, testNameStack)
			originalDepth := depth
			// BUG: in go test, double nested output is double indented for some reason
			if depth >= len(testNameStack) {
				depth = len(testNameStack) - 1
			}
			// Output that appears after a nested result line at the same indentation belongs to the
			// parent test, because the child at that depth has already been closed by its result line.
			if originalDepth == len(testNameStack)-1 && len(testNameStack) > 1 {
				name := testNameStack[depth]
				if test, ok := tests[name]; ok && (test.FailureOutput != nil || test.SkipMessage != nil) {
					depth--
				}
			}
			name := testNameStack[depth]
			log("    name %s\n", name)
			if test, ok := tests[name]; ok {
				switch {
				case test.FailureOutput != nil, test.SkipMessage != nil:
					messages[name] = append(messages[name], output)
				}
			}

		case stateComplete:
			// suite exit line
			if name, duration, coverage, ok := ExtractPackage(line); ok {
				currentSuite.Name = name
				if props, ok := ExtractProperties(coverage); ok {
					for k, v := range props {
						currentSuite.AddProperty(k, v)
					}
				}
				for _, name := range orderedTests {
					test := tests[name]
					messageLines := messages[name]
					var extraOutput []string
					for i, s := range messageLines {
						if s == "=== OUTPUT" {
							log("test %s has OUTPUT section, %d %d\n", name, i, len(messageLines))
							if i < len(messageLines) {
								log("  test %s add lines: %d\n", name, len(messageLines[i+1:]))
								extraOutput = messageLines[i+1:]
							}
							messageLines = messageLines[:i]
							break
						}
					}

					switch {
					case test.FailureOutput != nil:
						test.FailureOutput.Output = joinLinesPreservingTrailingNewline(messageLines)

						lines := append(output[name], extraOutput...)
						test.SystemOut = joinLinesPreservingTrailingNewline(lines)

					case test.SkipMessage != nil:
						test.SkipMessage.Message = joinLinesPreservingTrailingNewline(messageLines)

					default:
						lines := append(output[name], extraOutput...)
						test.SystemOut = joinLinesPreservingTrailingNewline(lines)
					}

					currentSuite.AddTestCase(test)
				}
				if err := currentSuite.SetDuration(duration); err != nil {
					return nil, fmt.Errorf("unexpected duration on line %d: %s", count, duration)
				}
				suites.Suites = append(suites.Suites, currentSuite)

				state = stateBegin
				continue
			}

			// coverage only line
			if props, ok := ExtractProperties(line); ok {
				for k, v := range props {
					currentSuite.AddProperty(k, v)
				}
			}
		}
	}

	return suites, nil
}
