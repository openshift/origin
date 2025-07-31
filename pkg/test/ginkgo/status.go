package ginkgo

import (
	"fmt"
	"io"
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

	if isTestFailed(testRunResult.testState) {
		s.failures++
	}
}

func summarizeTests(tests []*testCase) *testSuiteResult {
	result := &testSuiteResult{}

	// Track results by test name to detect flakes
	testResults := make(map[string]struct {
		hasSuccess bool
		hasFailure bool
		testCases  []*testCase
	})

	// Collect all results by test name
	for _, t := range tests {
		testResult := testResults[t.name]
		testResult.testCases = append(testResult.testCases, t)

		switch {
		case t.success:
			testResult.hasSuccess = true
		case t.failed:
			testResult.hasFailure = true
		case t.skipped:
			result.skipped = append(result.skipped, t)
		}

		testResults[t.name] = testResult
	}

	// Categorize tests based on their overall result
	for _, testResult := range testResults {
		if testResult.hasFailure && testResult.hasSuccess {
			// Test has both successes and failures - it's a flake
			result.flaked = append(result.flaked, testResult.testCases)
		} else if testResult.hasFailure {
			// Test only has failures - it's a hard failure
			// Add all failed test cases to maintain the list for detailed reporting
			result.failed = append(result.failed, testResult.testCases)
		} else if testResult.hasSuccess {
			// Test only has successes - it's a pass
			// Add all successful test cases (there should only be 1)
			for _, tc := range testResult.testCases {
				if tc.success {
					result.passed = append(result.passed, tc)
				}
			}
		}
	}

	return result
}

func sortedTests(tests []*testCase) []*testCase {
	copied := make([]*testCase, len(tests))
	copy(copied, tests)
	sort.Slice(copied, func(i, j int) bool { return copied[i].name < copied[j].name })
	return copied
}
