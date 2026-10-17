package ginkgo

import (
	"io"
	"strings"
	"testing"
)

func TestProcessRetryResults(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name         string
		unretried    int
		priorSuccess bool
		outcome      string
		wantFailures int
		wantTests    int
		wantFlaky    int
	}{
		{name: "permanent failure has one rollup", outcome: "failed", wantFailures: 1, wantTests: 1},
		{name: "unretried failures are preserved alongside a permanent retry failure", unretried: 3, outcome: "failed", wantFailures: 4, wantTests: 4},
		{name: "permanent retry failure preserves an earlier same-name success", priorSuccess: true, outcome: "failed", wantFailures: 1, wantTests: 2},
		{name: "permanent retry failure preserves earlier success and unrelated failures", unretried: 1, priorSuccess: true, outcome: "failed", wantFailures: 2, wantTests: 3},
		{name: "successful retry is flaky", outcome: "success", wantTests: 2, wantFlaky: 1},
		{name: "successful retry preserves unretried failure", unretried: 1, outcome: "success", wantFailures: 1, wantTests: 3, wantFlaky: 1},
		{name: "no eligible tests unchanged", unretried: 3, wantFailures: 3, wantTests: 3},
		{name: "skipped precondition removes original failure", outcome: "skipped"},
		{name: "skipped precondition preserves unretried failure", unretried: 1, outcome: "skipped", wantFailures: 1, wantTests: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			var tests, failing []*testCase
			for index := 0; index < scenario.unretried; index++ {
				original := &testCase{name: strings.Repeat("unretried", index+1), failed: true}
				tests = append(tests, original)
				failing = append(failing, original)
			}
			originalFailures := append([]*testCase(nil), failing...)
			attempts := map[string][]*testCase{}
			var priorSuccess *testCase
			if scenario.outcome != "" {
				original := &testCase{name: "FIPS", failed: true, testOutputBytes: []byte("original failure evidence")}
				if scenario.priorSuccess {
					priorSuccess = &testCase{name: original.name, success: true, testOutputBytes: []byte("earlier success evidence")}
					tests = append(tests, priorSuccess)
				}
				retry := &testCase{name: original.name, failed: scenario.outcome == "failed", success: scenario.outcome == "success", skipped: scenario.outcome == "skipped", testOutputBytes: []byte("retry evidence")}
				tests = append(tests, original)
				failing = append(failing, original)
				attempts[original.name] = []*testCase{original, retry}
			}
			options := &GinkgoRunSuiteOptions{RetryStrategy: &RetryOnceStrategy{}}
			options.Out = io.Discard
			results, failures, flaky := options.processRetryResults(tests, failing, attempts)
			if len(results) != scenario.wantTests || len(failures) != scenario.wantFailures || flaky != scenario.wantFlaky {
				t.Fatalf("got tests=%d failures=%d flaky=%d, want tests=%d failures=%d flaky=%d", len(results), len(failures), flaky, scenario.wantTests, scenario.wantFailures, scenario.wantFlaky)
			}
			for index, original := range originalFailures {
				if results[index] != original || failures[index] != original {
					t.Fatalf("unretried failure %q was replaced", original.name)
				}
			}
			if scenario.outcome == "failed" {
				fullJUnit := generateJUnitTestSuiteResults("retry-results", 0, results)
				if fullJUnit.NumTests != uint(scenario.wantTests) || len(fullJUnit.TestCases) != scenario.wantTests || fullJUnit.NumFailed != uint(len(failures)) {
					t.Errorf("full JUnit has tests=%d cases=%d failures=%d, want tests=%d failures=%d", fullJUnit.NumTests, len(fullJUnit.TestCases), fullJUnit.NumFailed, scenario.wantTests, len(failures))
				}
				if priorSuccess != nil && (results[scenario.unretried] != priorSuccess || !priorSuccess.success || priorSuccess.failed) {
					t.Error("earlier same-name success must retain its execution identity and status")
				}
				rollup := failures[len(failures)-1]
				if !rollup.failed || rollup.success || rollup.skipped || !isBlockingFailure(rollup) {
					t.Fatal("rollup no longer represents a blocking failure")
				}
				if rollup != results[len(results)-1] || rollup == attempts["FIPS"][0] {
					t.Fatal("results and failures must share exactly one replacement rollup")
				}
				junit := generateJUnitTestSuiteResults("retry-results", 0, []*testCase{rollup})
				if junit.NumFailed != 1 || len(junit.TestCases) != 1 || junit.TestCases[0].FailureOutput == nil {
					t.Fatal("rollup JUnit must retain one failed case")
				}
				for _, evidence := range []string{"failed all 2 attempts", "=== Attempt 1: FAILED ===", "original failure evidence", "=== Attempt 2: FAILED ===", "retry evidence"} {
					if !strings.Contains(junit.TestCases[0].SystemOut, evidence) || !strings.Contains(junit.TestCases[0].FailureOutput.Output, evidence) {
						t.Errorf("JUnit lost retry evidence %q", evidence)
					}
				}
			}
		})
	}
}
