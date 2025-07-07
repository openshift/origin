package suiteselection

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/client-go/discovery"

	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type DiscoveryClientGetter interface {
	GetDiscoveryClient() (discovery.AggregatedDiscoveryInterface, error)
}

type ConfigClientGetter interface {
	GetConfigClient() (clientconfigv1.Interface, error)
}

// TestSuiteSelectionFlags is used to run a suite of tests by invoking each test
// as a call to a child worker (the run-tests command).
type TestSuiteSelectionFlags struct {
	TestFile string

	// Regex allows a selection of a subset of tests
	Regex string

	genericclioptions.IOStreams
}

func NewTestSuiteSelectionFlags(streams genericclioptions.IOStreams) *TestSuiteSelectionFlags {
	return &TestSuiteSelectionFlags{
		IOStreams: streams,
	}
}

func (f *TestSuiteSelectionFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&f.TestFile, "file", "f", f.TestFile, "Create a suite from the newline-delimited test names in this file.")
	flags.StringVar(&f.Regex, "run", f.Regex, "Regular expression of tests to run.")
}

func (f *TestSuiteSelectionFlags) Validate() error {
	return nil
}

func (f *TestSuiteSelectionFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
}

// SelectSuite returns the defined suite plus the requested modifications to the suite in order to select the specified tests
func (f *TestSuiteSelectionFlags) SelectSuite(
	suites []*testginkgo.TestSuite,
	args []string) (*testginkgo.TestSuite, error) {
	var suite *testginkgo.TestSuite

	// If a test file was provided with no suite, use the "files" suite.
	if len(f.TestFile) > 0 && len(args) == 0 {
		suite = &testginkgo.TestSuite{
			Name: "files",
			Kind: testginkgo.KindInternal,
		}
	}
	if suite == nil && len(args) == 0 {
		fmt.Fprintf(f.ErrOut, SuitesString(suites, "Select a test suite to run against the server:\n\n"))
		return nil, fmt.Errorf("specify a test suite to run, for example: %s run %s", filepath.Base(os.Args[0]), suites[0].Name)
	}
	if suite == nil && len(args) > 0 {
		for _, s := range suites {
			if s.Name == args[0] {
				suite = s
				break
			}
		}
	}
	if suite == nil {
		fmt.Fprintf(f.ErrOut, SuitesString(suites, "Select a test suite to run against the server:\n\n"))
		return nil, fmt.Errorf("suite %q does not exist", args[0])
	}

	testFileMatchFn, err := f.testFileMatchFunc()
	if err != nil {
		return nil, err
	}
	suite.AddRequiredMatchFunc(testFileMatchFn)

	if len(f.Regex) > 0 {
		re, err := regexp.Compile(f.Regex)
		if err != nil {
			return nil, err
		}
		suite.AddRequiredMatchFunc(re.MatchString)
	}

	return suite, nil
}

// If a test file was provided, override the SuiteMatcher function
// to match the tests from both the suite and the file.
func (f *TestSuiteSelectionFlags) testFileMatchFunc() (testginkgo.TestMatchFunc, error) {
	if len(f.TestFile) == 0 {
		return nil, nil
	}

	var contents []byte
	var err error
	if f.TestFile == "-" {
		contents, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
	} else {
		contents, err = ioutil.ReadFile(f.TestFile)
	}
	if err != nil {
		return nil, err
	}

	tests := make(map[string]int)
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "\"") {
			var err error
			line, err = strconv.Unquote(line)
			if err != nil {
				return nil, err
			}
			tests[line]++
		}
	}

	return func(name string) bool {
		_, ok := tests[name]
		return ok
	}, nil
}

// TODO re-collapse
// SuitesString returns a string with the provided suites formatted. Prefix is
// printed at the beginning of the output.
func SuitesString(suites []*testginkgo.TestSuite, prefix string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, prefix)
	for _, suite := range suites {
		fmt.Fprintf(buf, "%s\n  %s\n\n", suite.Name, suite.Description)
	}
	return buf.String()
}
