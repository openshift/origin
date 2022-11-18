package ginkgo

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2/types"
)

type testCase struct {
	// name is the name of this test after label annotation has been performed
	name string
	// nameFromBinary is the name of this test case as reported by the binary that owns it,
	// that is the name w/o any annotating performed
	nameFromBinary string
	spec           types.TestSpec
	locations      []types.CodeLocation
	apigroups      []string

	// identifies which tests can be run in parallel (ginkgo runs suites linearly)
	testExclusion string
	// specific timeout for the current test. When set, it overrides the current
	// suite timeout
	testTimeout time.Duration

	// identifies which binary contains this test
	binary string

	start           time.Time
	end             time.Time
	duration        time.Duration
	testOutputBytes []byte

	flake    bool
	failed   bool
	skipped  bool
	success  bool
	timedOut bool

	previous *testCase
}

var re = regexp.MustCompile(`.*\[Timeout:(.[^\]]*)\]`)

func newTestCaseFromGinkgoSpec(spec types.TestSpec) (*testCase, error) {
	name := spec.Text()
	tc := &testCase{
		name:      name,
		locations: spec.CodeLocations(),
		spec:      spec,
	}

	matches := regexp.MustCompile(`\[apigroup:([^]]*)\]`).FindAllStringSubmatch(name, -1)
	for _, match := range matches {
		if len(match) < 2 {
			return nil, fmt.Errorf("regexp match %v is invalid: len(match) < 2", match)
		}
		apigroup := match[1]
		tc.apigroups = append(tc.apigroups, apigroup)
	}

	if match := re.FindStringSubmatch(name); match != nil {
		testTimeOut, err := time.ParseDuration(match[1])
		if err != nil {
			return nil, err
		}
		tc.testTimeout = testTimeOut
	}

	return tc, nil
}

func (t *testCase) Retry() *testCase {
	copied := &testCase{
		name:          t.name,
		spec:          t.spec,
		locations:     t.locations,
		testExclusion: t.testExclusion,

		previous: t,
	}
	return copied
}

type TestSuite struct {
	Name        string
	Description string

	Matches func(name string) bool

	// The number of times to execute each test in this suite.
	Count int
	// The maximum parallelism of this suite.
	Parallelism int
	// The number of flakes that may occur before this test is marked as a failure.
	MaximumAllowedFlakes int

	// SyntheticEventTests is a set of suite level synthetics applied
	SyntheticEventTests JUnitsForEvents

	TestTimeout time.Duration
}

func (s *TestSuite) Filter(tests []*testCase) []*testCase {
	matches := make([]*testCase, 0, len(tests))
	for _, test := range tests {
		if !s.Matches(test.name) {
			continue
		}
		matches = append(matches, test)
	}
	return matches
}

func matchTestsFromFile(suite *TestSuite, contents []byte) error {
	tests := make(map[string]int)
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "\"") {
			var err error
			line, err = strconv.Unquote(line)
			if err != nil {
				return err
			}
			tests[line]++
		}
	}
	match := suite.Matches
	suite.Matches = func(name string) bool {
		// If there is an existing Matches function for the suite,
		// require the test to pass the existing match and also
		// be in the file contents.
		if match != nil && !match(name) {
			return false
		}
		_, ok := tests[name]
		return ok
	}
	return nil
}

func filterWithRegex(suite *TestSuite, regex string) error {
	re, err := regexp.Compile(regex)
	if err != nil {
		return err
	}
	origMatches := suite.Matches
	suite.Matches = func(name string) bool {
		return origMatches(name) && re.MatchString(name)
	}
	return nil
}

func testNames(tests []*testCase) []string {
	var names []string
	for _, t := range tests {
		names = append(names, t.name)
	}
	return names
}

// SuitesString returns a string with the provided suites formatted. Prefix is
// printed at the beginning of the output.
func SuitesString(suites []*TestSuite, prefix string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, prefix)
	for _, suite := range suites {
		fmt.Fprintf(buf, "%s\n  %s\n\n", suite.Name, suite.Description)
	}
	return buf.String()
}
