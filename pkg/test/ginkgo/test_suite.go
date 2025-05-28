package ginkgo

import (
	"regexp"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"

	"k8s.io/apimachinery/pkg/util/errors"

	k8sgenerated "k8s.io/kubernetes/openshift-hack/e2e/annotate/generated"

	"github.com/openshift/origin/pkg/test/extensions"
	origingenerated "github.com/openshift/origin/test/extended/util/annotate/generated"
)

func testsForSuite() ([]*testCase, error) {
	var tests []*testCase
	var errs []error

	// Don't build the tree multiple times, it results in multiple initing of tests
	if !ginkgo.GetSuite().InPhaseBuildTree() {
		ginkgo.GetSuite().BuildTree()
	}

	ginkgo.GetSuite().WalkTests(func(name string, spec types.TestSpec) {
		// we need to ensure the default path always annotates both
		// origin and k8s tests accordingly, since each of these
		// currently have their own annotations which are not
		// merged anywhere else but applied here
		if append, ok := origingenerated.Annotations[name]; ok {
			spec.AppendText(append)
		}
		if append, ok := k8sgenerated.Annotations[name]; ok {
			spec.AppendText(append)
		}
		tc, err := newTestCaseFromGinkgoSpec(spec)
		if err != nil {
			errs = append(errs, err)
		}
		tests = append(tests, tc)
	})
	if len(errs) > 0 {
		return nil, errors.NewAggregate(errs)
	}
	return tests, nil
}

var re = regexp.MustCompile(`.*\[Timeout:(.[^\]]*)\]`)

func externalBinaryTestsToOriginTestCases(specs extensions.ExtensionTestSpecs) []*testCase {
	var tests []*testCase
	for _, spec := range specs {
		tests = append(tests, &testCase{
			name:    spec.Name,
			rawName: spec.Name,
			binary:  spec.Binary,
		})
	}
	return tests
}

func newTestCaseFromGinkgoSpec(spec types.TestSpec) (*testCase, error) {
	name := spec.Text()
	tc := &testCase{
		name:      name,
		locations: spec.CodeLocations(),
		spec:      spec,
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

type TestSuite struct {
	Name        string
	Description string

	Matches TestMatchFunc

	// The number of times to execute each test in this suite.
	Count int
	// The maximum parallelism of this suite.
	Parallelism int
	// The number of flakes that may occur before this test is marked as a failure.
	MaximumAllowedFlakes int

	ClusterStabilityDuringTest ClusterStabilityDuringTest

	TestTimeout time.Duration
}

type TestMatchFunc func(name string) bool

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

func (s *TestSuite) AddRequiredMatchFunc(matchFn TestMatchFunc) {
	if matchFn == nil {
		return
	}
	if s.Matches == nil {
		s.Matches = matchFn
		return
	}

	originalMatchFn := s.Matches
	s.Matches = func(name string) bool {
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
