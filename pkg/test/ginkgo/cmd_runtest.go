package ginkgo

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/openshift/origin/pkg/monitor"
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
	// EnableMonitor is an easy way to enable monitor gathering for a single e2e test.
	// TODO if this is useful enough for general users, we can extend this into an arg, this just ensures the plumbing.
	EnableMonitor        bool
	MonitorEventsOptions *MonitorEventsOptions

	DryRun bool
	Out    io.Writer
	ErrOut io.Writer
}

func NewTestOptions(out io.Writer, errOut io.Writer) *TestOptions {
	return &TestOptions{
		MonitorEventsOptions: NewMonitorEventsOptions(out, errOut),
		Out:                  out,
		ErrOut:               errOut,
	}
}

func (opt *TestOptions) Run(args []string) error {
	ctx := context.TODO()

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

	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}
	if opt.EnableMonitor {
		_, err = opt.MonitorEventsOptions.Start(ctx, restConfig)
		if err != nil {
			return err
		}
	}

	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	suiteConfig.FocusStrings = []string{fmt.Sprintf("^%s$", regexp.QuoteMeta(" "+test.name))}

	// These settings are matched to upstream's ginkgo configuration. See:
	// https://github.com/kubernetes/kubernetes/blob/ddeb3ab90b581a7531dcaee3c55c7b9199981fd6/test/e2e/framework/test_context.go#L324-L334
	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	suiteConfig.EmitSpecProgress = true
	// Randomize specs as well as suites
	suiteConfig.RandomizeAllSpecs = true
	reporterConfig.NoColor = true

	ginkgo.SetReporterConfig(reporterConfig)

	reporter := reporters.NoopReporter{}
	ginkgo.GetSuite().RunSpec(test.spec.InternalSpec, ginkgo.Labels{}, "", ginkgo.GetFailer(), reporter, ginkgo.GetWriter(), ginkgo.GetOutputInterceptor(), ginkgo.NewInterruptHandler(suiteConfig.Timeout, nil), nil, suiteConfig)

	if opt.EnableMonitor {
		if err := opt.MonitorEventsOptions.End(ctx, restConfig, ""); err != nil {
			return err
		}
		if err := opt.MonitorEventsOptions.WriteRunDataToArtifactsDir(""); err != nil {
			fmt.Fprintf(opt.ErrOut, "error: Failed to write run-data: %v\n", err)
		}
	}

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
