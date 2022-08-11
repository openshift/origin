package ginkgo

import (
	"fmt"
	"io"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/config"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/openshift/origin/test/extended/util/annotate/generated"
)

/* v1 spec
type Spec struct {
	subject          leafnodes.SubjectNode
	focused          bool
	announceProgress bool

	containers []*containernode.ContainerNode

	state            types.SpecState
	runTime          time.Duration
	startTime        time.Time
	failure          types.SpecFailure
	previousFailures bool

	stateMutex *sync.Mutex
}
*/

/* v1 spec
type Spec struct {
	Nodes Nodes
	Skip  bool
}
*/

func testsForSuite() ([]*testCase, error) {
	if err := ginkgo.GetSuite().BuildTree(); err != nil {
		return nil, err
	}
	specs := ginkgo.GetSpecs()
	var tests []*testCase
	for _, spec := range specs {

		if append, ok := generated.Annotations[spec.Text()]; ok {
			spec.AppendText(append)

		} else {
			panic(fmt.Sprintf("unable to find test %s", spec.Text()))
		}

		tc, err := newTestCaseFromGinkgoSpec(ginkgo.Spec{spec})
		if err != nil {
			return nil, err
		}
		tests = append(tests, tc)
	}
	return tests, nil
}

type ginkgoSpec interface {
	Run(io.Writer)
	ConcatenatedString() string
	Skip()
	Skipped() bool
	Failed() bool
	Passed() bool
	Summary(suiteID string) *types.SpecSummary
}

type MinimalReporter struct {
	name     string
	location types.CodeLocation
	spec     *types.SpecSummary
	setup    *types.SetupSummary
}

func NewMinimalReporter(name string, location types.CodeLocation) *MinimalReporter {
	return &MinimalReporter{
		name:     name,
		location: location,
	}
}

func (r *MinimalReporter) Fail() {
}

func (r *MinimalReporter) Summary() (*types.SpecSummary, *types.SetupSummary) {
	return r.spec, r.setup
}

func (r *MinimalReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *types.SuiteSummary) {
}

func (r *MinimalReporter) BeforeSuiteDidRun(setup *types.SetupSummary) {
	r.setup = setup
}

func (r *MinimalReporter) SpecWillRun(spec *types.SpecSummary) {
}

func (r *MinimalReporter) SpecDidComplete(spec *types.SpecSummary) {
	if spec.ComponentCodeLocations[len(spec.ComponentCodeLocations)-1] != r.location {
		return
	}
	if specName(spec) != r.name {
		return
	}
	if r.spec != nil {
		panic(fmt.Sprintf("spec was set twice: %q and %q", specName(r.spec), specName(spec)))
	}
	r.spec = spec
}

func (r *MinimalReporter) AfterSuiteDidRun(setupSummary *types.SetupSummary) {
}

func (r *MinimalReporter) SpecSuiteDidEnd(summary *types.SuiteSummary) {
}

func specName(spec *types.SpecSummary) string {
	return strings.Join(spec.ComponentTexts[1:], " ")
}
