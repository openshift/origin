package ginkgo

import (
	"fmt"
	"io"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
)

func testsForSuite(cfg config.GinkgoConfigType) ([]*testCase, error) {
	iter := ginkgo.GlobalSuite().Iterator(cfg)
	var tests []*testCase
	for {
		spec, err := iter.Next()
		if err != nil {
			if err.Error() == "no more specs to run" {
				break
			}
			return nil, err
		}
		tests = append(tests, newTestCase(spec))
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
