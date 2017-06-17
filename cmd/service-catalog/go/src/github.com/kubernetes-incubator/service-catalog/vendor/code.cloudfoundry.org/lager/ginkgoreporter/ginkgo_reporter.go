package ginkgoreporter

import (
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
)

type SuiteStartSummary struct {
	RandomSeed                 int64  `json:"random_seed"`
	SuiteDescription           string `json:"description"`
	NumberOfSpecsThatWillBeRun int    `json:"num_specs"`
}

type SuiteEndSummary struct {
	SuiteDescription           string `json:"description"`
	Passed                     bool
	NumberOfSpecsThatWillBeRun int `json:"num_specs"`
	NumberOfPassedSpecs        int `json:"num_passed"`
	NumberOfFailedSpecs        int `json:"num_failed"`
}

type SpecSummary struct {
	Name     []string      `json:"name"`
	Location string        `json:"location"`
	State    string        `json:"state"`
	Passed   bool          `json:"passed"`
	RunTime  time.Duration `json:"run_time"`

	StackTrace string `json:"stack_trace,omitempty"`
}

type SetupSummary struct {
	Name    string        `json:"name"`
	State   string        `json:"state"`
	Passed  bool          `json:"passed"`
	RunTime time.Duration `json:"run_time,omitempty"`

	StackTrace string `json:"stack_trace,omitempty"`
}

func New(writer io.Writer) *GinkgoReporter {
	logger := lager.NewLogger("ginkgo")
	logger.RegisterSink(lager.NewWriterSink(writer, lager.DEBUG))
	return &GinkgoReporter{
		writer: writer,
		logger: logger,
	}
}

type GinkgoReporter struct {
	logger  lager.Logger
	writer  io.Writer
	session lager.Logger
}

func (g *GinkgoReporter) wrappedWithNewlines(f func()) {
	g.writer.Write([]byte("\n"))
	f()
	g.writer.Write([]byte("\n"))
}

func (g *GinkgoReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *types.SuiteSummary) {
	if config.ParallelTotal > 1 {
		var session = g.logger
		for i := 0; i < config.ParallelNode; i++ {
			session = g.logger.Session(fmt.Sprintf("node-%d", i+1))
		}
		g.logger = session
	}
}

func (g *GinkgoReporter) BeforeSuiteDidRun(setupSummary *types.SetupSummary) {
}

func (g *GinkgoReporter) SpecWillRun(specSummary *types.SpecSummary) {
	g.wrappedWithNewlines(func() {
		g.session = g.logger.Session("spec")
		g.session.Info("start", lager.Data{
			"summary": SpecSummary{
				Name:     specSummary.ComponentTexts,
				Location: specSummary.ComponentCodeLocations[len(specSummary.ComponentTexts)-1].String(),
			},
		})
	})
}

func (g *GinkgoReporter) SpecDidComplete(specSummary *types.SpecSummary) {
	g.wrappedWithNewlines(func() {
		if g.session == nil {
			return
		}
		summary := SpecSummary{
			Name:     specSummary.ComponentTexts,
			Location: specSummary.ComponentCodeLocations[len(specSummary.ComponentTexts)-1].String(),
			State:    stateAsString(specSummary.State),
			Passed:   passed(specSummary.State),
			RunTime:  specSummary.RunTime,
		}

		if passed(specSummary.State) {
			g.session.Info("end", lager.Data{
				"summary": summary,
			})
		} else {
			summary.StackTrace = specSummary.Failure.Location.FullStackTrace
			g.session.Error("end", errorForFailure(specSummary.Failure), lager.Data{
				"summary": summary,
			})
		}
		g.session = nil
	})
}

func (g *GinkgoReporter) AfterSuiteDidRun(setupSummary *types.SetupSummary) {
}

func (g *GinkgoReporter) SpecSuiteDidEnd(summary *types.SuiteSummary) {
}

func stateAsString(state types.SpecState) string {
	switch state {
	case types.SpecStatePending:
		return "PENDING"
	case types.SpecStateSkipped:
		return "SKIPPED"
	case types.SpecStatePassed:
		return "PASSED"
	case types.SpecStateFailed:
		return "FAILED"
	case types.SpecStatePanicked:
		return "PANICKED"
	case types.SpecStateTimedOut:
		return "TIMED OUT"
	default:
		return "INVALID"
	}
}

func passed(state types.SpecState) bool {
	return !(state == types.SpecStateFailed || state == types.SpecStatePanicked || state == types.SpecStateTimedOut)
}

func errorForFailure(failure types.SpecFailure) error {
	message := failure.Message
	if failure.ForwardedPanic != "" {
		message += fmt.Sprintf("%s", failure.ForwardedPanic)
	}

	return fmt.Errorf("%s\n%s", message, failure.Location.String())
}
