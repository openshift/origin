package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/tools/gotest2junit/pkg/api"
)

type Record struct {
	Package string
	Test    string

	Time    time.Time
	Action  string
	Output  string
	Elapsed float64
}

type testSuite struct {
	suite *api.TestSuite
	tests map[string]*api.TestCase
}

func main() {
	summarize := false
	verbose := false
	flag.BoolVar(&summarize, "summary", true, "display a summary as items are processed")
	flag.BoolVar(&verbose, "v", false, "display passing results")
	flag.Parse()

	if err := process(os.Stdin, summarize, verbose); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func process(r io.Reader, summarize, verbose bool) error {
	suites, err := stream(r, summarize, verbose)
	if err != nil {
		return err
	}
	obj := newTestSuites(suites)
	out, err := xml.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", string(out))
	return nil
}

func newTestSuites(suites map[string]*testSuite) *api.TestSuites {
	all := &api.TestSuites{}
	for _, suite := range suites {
		for _, test := range suite.suite.TestCases {
			suite.suite.NumTests++
			if test.SkipMessage != nil {
				suite.suite.NumSkipped++
				continue
			}
			if test.FailureOutput != nil {
				suite.suite.NumFailed++
				continue
			}
		}
		// suites with no tests are usually empty packages, ignore them
		if suite.suite.NumTests == 0 {
			continue
		}
		// always return the test cases in consistent order
		sort.Slice(suite.suite.TestCases, func(i, j int) bool {
			return suite.suite.TestCases[i].Name < suite.suite.TestCases[j].Name
		})
		all.Suites = append(all.Suites, suite.suite)
	}
	// always return the test suites in consistent order
	sort.Slice(all.Suites, func(i, j int) bool {
		return all.Suites[i].Name < all.Suites[j].Name
	})
	return all
}

func stream(r io.Reader, summarize, verbose bool) (map[string]*testSuite, error) {
	suites := make(map[string]*testSuite)
	defaultTest := &api.TestCase{
		Name: "build and execution",
	}
	defaultSuite := &testSuite{
		suite: &api.TestSuite{Name: "go test", TestCases: []*api.TestCase{defaultTest}},
	}
	suites[""] = defaultSuite

	rdr := bufio.NewReader(r)
	for {
		// some output from go test -json is not valid JSON - read the line to see whether it
		// starts with { - if not, just mirror it to stderr and continue.
		line, err := rdr.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return suites, err
			}
			break
		}
		if len(line) == 0 || line[0] != '{' {
			defaultTest.SystemOut += line
			if strings.HasPrefix(line, "FAIL") {
				defaultTest.FailureOutput = &api.FailureOutput{}
			}
			fmt.Fprint(os.Stderr, line)
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			if err == io.EOF {
				return suites, nil
			}
			fmt.Fprintf(os.Stderr, "error: Unable to parse remainder of output %v\n", err)
			return suites, nil
		}

		suite, ok := suites[r.Package]
		if !ok {
			suite = &testSuite{
				suite: &api.TestSuite{
					Name: r.Package,
				},
				tests: make(map[string]*api.TestCase),
			}
			suites[r.Package] = suite
		}

		// if this is package level output, we only care about pass/fail duration
		if len(r.Test) == 0 {
			switch r.Action {
			case "pass", "fail":
				suite.suite.Duration = r.Elapsed
			}
			continue
		}

		test, ok := suite.tests[r.Test]
		if !ok {
			test = &api.TestCase{
				Name: r.Test,
			}
			suite.suite.TestCases = append(suite.suite.TestCases, test)
			suite.tests[r.Test] = test
		}

		switch r.Action {
		case "run":
		case "pause":
		case "cont":
		case "bench":
		case "skip":
			if summarize {
				fmt.Fprintf(os.Stderr, "SKIP: %s %s\n", r.Package, r.Test)
			}
			test.SkipMessage = &api.SkipMessage{
				Message: r.Output,
			}
		case "pass":
			if summarize && verbose {
				fmt.Fprintf(os.Stderr, "PASS: %s %s %s\n", r.Package, r.Test, time.Duration(r.Elapsed*float64(time.Second)))
			}
			test.SystemOut = ""
			test.Duration = r.Elapsed
		case "fail":
			if summarize {
				fmt.Fprintf(os.Stderr, "FAIL: %s %s %s\n", r.Package, r.Test, time.Duration(r.Elapsed*float64(time.Second)))
			}
			test.Duration = r.Elapsed
			if len(r.Output) == 0 {
				r.Output = test.SystemOut
				if len(r.Output) > 50 {
					r.Output = r.Output[:50] + " ..."
				}
			}
			test.FailureOutput = &api.FailureOutput{
				Message: r.Output,
				Output:  r.Output,
			}
		case "output":
			test.SystemOut += r.Output
		default:
			// usually a bug in go test -json
			out := fmt.Sprintf("error: Unrecognized go test action %s: %#v\n", r.Action, r)
			defaultTest.SystemOut += line
			defaultTest.SystemOut += out
			defaultTest.FailureOutput = &api.FailureOutput{}
			fmt.Fprint(os.Stderr, out)
		}
	}

	// if we recorded any failure output
	if defaultTest.FailureOutput != nil {
		defaultTest.FailureOutput.Message = "Some packages failed during test execution"
		defaultTest.FailureOutput.Output = defaultTest.SystemOut
		defaultTest.SystemOut = ""
	}

	return suites, nil
}
