package ginkgo

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/onsi/ginkgo/config"
)

// Options is used to run a suite of tests by invoking each test
// as a call to a child worker (the run-tests command).
type Options struct {
	Parallelism int
	Count       int
	Timeout     time.Duration
	JUnitDir    string
	TestFile    string
	OutFile     string

	// Regex allows a selection of a subset of tests
	Regex string
	// MatchFn if set is also used to filter the suite contents
	MatchFn func(name string) bool

	IncludeSuccessOutput bool

	Provider     string
	SuiteOptions string

	Suites []*TestSuite

	DryRun        bool
	PrintCommands bool
	Out, ErrOut   io.Writer
}

func (opt *Options) AsEnv() []string {
	var args []string
	args = append(args, fmt.Sprintf("TEST_PROVIDER=%s", opt.Provider))
	args = append(args, fmt.Sprintf("TEST_SUITE_OPTIONS=%s", opt.SuiteOptions))
	return args
}

func (opt *Options) Run(args []string) error {
	var suite *TestSuite

	if len(opt.TestFile) > 0 {
		var in []byte
		var err error
		if opt.TestFile == "-" {
			in, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
		} else {
			in, err = ioutil.ReadFile(opt.TestFile)
		}
		if err != nil {
			return err
		}
		suite, err = newSuiteFromFile("files", in)
		if err != nil {
			return fmt.Errorf("could not read test suite from input: %v", err)
		}
	}

	if suite == nil && len(args) == 0 {
		fmt.Fprintf(opt.ErrOut, SuitesString(opt.Suites, "Select a test suite to run against the server:\n\n"))
		return fmt.Errorf("specify a test suite to run, for example: %s run %s", filepath.Base(os.Args[0]), opt.Suites[0].Name)
	}
	if suite == nil && len(args) > 0 {
		for _, s := range opt.Suites {
			if s.Name == args[0] {
				suite = s
				break
			}
		}
	}
	if suite == nil {
		fmt.Fprintf(opt.ErrOut, SuitesString(opt.Suites, "Select a test suite to run against the server:\n\n"))
		return fmt.Errorf("suite %q does not exist", args[0])
	}

	if len(opt.Regex) > 0 {
		if err := filterWithRegex(suite, opt.Regex); err != nil {
			return err
		}
	}
	if opt.MatchFn != nil {
		original := suite.Matches
		suite.Matches = func(name string) bool {
			return original(name) && opt.MatchFn(name)
		}
	}

	tests, err := testsForSuite(config.GinkgoConfig)
	if err != nil {
		return err
	}

	// This ensures that tests in the identified paths do not run in parallel, because
	// the test suite reuses shared resources without considering whether another test
	// could be running at the same time. While these are technically [Serial], ginkgo
	// parallel mode provides this guarantee. Doing this for all suites would be too
	// slow.
	setTestExclusion(tests, func(suitePath string, t *testCase) bool {
		for _, name := range []string{
			"/k8s.io/kubernetes/test/e2e/apps/disruption.go",
		} {
			if strings.HasSuffix(suitePath, name) {
				return true
			}
		}
		return false
	})

	tests = suite.Filter(tests)
	if len(tests) == 0 {
		return fmt.Errorf("suite %q does not contain any tests", suite.Name)
	}

	count := opt.Count
	if count == 0 {
		count = suite.Count
	}
	if count > 1 {
		var newTests []*testCase
		for i := 0; i < count; i++ {
			newTests = append(newTests, tests...)
		}
		tests = newTests
	}

	if opt.PrintCommands {
		status := newTestStatus(opt.Out, true, len(tests), time.Minute, &monitor.Monitor{}, opt.AsEnv())
		newParallelTestQueue(tests).Execute(context.Background(), 1, status.OutputCommand)
		return nil
	}
	if opt.DryRun {
		for _, test := range sortedTests(tests) {
			fmt.Fprintf(opt.Out, "%q\n", test.name)
		}
		return nil
	}

	if len(opt.JUnitDir) > 0 {
		if _, err := os.Stat(opt.JUnitDir); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not access --junit-dir: %v", err)
			}
			if err := os.MkdirAll(opt.JUnitDir, 0755); err != nil {
				return fmt.Errorf("could not create --junit-dir: %v", err)
			}
		}
	}

	parallelism := opt.Parallelism
	if parallelism == 0 {
		parallelism = suite.Parallelism
	}
	if parallelism == 0 {
		parallelism = 10
	}
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = suite.TestTimeout
	}
	if timeout == 0 {
		timeout = 15 * time.Minute
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal)
	go func() {
		<-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted, terminating tests\n")
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	m, err := monitor.Start(ctx)
	if err != nil {
		return err
	}
	// if we run a single test, always include success output
	includeSuccess := opt.IncludeSuccessOutput
	if len(tests) == 1 {
		includeSuccess = true
	}
	status := newTestStatus(opt.Out, includeSuccess, len(tests), timeout, m, opt.AsEnv())

	smoke, normal := splitTests(tests, func(t *testCase) bool {
		return strings.Contains(t.name, "[Smoke]")
	})

	// run the tests
	start := time.Now()

	// run our smoke tests first
	q := newParallelTestQueue(smoke)
	q.Execute(ctx, parallelism, status.Run)

	// run other tests next
	q = newParallelTestQueue(normal)
	q.Execute(ctx, parallelism, status.Run)

	duration := time.Now().Sub(start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}

	pass, fail, skip, failing := summarizeTests(tests)

	// monitor the cluster while the tests are running and report any detected
	// anomalies
	var syntheticTestResults []*JUnitTestCase
	if events := m.Events(time.Time{}, time.Time{}); len(events) > 0 {
		// Serialize the interval data for easier external analysis
		intervalsFilename := filepath.Join(opt.JUnitDir, "intervals.json.gz")
		if file, err := os.Create(intervalsFilename); err != nil {
			fmt.Fprintf(opt.Out, "ERROR: failed to create %s: %v\n", intervalsFilename, err)
		} else {
			gz := gzip.NewWriter(file)
			defer func() {
				if err := gz.Close(); err != nil {
					fmt.Fprintf(opt.Out, "ERROR: failed to close %s: %v\n", intervalsFilename, err)
				}
			}()
			if err := json.NewEncoder(gz).Encode(events); err != nil {
				fmt.Fprintf(opt.Out, "ERROR: failed to write %s: %v\n", intervalsFilename, err)
			} else {
				fmt.Fprintf(opt.Out, "wrote monitor interval data to %s\n", intervalsFilename)
			}
		}
		buf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
		fmt.Fprintf(buf, "\nTimeline:\n\n")
		errorCount := 0
		for _, test := range tests {
			if !test.failed {
				continue
			}
			events = append(events,
				&monitor.EventInterval{
					From: test.start,
					To:   test.end,
					Condition: &monitor.Condition{
						Level:   monitor.Info,
						Locator: fmt.Sprintf("test=%q", test.name),
						Message: "running",
					},
				},
				&monitor.EventInterval{
					From: test.end,
					To:   test.end,
					Condition: &monitor.Condition{
						Level:   monitor.Info,
						Locator: fmt.Sprintf("test=%q", test.name),
						Message: "failed",
					},
				},
			)
		}
		sort.Sort(events)
		for _, event := range events {
			if event.Level == monitor.Error {
				errorCount++
				fmt.Fprintln(errBuf, event.String())
			}
			fmt.Fprintln(buf, event.String())
		}
		fmt.Fprintln(buf)

		if errorCount > 0 {
			syntheticTestResults = append(syntheticTestResults, &JUnitTestCase{
				Name:      "Monitor cluster while tests execute",
				SystemOut: buf.String(),
				Duration:  duration.Seconds(),
				FailureOutput: &FailureOutput{
					Output: fmt.Sprintf("%d error level events were detected during this test run:\n\n%s", errorCount, errBuf.String()),
				},
			})
		}

		opt.Out.Write(buf.Bytes())
	}

	// attempt to retry failures to do flake detection
	if fail > 0 && fail <= suite.MaximumAllowedFlakes {
		var retries []*testCase
		for _, test := range failing {
			retries = append(retries, test.Retry())
			if len(retries) > suite.MaximumAllowedFlakes {
				break
			}
		}

		q := newParallelTestQueue(retries)
		status := newTestStatus(ioutil.Discard, opt.IncludeSuccessOutput, len(retries), timeout, m, opt.AsEnv())
		q.Execute(ctx, parallelism, status.Run)
		var flaky []string
		var repeatFailures []*testCase
		for _, test := range retries {
			if test.success {
				flaky = append(flaky, test.name)
			} else {
				repeatFailures = append(repeatFailures, test)
			}
		}
		if len(flaky) > 0 {
			failing = repeatFailures
			sort.Strings(flaky)
			fmt.Fprintf(opt.Out, "Flaky tests:\n\n%s\n\n", strings.Join(flaky, "\n"))
		}
	}

	if len(failing) > 0 {
		names := testNames(failing)
		sort.Strings(names)
		fmt.Fprintf(opt.Out, "Failing tests:\n\n%s\n\n", strings.Join(names, "\n"))
	}

	if len(opt.JUnitDir) > 0 {
		if err := writeJUnitReport("junit_e2e", "openshift-tests", tests, opt.JUnitDir, duration, opt.ErrOut, syntheticTestResults...); err != nil {
			fmt.Fprintf(opt.Out, "error: Unable to write e2e JUnit results: %v", err)
		}
	}

	if fail > 0 {
		if len(failing) > 0 || suite.MaximumAllowedFlakes == 0 {
			return fmt.Errorf("%d fail, %d pass, %d skip (%s)", fail, pass, skip, duration)
		}
		fmt.Fprintf(opt.Out, "%d flakes detected, suite allows passing with only flakes\n\n", fail)
	}

	fmt.Fprintf(opt.Out, "%d pass, %d skip (%s)\n", pass, skip, duration)
	return ctx.Err()
}
