package ginkgo

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

type testSuiteProgress struct {
	lock     sync.Mutex
	failures int
	index    int
	total    int
}

func newTestSuiteProgress(total int) *testSuiteProgress {
	return &testSuiteProgress{
		total: total,
	}
}

func (s *testSuiteProgress) LogTestStart(out io.Writer, testName string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.index < s.total {
		s.index++
	} else {
		s.index++
		s.total++
	}

	fmt.Fprintf(out, "started: %d/%d/%d %q\n\n", s.failures, s.index, s.total, testName)
}

func (s *testSuiteProgress) TestEnded(testName string, testRunResult *testRunResultHandle) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if testRunResult == nil {
		fmt.Fprintln(os.Stderr, "testRunResult is nil")
		return
	}

	if isTestFailed(testRunResult.testState) {
		s.failures++
	}
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
