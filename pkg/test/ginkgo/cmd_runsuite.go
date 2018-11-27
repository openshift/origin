package ginkgo

import (
	"context"
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

	"github.com/onsi/ginkgo/config"
)

// Options is used to run a suite of tests by invoking each test
// as a call to a child worker (the run-tests command).
type Options struct {
	Parallelism  int
	Timeout      time.Duration
	JUnitDir     string
	TestFile     string
	OutFile      string
	DetectFlakes int

	Provider string

	Suites []*TestSuite

	DryRun      bool
	Out, ErrOut io.Writer
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

	if opt.DryRun {
		// if true {
		// 	bySuite := make(map[string][]*testCase)
		// 	for _, test := range tests {
		// 		bySuite[test.testExclusion] = append(bySuite[test.testExclusion], test)
		// 	}
		// 	var names []string
		// 	for k := range bySuite {
		// 		names = append(names, k)
		// 	}
		// 	sort.Slice(names, func(i, j int) bool {
		// 		return len(bySuite[names[i]]) > len(bySuite[names[j]])
		// 	})
		// 	for _, name := range names {
		// 		if len(name) == 0 {
		// 			fmt.Fprintf(out, "<none>:\n")
		// 		} else {
		// 			fmt.Fprintf(out, "%s:\n", name)
		// 		}
		// 		for _, test := range sortedTests(bySuite[name]) {
		// 			fmt.Fprintf(out, "  %q\n", test.name)
		// 		}
		// 	}
		// 	return nil
		// }
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
		// TODO: temporarily increased because some normally fast build tests have become much slower
		//   reduce back to 10m at some point in the future
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

	status := newTestStatus(opt.Out, len(tests), timeout)

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

	if fail > 0 && fail <= opt.DetectFlakes {
		var retries []*testCase
		for _, test := range failing {
			retries = append(retries, test.Retry())
			if len(retries) > opt.DetectFlakes {
				break
			}
		}

		q := newParallelTestQueue(retries)
		status := newTestStatus(ioutil.Discard, len(retries), timeout)
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
		if err := writeJUnitReport("openshift-tests", tests, opt.JUnitDir, duration, opt.ErrOut); err != nil {
			fmt.Fprintf(opt.Out, "error: Unable to write JUnit results: %v", err)
		}
	}

	if fail > 0 {
		if len(failing) > 0 || !suite.AllowPassWithFlakes {
			return fmt.Errorf("%d fail, %d pass, %d skip (%s)", fail, pass, skip, duration)
		}
		fmt.Fprintf(opt.Out, "%d flakes detected, suite allows passing with only flakes\n\n", fail)
	}
	fmt.Fprintf(opt.Out, "%d pass, %d skip (%s)\n", pass, skip, duration)
	return nil
}
