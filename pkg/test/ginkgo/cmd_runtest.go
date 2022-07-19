package ginkgo

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/ginkgo/types"

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

	tests, err := testsForSuite(config.GinkgoConfig)
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
		return fmt.Errorf("no test exists with that name")
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

	config.GinkgoConfig.FocusString = fmt.Sprintf("^%s$", regexp.QuoteMeta(" [Top Level] "+test.name))
	config.DefaultReporterConfig.NoColor = true
	w := ginkgo.GinkgoWriterType()
	w.SetStream(true)
	reporter := NewMinimalReporter(test.name, test.location)
	ginkgo.GlobalSuite().Run(reporter, "", []reporters.Reporter{reporter}, w, config.GinkgoConfig)
	summary, setup := reporter.Summary()
	if summary == nil && setup != nil {
		summary = &types.SpecSummary{
			Failure: setup.Failure,
			State:   setup.State,
		}
	}

	if opt.EnableMonitor {
		if err := opt.MonitorEventsOptions.End(ctx, restConfig, ""); err != nil {
			return err
		}
		if err := opt.MonitorEventsOptions.WriteRunDataToArtifactsDir(""); err != nil {
			fmt.Fprintf(opt.ErrOut, "error: Failed to write run-data: %v\n", err)
		}
	}

	// TODO: print stack line?
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
