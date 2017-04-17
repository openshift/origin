package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"

	upstream "github.com/jstemmer/go-junit-report/parser"
)

// The following regex patterns define the syntax that we are expecting our output file to work with.
// Each package and test will begin with a line declaring the start and its name, and will end
// with a line declaring the end and the result (pass, fail, skip).
var (
	// packageStartPattern matches the beginning of a package output section
	// the sub-matches in this pattern are:
	//   1 - the name of the package
	packageStartPattern = regexp.MustCompile(`^>>>> BEGIN PACKAGE: (.+) <<<<$`)
	// testStartPattern matches the beginning of a test output section
	// the sub-matches in this pattern are:
	//   1 - the name of the test (which contains file. line number, and invokation details)
	testStartPattern = regexp.MustCompile(`^==== BEGIN TEST AT (.+) ====$`)
	// testResultPattern matches the end of a test output section
	// the sub-matches in this pattern are:
	//   1 - test result {pass,fail,skip}
	//   2 - time taken in milliseconds (real time)
	testResultPattern = regexp.MustCompile(`^==== END TEST: (PASS|FAIL|SKIP) AFTER ([0-9]+.[0-9]+) MILLISECONDS ====$`)
	// packageResultPattern matches the end of a package output section
	packageResultPattern = regexp.MustCompile(`^>>>> END PACKAGE <<<<$`)
)

// Parse parses test output from a reader (like stdin) and returns a Report
func Parse(input io.Reader) (*upstream.Report, error) {
	reader := bufio.NewReader(input)

	report := &upstream.Report{Packages: []*upstream.Package{}}

	inProgress := NewPackageStack()

	err := parseLines(reader, report, inProgress)
	if err != nil {
		return nil, fmt.Errorf("could not parse the input file: %s\n", err)
	}
	return report, nil
}

// parseLines uses a stack of in-progress packages to build the final report. Critical assumptions that
// parseLines makes are:
//   1 - packages may be nested but tests may not
//   2 - no package declarations will occur within the boundaries of a test
//   3 - all tests and packages are fully bounded by a start and result line
//   4 - if a package or test declaration occurs after the start of a package but before it's result,
//       the sub-package's or member test's result line will occur before that of the parent package
//       i.e. any test or package overlap will necessarily mean that one package's lines are a superset
//       of any lines of tests or other packages overlapping with it
//   5 - any text in the input file that doesn't match the above regex is necessarily the output of the
//       current test
func parseLines(input *bufio.Reader, report *upstream.Report, inProgress PackageStack) error {
	// currentTest is the current test being populated. It may not always be used in a call
	// to parseLines() as the parser may instead find the beginning or end to a package instead
	currentTest := &upstream.Test{}

	for {
		line, _, err := input.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		lineText := string(line)

		if packageStartPattern.Match(line) {
			fmt.Printf("Found start of package: \t%s\n", lineText)
			// if we encounter the beginning of a package, we create a new package to be considered and
			// add it to the head of our in progress package stack
			matches := packageStartPattern.FindStringSubmatch(lineText)
			inProgress.Push(&upstream.Package{
				Name: matches[1],
			})
		} else if testStartPattern.Match(line) {
			fmt.Printf("Found start of test: \t\t%s\n", lineText)
			// if we encounter the beginning of a test, we initialize our current test
			matches := testStartPattern.FindStringSubmatch(lineText)
			currentTest.Name = matches[1]
		} else if testResultPattern.Match(line) {
			fmt.Printf("Found end of test: \t\t%s\n", lineText)
			// if we encounter the end of a test, we finalize our current test, add it to the package
			// at the head of our in progress package stack, and clear our current test record
			matches := testResultPattern.FindStringSubmatch(lineText)

			// we can ignore the error comint out of the ParseFloat as the regex that creates this
			// ensures that we have a float
			floatTime, _ := strconv.ParseFloat(matches[2], 64)
			currentTest.Time = int(floatTime)

			currentTest.Result = parseResult(matches[1])

			inProgress.Peek().Tests = append(inProgress.Peek().Tests, currentTest)
			currentTest = &upstream.Test{}
		} else if packageResultPattern.Match(line) {
			fmt.Printf("Found end of package: \t\t%s\n", lineText)
			// if we encounter the end of a package, we finalize the package at the head of the in progress
			// package stack, remove it from the stack, and add it as a child to the new head of the stack
			// if it exists
			currentPackage := inProgress.Pop()

			if inProgress.Peek() != nil {
				inProgress.Peek().Children = append(inProgress.Peek().Children, currentPackage)
			} else {
				report.Packages = append(report.Packages, currentPackage)
			}
		} else {
			fmt.Printf("Found test output: \t\t%s\n", lineText)
			// if we did not encounter the beginning or end of a package or test, we are finding test output
			currentTest.Output = append(currentTest.Output, lineText)
		}
	}
	fmt.Println()
	return nil
}

// parseResult parses a result string into an upstream.Result. The regex for this is explicit,
// so we know we will have one of the three types. The default is to return a FAIL.
func parseResult(result string) upstream.Result {
	switch result {
	case "PASS":
		return upstream.PASS
	case "FAIL":
		return upstream.FAIL
	case "SKIP":
		return upstream.SKIP
	default:
		return upstream.FAIL
	}
}
