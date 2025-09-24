package ginkgo

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	_ "embed"
)

//go:embed testNames.txt
var allTestNames string

func makeTestCases() []*testCase {
	ret := []*testCase{}
	for _, testName := range strings.Split(allTestNames, "\n") {
		ret = append(
			ret, &testCase{
				name: testName,
				spec: nil,
			},
		)
	}

	return ret
}

type testingSuiteRunner struct {
	lock     sync.Mutex
	testsRun []string
}

func (r *testingSuiteRunner) RunOneTest(ctx context.Context, test *testCase) {
	var delay int64
	delay = rand.Int63n(30)

	time.Sleep(time.Duration(delay) * time.Millisecond)

	r.lock.Lock()
	defer r.lock.Unlock()
	r.testsRun = append(r.testsRun, test.name)
}

func (r *testingSuiteRunner) RunMultipleTests(ctx context.Context, tests ...*testCase) {
	return
}

func (r *testingSuiteRunner) getTestsRun() []string {
	r.lock.Lock()
	defer r.lock.Unlock()

	ret := make([]string, len(r.testsRun))
	copy(ret, r.testsRun)
	return ret
}

func Test_execute(t *testing.T) {
	tests := makeTestCases()
	testSuiteRunner := &testingSuiteRunner{}
	parallelism := 30
	execute(context.TODO(), testSuiteRunner, tests, parallelism, false)

	testsCompleted := testSuiteRunner.getTestsRun()
	if len(tests) != len(testsCompleted) {
		t.Errorf("expected %v, got %v", len(tests), len(testsCompleted))
	}
}
