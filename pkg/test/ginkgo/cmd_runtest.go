package ginkgo

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
)

type ExitError struct {
	Code int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("exit with code %d", e.Code)
}

// TestOptions handles running a single test.
type TestOptions struct {
	DryRun      bool
	Out, ErrOut io.Writer
}

func (opt *TestOptions) Run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("only a single test name may be passed")
	}

	// Ignore the upstream suite behavior within test execution
	ginkgo.GetSuite().ClearBeforeAndAfterSuiteNodes()
	tests, err := testsForSuite()
	if err != nil {
		return err
	}
	var test *testCase
	for _, t := range tests {
		if t.name == args[0] {
			test = t
			break
		}
	}

	if test == nil {
		return fmt.Errorf("no test exists with name: %s", args[0])
	}

	if opt.DryRun {
		fmt.Fprintf(opt.Out, "Running test (dry-run)\n")
		return nil
	}

	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	suiteConfig.FocusStrings = []string{fmt.Sprintf("^%s$", regexp.QuoteMeta(" "+test.name))}
	//suiteConfig.FocusStrings = []string{fmt.Sprintf("^%s$", regexp.QuoteMeta(test.name))}
	// FIX ME
	//config.DefaultReporterConfig.NoColor = true
	//w := ginkgo.GinkgoWriterType()
	// FIX ME
	//w.SetStream(true)
	// reporter := NewMinimalReporter(test.name, test.locations[len(test.locations)-1])
	//ginkgo.GetSuite().BuildTree()

	// FIX ME
	//ginkgo.GetSuite().BuildTree()
	reporterConfig.JUnitReport = "/tmp/junit.xml"

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	suiteConfig.EmitSpecProgress = true
	// Randomize specs as well as suites
	suiteConfig.RandomizeAllSpecs = true
	// Turn on verbose by default to get spec names
	reporterConfig.Verbose = false
	reporterConfig.NoColor = false

	ginkgo.SetReporterConfig(reporterConfig)

	reporter := reporters.NoopReporter{}
	//reporter := reporters.NewDefaultReporter(reporterConfig, ginkgo.GetWriter())
	ginkgo.GetSuite().RunSpec(test.spec.InternalSpec, ginkgo.Labels{}, "/tmp/suitepath", ginkgo.GetFailer(), reporter, ginkgo.GetWriter(), ginkgo.GetOutputInterceptor(), ginkgo.NewInterruptHandler(suiteConfig.Timeout, nil), nil, suiteConfig)
	// func (suite *Suite) Run(description string, suiteLabels Labels, suitePath string, failer *Failer, reporter reporters.Reporter, writer WriterInterface, outputInterceptor OutputInterceptor, interruptHandler interrupt_handler.InterruptHandlerInterface, client parallel_support.Client, suiteConfig types.SuiteConfig) (bool, bool) {

	var summary types.SpecReport
	for _, report := range ginkgo.GetSuite().GetReport().SpecReports {
		if report.NumAttempts > 0 {
			//fmt.Printf("Test: %s\nReport:%#v\n", test.name, report.State.String())
			summary = report
		}
	}

	//reporters.ReportViaDeprecatedReporter(util.NewSimpleReporter(), ginkgo.GetSuite().GetReport())
	switch {
	//case summary == nil:
	//	return fmt.Errorf("test suite set up failed, see logs")
	case summary.State == types.SpecStatePassed:
		if s, ok := result.LastFlake(); ok {
			fmt.Fprintf(opt.ErrOut, "flake: %s\n", s)
			return ExitError{Code: 4}
		}
	case summary.State == types.SpecStateSkipped:
		if len(summary.Failure.Message) > 0 {
			fmt.Fprintf(opt.ErrOut, "skip [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message)
		}
		if len(summary.Failure.ForwardedPanic) > 0 {
			fmt.Fprintf(opt.ErrOut, "skip [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic)
		}
		return ExitError{Code: 3}
	case summary.State == types.SpecStateFailed, summary.State == types.SpecStatePanicked:
		if len(summary.Failure.ForwardedPanic) > 0 {
			if len(summary.Failure.Location.FullStackTrace) > 0 {
				fmt.Fprintf(opt.ErrOut, "\n%s\n", summary.Failure.Location.FullStackTrace)
			}
			fmt.Fprintf(opt.ErrOut, "fail [%s:%d]: Test Panicked: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic)
			return ExitError{Code: 1}
		}
		fmt.Fprintf(opt.ErrOut, "fail [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message)
		return ExitError{Code: 1}
	default:
		return fmt.Errorf("unrecognized test case outcome: %#v", summary)
	}
	// original
	/*
		ginkgo.GetSuite().Run(reporter, "", []reporters.Reporter{reporter}, *w, config.GinkgoConfig)

		summary, setup := reporter.Summary()
		if summary == nil && setup != nil {
			summary = &types.SpecSummary{
				Failure: setup.Failure,
				State:   setup.State,
			}
		}
	*/

	// TODO: print stack line?
	/*
		switch {
		case summary == nil:
			return fmt.Errorf("test suite set up failed, see logs")
		case summary.Passed():
			if s, ok := result.LastFlake(); ok {
				fmt.Fprintf(opt.ErrOut, "flake: %s\n", s)
				return ExitError{Code: 4}
			}
		case summary.Skipped():
			if len(summary.Failure.Message) > 0 {
				fmt.Fprintf(opt.ErrOut, "skip [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message)
			}
			if len(summary.Failure.ForwardedPanic) > 0 {
				fmt.Fprintf(opt.ErrOut, "skip [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic)
			}
			return ExitError{Code: 3}
		case summary.Failed(), summary.Panicked():
			if len(summary.Failure.ForwardedPanic) > 0 {
				if len(summary.Failure.Location.FullStackTrace) > 0 {
					fmt.Fprintf(opt.ErrOut, "\n%s\n", summary.Failure.Location.FullStackTrace)
				}
				fmt.Fprintf(opt.ErrOut, "fail [%s:%d]: Test Panicked: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic)
				return ExitError{Code: 1}
			}
			fmt.Fprintf(opt.ErrOut, "fail [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message)
			return ExitError{Code: 1}
		default:
			return fmt.Errorf("unrecognized test case outcome: %#v", summary)
		}
	*/
	return nil
}

func lastFilenameSegment(filename string) string {
	if parts := strings.Split(filename, "/vendor/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	if parts := strings.Split(filename, "/src/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return filename
}
