package ginkgo

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"
)

type testStatus struct {
	out     io.Writer
	timeout time.Duration

	lock     sync.Mutex
	failures int
	index    int
	total    int
}

func newTestStatus(out io.Writer, total int, timeout time.Duration) *testStatus {
	return &testStatus{
		out:     out,
		total:   total,
		timeout: timeout,
	}
}

func (s *testStatus) Failure() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.failures++
}

func (s *testStatus) Fprintf(format string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.index < s.total {
		s.index++
	}
	fmt.Fprintf(s.out, format, s.failures, s.index, s.total)
}

func (s *testStatus) Run(ctx context.Context, test *testCase) {
	defer func() {
		switch {
		case test.success:
			fmt.Fprint(s.out, string(test.out)+fmt.Sprintf("\npassed: (%s) %q\n\n", test.duration, test.name))
		case test.skipped:
			fmt.Fprint(s.out, string(test.out)+fmt.Sprintf("\nskipped: (%s) %q\n\n", test.duration, test.name))
		case test.failed:
			fmt.Fprint(s.out, string(test.out)+fmt.Sprintf("\nfailed: (%s) %q\n\n", test.duration, test.name))
			s.Failure()
		}
	}()

	test.start = time.Now()
	c := exec.Command(os.Args[0], "run-test", test.name)
	s.Fprintf(fmt.Sprintf("started: (%s) %q\n\n", "%d/%d/%d", test.name))
	out, err := runWithTimeout(ctx, c, s.timeout)
	test.end = time.Now()

	duration := test.end.Sub(test.start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}
	test.duration = duration
	test.out = out
	if err == nil {
		test.success = true
		return
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ProcessState.Sys().(syscall.WaitStatus).ExitStatus() {
		case 1:
			// failed
			test.failed = true
		case 2:
			// timeout (ABRT is an exit code 2)
			test.failed = true
		case 3:
			// skipped
			test.skipped = true
		default:
			test.failed = true
		}
		return
	}
	test.failed = true
}

func summarizeTests(tests []*testCase) (int, int, int, []*testCase) {
	var pass, fail, skip int
	var failingTests []*testCase
	for _, t := range tests {
		switch {
		case t.success:
			pass++
		case t.failed:
			fail++
			failingTests = append(failingTests, t)
		case t.skipped:
			skip++
		}
	}
	return pass, fail, skip, failingTests
}

func sortedTests(tests []*testCase) []*testCase {
	copied := make([]*testCase, len(tests))
	copy(copied, tests)
	sort.Slice(copied, func(i, j int) bool { return copied[i].name < copied[j].name })
	return copied
}
