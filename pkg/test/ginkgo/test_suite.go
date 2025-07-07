package ginkgo

import (
	"regexp"
	"time"

	"github.com/onsi/ginkgo/v2/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/test/extensions"
)

var re = regexp.MustCompile(`.*\[Timeout:(.[^\]]*)\]`)

func extensionTestSpecsToOriginTestCases(specs extensions.ExtensionTestSpecs) ([]*testCase, error) {
	var tests []*testCase
	for _, spec := range specs {
		tc := &testCase{
			name:    spec.Name,
			rawName: spec.Name,
			binary:  spec.Binary,
		}

		// Override timeout from suite with `[Timeout:X]` duration
		if match := re.FindStringSubmatch(tc.name); match != nil {
			testTimeOut, err := time.ParseDuration(match[1])
			if err != nil {
				return nil, errors.WithMessage(err, "failed to parse test timeout")
			}
			logrus.WithField("test", tc.name).Debugf("Overriding test timeout to %s", testTimeOut)
			tc.testTimeout = testTimeOut
		}

		tests = append(tests, tc)
	}

	return tests, nil
}

type testCase struct {
	// name is the fully labeled test name as reported by openshift-tests
	// this is being used for placing tests in buckets, as well as filtering
	// them out based suite being currently executed
	name string
	// rawName is the name as reported by external binary
	rawName string

	// binaryName is the name of the binary to execute for internal tests
	binaryName string
	// binary is the reference when using an external binary
	binary *extensions.TestBinary

	spec      types.TestSpec
	locations []types.CodeLocation

	// identifies which tests can be run in parallel (ginkgo runs suites linearly)
	testExclusion string
	// specific timeout for the current test. When set, it overrides the current
	// suite timeout
	testTimeout time.Duration

	start           time.Time
	end             time.Time
	duration        time.Duration
	testOutputBytes []byte

	flake               bool
	failed              bool
	skipped             bool
	success             bool
	timedOut            bool
	extensionTestResult *extensions.ExtensionTestResult

	previous *testCase
}

func (t *testCase) Retry() *testCase {
	copied := &testCase{
		name:          t.name,
		spec:          t.spec,
		binary:        t.binary,
		rawName:       t.rawName,
		binaryName:    t.binaryName,
		locations:     t.locations,
		testExclusion: t.testExclusion,
		testTimeout:   t.testTimeout,

		previous: t,
	}
	return copied
}

type ClusterStabilityDuringTest string

var (
	// Stable means that at no point during testing do we expect a component to take downtime and upgrades are not happening.
	Stable ClusterStabilityDuringTest = "Stable"
	// TODO only bring this back if we have some reason to collect Upgrade specific information.  I can't think of reason.
	// TODO please contact @deads2k for vetting if you think you found something
	//Upgrade    ClusterStabilityDuringTest = "Upgrade"
	// Disruptive means that the suite is expected to induce outages to the cluster.
	Disruptive ClusterStabilityDuringTest = "Disruptive"
)

type Kind int

const (
	KindInternal Kind = iota
	KindExternal
)

type TestSuite struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        Kind   `json:"kind"`

	SuiteMatcher TestMatchFunc `json:"-"`

	// The number of times to execute each test in this suite.
	Count int `json:"count,omitempty"`
	// The maximum parallelism of this suite.
	Parallelism int `json:"parallelism,omitempty"`
	// The number of flakes that may occur before this test is marked as a failure.
	MaximumAllowedFlakes int `json:"maximumAllowedFlakes,omitempty"`

	ClusterStabilityDuringTest ClusterStabilityDuringTest `json:"clusterStabilityDuringTest,omitempty"`

	TestTimeout time.Duration `json:"testTimeout,omitempty"`

	// OTE
	Qualifiers []string              `json:"qualifiers,omitempty"`
	Extension  *extensions.Extension `json:"-"`
}

type TestMatchFunc func(name string) bool

func (s *TestSuite) Filter(tests []*testCase) []*testCase {
	if s.SuiteMatcher == nil {
		return tests
	}

	matches := make([]*testCase, 0, len(tests))
	for _, test := range tests {
		if !s.SuiteMatcher(test.name) {
			continue
		}
		matches = append(matches, test)
	}
	return matches
}

func (s *TestSuite) AddRequiredMatchFunc(matchFn TestMatchFunc) {
	if matchFn == nil {
		return
	}
	if s.SuiteMatcher == nil {
		s.SuiteMatcher = matchFn
		return
	}

	originalMatchFn := s.SuiteMatcher
	s.SuiteMatcher = func(name string) bool {
		return originalMatchFn(name) && matchFn(name)
	}
}

func testNames(tests []*testCase) []string {
	var names []string
	for _, t := range tests {
		names = append(names, t.name)
	}
	return names
}
