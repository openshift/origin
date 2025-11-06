package ginkgo

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "embed"

	extensiontests "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
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
	execute(context.TODO(), testSuiteRunner, tests, parallelism)

	testsCompleted := testSuiteRunner.getTestsRun()
	if len(tests) != len(testsCompleted) {
		t.Errorf("expected %v, got %v", len(tests), len(testsCompleted))
	}
}

// trackingTestRunner extends testingSuiteRunner with execution tracking
type trackingTestRunner struct {
	testingSuiteRunner
	runningTests   map[string]bool
	runningMutex   sync.Mutex
	executionOrder []string
	startTimes     map[string]time.Time
	endTimes       map[string]time.Time
}

func newTrackingTestRunner() *trackingTestRunner {
	return &trackingTestRunner{
		runningTests: make(map[string]bool),
		startTimes:   make(map[string]time.Time),
		endTimes:     make(map[string]time.Time),
	}
}

func (r *trackingTestRunner) RunOneTest(ctx context.Context, test *testCase) {
	r.runningMutex.Lock()
	r.runningTests[test.name] = true
	r.startTimes[test.name] = time.Now()
	r.executionOrder = append(r.executionOrder, test.name+":start")
	r.runningMutex.Unlock()

	// Simulate test execution
	time.Sleep(50 * time.Millisecond)

	r.runningMutex.Lock()
	delete(r.runningTests, test.name)
	r.endTimes[test.name] = time.Now()
	r.executionOrder = append(r.executionOrder, test.name+":end")
	r.runningMutex.Unlock()

	// Call parent method
	r.testingSuiteRunner.RunOneTest(ctx, test)
}

func (r *trackingTestRunner) getRunningTests() []string {
	r.runningMutex.Lock()
	defer r.runningMutex.Unlock()

	var running []string
	for test := range r.runningTests {
		running = append(running, test)
	}
	return running
}

func (r *trackingTestRunner) getExecutionOrder() []string {
	r.runningMutex.Lock()
	defer r.runningMutex.Unlock()

	order := make([]string, len(r.executionOrder))
	copy(order, r.executionOrder)
	return order
}

func (r *trackingTestRunner) wereTestsRunningSimultaneously(test1, test2 string) bool {
	r.runningMutex.Lock()
	defer r.runningMutex.Unlock()

	start1, ok1 := r.startTimes[test1]
	end1, ok2 := r.endTimes[test1]
	start2, ok3 := r.startTimes[test2]
	end2, ok4 := r.endTimes[test2]

	if !ok1 || !ok2 || !ok3 || !ok4 {
		return false
	}

	// Tests overlap if one starts before the other ends
	return (start1.Before(end2) && start2.Before(end1))
}

// Test basic conflict detection - tests with same conflict should not run simultaneously
func TestConflictTracker_ConflictPrevention(t *testing.T) {
	conflictTracker := newConflictTracker()
	runner := newTrackingTestRunner()

	// Create tests with same conflict
	test1 := &testCase{
		name: "test1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}
	test2 := &testCase{
		name: "test2",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}
	test3 := &testCase{
		name: "test3",
		isolation: extensiontests.Isolation{
			Conflict: []string{"network"}, // Different conflict
		},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	var wg sync.WaitGroup

	// Test concurrent execution
	wg.Add(3)

	var success1, success2, success3 bool

	// Start test1 in goroutine
	go func() {
		defer wg.Done()
		success1 = conflictTracker.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	}()

	// Small delay then start test2 (should be blocked by test1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = conflictTracker.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	}()

	// Start test3 (different conflict, should succeed)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = conflictTracker.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	if !success1 {
		t.Error("test1 should have been able to run")
	}

	if success2 {
		t.Error("test2 should have been blocked by conflict with test1")
	}

	if !success3 {
		t.Error("test3 should have been able to run (different conflict)")
	}

	// Verify test1 and test2 didn't run simultaneously
	if runner.wereTestsRunningSimultaneously("test1", "test2") {
		t.Error("test1 and test2 should not have run simultaneously due to conflict")
	}

	// Verify test1 and test3 could run simultaneously
	if !runner.wereTestsRunningSimultaneously("test1", "test3") {
		t.Error("test1 and test3 should have been able to run simultaneously (different conflicts)")
	}
}

// Test multiple conflicts per test
func TestConflictTracker_MultipleConflicts(t *testing.T) {
	conflictTracker := newConflictTracker()
	runner := newTrackingTestRunner()

	test1 := &testCase{
		name: "test1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database", "network"},
		},
	}
	test2 := &testCase{
		name: "test2",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"}, // Conflicts with test1
		},
	}
	test3 := &testCase{
		name: "test3",
		isolation: extensiontests.Isolation{
			Conflict: []string{"network"}, // Also conflicts with test1
		},
	}
	test4 := &testCase{
		name: "test4",
		isolation: extensiontests.Isolation{
			Conflict: []string{"storage"}, // No conflicts
		},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 4

	var wg sync.WaitGroup
	wg.Add(4)

	var success1, success2, success3, success4 bool

	// Start test1
	go func() {
		defer wg.Done()
		success1 = conflictTracker.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	}()

	// test2 should be blocked (database conflict)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = conflictTracker.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	}()

	// test3 should be blocked (network conflict)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = conflictTracker.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)
	}()

	// test4 should succeed (no conflicts)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success4 = conflictTracker.tryRunTest(ctx, test4, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	if !success1 {
		t.Error("test1 should have been able to run")
	}

	if success2 {
		t.Error("test2 should be blocked by database conflict")
	}

	if success3 {
		t.Error("test3 should be blocked by network conflict")
	}

	if !success4 {
		t.Error("test4 should have been able to run (no conflicts)")
	}
}

// Test no conflicts - tests should run in parallel
func TestConflictTracker_NoConflicts(t *testing.T) {
	conflictTracker := newConflictTracker()
	runner := newTrackingTestRunner()

	// Tests with no isolation conflicts
	test1 := &testCase{name: "test1", isolation: extensiontests.Isolation{}}
	test2 := &testCase{name: "test2", isolation: extensiontests.Isolation{}}
	test3 := &testCase{name: "test3", isolation: extensiontests.Isolation{}}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	// All tests should be able to start
	success1 := conflictTracker.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	success2 := conflictTracker.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	success3 := conflictTracker.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)

	if !success1 || !success2 || !success3 {
		t.Error("All tests without conflicts should be able to run")
	}

	// Check all tests completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 3 {
		t.Errorf("Expected 3 tests to complete, got %d", len(testsRun))
	}

	// Check channel was closed (pending count reached 0)
	select {
	case _, ok := <-remainingTests:
		if ok {
			t.Error("Channel should be closed when all tests complete")
		}
	default:
		t.Error("Channel should be closed and readable")
	}
}

// Test conflict cleanup after test completion
func TestConflictTracker_ConflictCleanup(t *testing.T) {
	conflictTracker := newConflictTracker()

	test1 := &testCase{
		name: "test1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}
	test2 := &testCase{
		name: "test2",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}

	ctx := context.Background()
	runner := newTrackingTestRunner()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 2

	// Start test1
	success1 := conflictTracker.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	if !success1 {
		t.Error("test1 should have been able to run")
	}

	// Now test2 should be able to run (test1 completed and cleaned up conflicts)
	success2 := conflictTracker.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	if !success2 {
		t.Error("test2 should be able to run after test1 completed")
	}

	// Verify both tests completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}
}

// Test channel coordination with pending count
func TestDecrementAndCloseIfDone(t *testing.T) {
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	// First two decrements shouldn't close channel
	decrementAndCloseIfDone(&pendingTestCount, remainingTests)
	decrementAndCloseIfDone(&pendingTestCount, remainingTests)

	select {
	case <-remainingTests:
		t.Error("Channel should not be closed yet")
	default:
		// Expected - channel still open
	}

	// Third decrement should close channel
	decrementAndCloseIfDone(&pendingTestCount, remainingTests)

	select {
	case _, ok := <-remainingTests:
		if ok {
			t.Error("Channel should be closed after final decrement")
		}
	default:
		t.Error("Channel should be closed and readable")
	}

	// Verify counter is zero
	if atomic.LoadInt64(&pendingTestCount) != 0 {
		t.Errorf("Expected pending count to be 0, got %d", atomic.LoadInt64(&pendingTestCount))
	}
}

// Test context cancellation
func TestConflictTracker_ContextCancellation(t *testing.T) {
	conflictTracker := newConflictTracker()
	runner := newTrackingTestRunner()

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	test := &testCase{
		name: "test1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}

	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 1

	// Cancel context before running test
	cancel()

	// Test should still run even with cancelled context (depends on implementation)
	// But this tests that cancellation doesn't break the conflict tracker
	success := conflictTracker.tryRunTest(ctx, test, runner, &pendingTestCount, remainingTests)
	if !success {
		t.Error("Test should have been attempted even with cancelled context")
	}
}
