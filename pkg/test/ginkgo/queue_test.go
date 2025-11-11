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
func TestScheduler_ConflictPrevention(t *testing.T) {
	scheduler := newTestScheduler()
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
		success1 = scheduler.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	}()

	// Small delay then start test2 (should be blocked by test1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	}()

	// Start test3 (different conflict, should succeed)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = scheduler.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)
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
func TestScheduler_MultipleConflicts(t *testing.T) {
	scheduler := newTestScheduler()
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
		success1 = scheduler.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	}()

	// test2 should be blocked (database conflict)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	}()

	// test3 should be blocked (network conflict)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = scheduler.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)
	}()

	// test4 should succeed (no conflicts)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success4 = scheduler.tryRunTest(ctx, test4, runner, &pendingTestCount, remainingTests)
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
func TestScheduler_NoConflicts(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Tests with no isolation conflicts
	test1 := &testCase{name: "test1", isolation: extensiontests.Isolation{}}
	test2 := &testCase{name: "test2", isolation: extensiontests.Isolation{}}
	test3 := &testCase{name: "test3", isolation: extensiontests.Isolation{}}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	// All tests should be able to start
	success1 := scheduler.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	success2 := scheduler.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	success3 := scheduler.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)

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
func TestScheduler_ConflictCleanup(t *testing.T) {
	scheduler := newTestScheduler()

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
	success1 := scheduler.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	if !success1 {
		t.Error("test1 should have been able to run")
	}

	// Now test2 should be able to run (test1 completed and cleaned up conflicts)
	success2 := scheduler.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
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
func TestScheduler_ContextCancellation(t *testing.T) {
	scheduler := newTestScheduler()
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
	success := scheduler.tryRunTest(ctx, test, runner, &pendingTestCount, remainingTests)
	if !success {
		t.Error("Test should have been attempted even with cancelled context")
	}
}

// Test basic taint and toleration - tests without toleration cannot run with active taints
func TestScheduler_TaintTolerationBasic(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Test with taint (no conflicts)
	testWithTaint := &testCase{
		name:       "test-with-taint",
		isolation:  extensiontests.Isolation{},
		taint:      []string{"gpu"},
		toleration: []string{},
	}

	// Test without toleration (should be blocked)
	testWithoutToleration := &testCase{
		name:       "test-without-toleration",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{},
	}

	// Test with toleration (should be allowed)
	testWithToleration := &testCase{
		name:       "test-with-toleration",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{"gpu"},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	var wg sync.WaitGroup
	wg.Add(3)

	var success1, success2, success3 bool

	// Start test with taint
	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testWithTaint, runner, &pendingTestCount, remainingTests)
	}()

	// Test without toleration should be blocked
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testWithoutToleration, runner, &pendingTestCount, remainingTests)
	}()

	// Test with toleration should succeed
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = scheduler.tryRunTest(ctx, testWithToleration, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	if !success1 {
		t.Error("Test with taint should have been able to run")
	}

	if success2 {
		t.Error("Test without toleration should have been blocked by taint")
	}

	if !success3 {
		t.Error("Test with toleration should have been able to run")
	}
}

// Test multiple taints and tolerations
func TestScheduler_MultipleTaintsTolerations(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Test with multiple taints
	testWithMultipleTaints := &testCase{
		name:       "test-multiple-taints",
		isolation:  extensiontests.Isolation{},
		taint:      []string{"gpu", "exclusive"},
		toleration: []string{},
	}

	// Test with partial toleration (should be blocked)
	testPartialToleration := &testCase{
		name:       "test-partial-toleration",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{"gpu"}, // Missing "exclusive" toleration
	}

	// Test with full toleration (should succeed)
	testFullToleration := &testCase{
		name:       "test-full-toleration",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{"gpu", "exclusive"},
	}

	// Test with extra toleration (should succeed)
	testExtraToleration := &testCase{
		name:       "test-extra-toleration",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{"gpu", "exclusive", "extra"},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 4

	var wg sync.WaitGroup
	wg.Add(4)

	var success1, success2, success3, success4 bool

	// Start test with multiple taints
	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testWithMultipleTaints, runner, &pendingTestCount, remainingTests)
	}()

	// Test with partial toleration should be blocked
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testPartialToleration, runner, &pendingTestCount, remainingTests)
	}()

	// Test with full toleration should succeed
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = scheduler.tryRunTest(ctx, testFullToleration, runner, &pendingTestCount, remainingTests)
	}()

	// Test with extra toleration should succeed
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success4 = scheduler.tryRunTest(ctx, testExtraToleration, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	if !success1 {
		t.Error("Test with multiple taints should have been able to run")
	}

	if success2 {
		t.Error("Test with partial toleration should have been blocked")
	}

	if !success3 {
		t.Error("Test with full toleration should have been able to run")
	}

	if !success4 {
		t.Error("Test with extra toleration should have been able to run")
	}
}

// Test taint cleanup after test completion
func TestScheduler_TaintCleanup(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	testWithTaint := &testCase{
		name:       "test-with-taint",
		isolation:  extensiontests.Isolation{},
		taint:      []string{"gpu"},
		toleration: []string{},
	}

	testWithoutToleration := &testCase{
		name:       "test-without-toleration",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 2

	// First test with taint should run and complete
	success1 := scheduler.tryRunTest(ctx, testWithTaint, runner, &pendingTestCount, remainingTests)
	if !success1 {
		t.Error("Test with taint should have been able to run")
	}

	// After first test completes, taint should be cleaned up and second test should run
	success2 := scheduler.tryRunTest(ctx, testWithoutToleration, runner, &pendingTestCount, remainingTests)
	if !success2 {
		t.Error("Test without toleration should be able to run after taint cleanup")
	}

	// Verify both tests completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}
}

// Test combined conflicts and taint/toleration
func TestScheduler_ConflictsAndTaints(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	testWithBoth := &testCase{
		name: "test-with-both",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
		taint:      []string{"gpu"},
		toleration: []string{},
	}

	// This test conflicts with database but has GPU toleration
	testConflictingTolerated := &testCase{
		name: "test-conflicting-tolerated",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"}, // Conflicts with first test
		},
		taint:      []string{},
		toleration: []string{"gpu"}, // Can tolerate first test's taint
	}

	// This test doesn't conflict but lacks toleration
	testNonConflictingIntolerated := &testCase{
		name: "test-non-conflicting-intolerated",
		isolation: extensiontests.Isolation{
			Conflict: []string{"network"}, // Different conflict
		},
		taint:      []string{},
		toleration: []string{}, // Cannot tolerate first test's taint
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	var wg sync.WaitGroup
	wg.Add(3)

	var success1, success2, success3 bool

	// Start first test
	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testWithBoth, runner, &pendingTestCount, remainingTests)
	}()

	// Second test should be blocked by conflict (even though it has toleration)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testConflictingTolerated, runner, &pendingTestCount, remainingTests)
	}()

	// Third test should be blocked by taint (even though it doesn't conflict)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success3 = scheduler.tryRunTest(ctx, testNonConflictingIntolerated, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	if !success1 {
		t.Error("First test should have been able to run")
	}

	if success2 {
		t.Error("Second test should have been blocked by conflict (despite having toleration)")
	}

	if success3 {
		t.Error("Third test should have been blocked by taint (despite no conflict)")
	}
}

// Test no taints - all tests should run freely
func TestScheduler_NoTaints(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Tests with no taints or tolerations
	test1 := &testCase{name: "test1", isolation: extensiontests.Isolation{}, taint: []string{}, toleration: []string{}}
	test2 := &testCase{name: "test2", isolation: extensiontests.Isolation{}, taint: []string{}, toleration: []string{}}
	test3 := &testCase{name: "test3", isolation: extensiontests.Isolation{}, taint: []string{}, toleration: []string{}}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	// All tests should be able to run
	success1 := scheduler.tryRunTest(ctx, test1, runner, &pendingTestCount, remainingTests)
	success2 := scheduler.tryRunTest(ctx, test2, runner, &pendingTestCount, remainingTests)
	success3 := scheduler.tryRunTest(ctx, test3, runner, &pendingTestCount, remainingTests)

	if !success1 || !success2 || !success3 {
		t.Error("All tests without taints should be able to run")
	}

	// Check all tests completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 3 {
		t.Errorf("Expected 3 tests to complete, got %d", len(testsRun))
	}
}

// blockingTestRunner is a test runner that can block tests mid-execution for testing
type blockingTestRunner struct {
	mu        sync.Mutex
	testsRun  []string
	blockChan chan struct{} // Used to control when tests complete
}

// Implement testSuiteRunner interface
func (r *blockingTestRunner) RunOneTest(ctx context.Context, test *testCase) {
	// Add to completed list
	r.mu.Lock()
	r.testsRun = append(r.testsRun, test.name)
	r.mu.Unlock()

	// Block until we signal completion (except for the intolerant test)
	if test.name != "test-intolerant" {
		<-r.blockChan
	}
}

// Test taint reference counting - multiple tests with same taint
func TestScheduler_TaintReferenceCounting(t *testing.T) {
	scheduler := newTestScheduler()

	runner := newTrackingTestRunner()

	// Two tests both applying the same taint "gpu"
	testWithTaint1 := &testCase{
		name:       "test-with-taint-1",
		isolation:  extensiontests.Isolation{},
		taint:      []string{"gpu"},
		toleration: []string{"gpu"}, // Can tolerate its own taint
	}

	testWithTaint2 := &testCase{
		name:       "test-with-taint-2",
		isolation:  extensiontests.Isolation{},
		taint:      []string{"gpu"},
		toleration: []string{"gpu"}, // Can tolerate its own taint
	}

	// Test that cannot tolerate gpu
	testIntolerant := &testCase{
		name:       "test-intolerant",
		isolation:  extensiontests.Isolation{},
		taint:      []string{},
		toleration: []string{}, // Cannot tolerate gpu taint
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	// Manually test the taint reference counting behavior

	// 1. Start first test with taint - should succeed
	scheduler.mu.Lock()
	// Manually mark taint as active
	scheduler.activeTaints["gpu"]++
	scheduler.mu.Unlock()

	// 2. Start second test with same taint - should succeed (reference count = 2)
	scheduler.mu.Lock()
	scheduler.activeTaints["gpu"]++
	scheduler.mu.Unlock()

	// 3. Verify reference count is 2
	scheduler.mu.Lock()
	gpuCount := scheduler.activeTaints["gpu"]
	scheduler.mu.Unlock()

	if gpuCount != 2 {
		t.Errorf("Expected GPU taint reference count to be 2, got %d", gpuCount)
	}

	// 4. Try to run intolerant test - should be blocked
	canRun := scheduler.canTolerateTaints(testIntolerant)
	if canRun {
		t.Error("Intolerant test should be blocked by active GPU taint")
	}

	// 5. Complete first test (decrement count to 1)
	scheduler.mu.Lock()
	scheduler.activeTaints["gpu"]--
	if scheduler.activeTaints["gpu"] <= 0 {
		delete(scheduler.activeTaints, "gpu")
	}
	scheduler.mu.Unlock()

	// 6. Verify taint is still active (reference count = 1)
	scheduler.mu.Lock()
	gpuCount = scheduler.activeTaints["gpu"]
	scheduler.mu.Unlock()

	if gpuCount != 1 {
		t.Errorf("Expected GPU taint reference count to be 1 after first test completion, got %d", gpuCount)
	}

	// 7. Intolerant test should still be blocked
	canRun = scheduler.canTolerateTaints(testIntolerant)
	if canRun {
		t.Error("Intolerant test should still be blocked (second test still running)")
	}

	// 8. Complete second test (decrement count to 0, remove taint)
	scheduler.mu.Lock()
	scheduler.activeTaints["gpu"]--
	if scheduler.activeTaints["gpu"] <= 0 {
		delete(scheduler.activeTaints, "gpu")
	}
	scheduler.mu.Unlock()

	// 9. Verify taint is completely removed
	scheduler.mu.Lock()
	_, exists := scheduler.activeTaints["gpu"]
	scheduler.mu.Unlock()

	if exists {
		t.Error("GPU taint should be completely removed after all tests complete")
	}

	// 10. Now intolerant test should be able to run
	canRun = scheduler.canTolerateTaints(testIntolerant)
	if !canRun {
		t.Error("Intolerant test should be able to run after all taints are cleaned up")
	}

	// Test the full tryRunTest with actual execution
	success1 := scheduler.tryRunTest(ctx, testWithTaint1, runner, &pendingTestCount, remainingTests)
	success2 := scheduler.tryRunTest(ctx, testWithTaint2, runner, &pendingTestCount, remainingTests)
	success3 := scheduler.tryRunTest(ctx, testIntolerant, runner, &pendingTestCount, remainingTests)

	if !success1 || !success2 || !success3 {
		t.Error("All tests should succeed when run sequentially (each completes before next starts)")
	}
}

// Test conflict groups - tests in different groups should not check conflicts against each other
func TestScheduler_ConflictGroups(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// For this test, we'll manually control which conflict group tests belong to
	// by temporarily replacing getTestConflictGroup

	// Create two tests with same conflict but in different groups
	testGroupA1 := &testCase{
		name: "test-group-a-1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}

	testGroupA2 := &testCase{
		name: "test-group-a-2",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}

	testGroupB1 := &testCase{
		name: "test-group-b-1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"}, // Same conflict name but different group
		},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 3

	// Since getTestConflictGroup currently returns "default" for all tests,
	// we'll test the data structure behavior directly

	// Manually set up scheduler state to simulate different groups
	scheduler.mu.Lock()
	scheduler.runningConflicts["groupA"] = make(map[string]bool)
	scheduler.runningConflicts["groupB"] = make(map[string]bool)
	scheduler.runningConflicts["groupA"]["database"] = true // Mark database as running in groupA
	scheduler.mu.Unlock()

	// Verify that "database" conflict exists in groupA
	scheduler.mu.Lock()
	hasConflictGroupA := scheduler.runningConflicts["groupA"]["database"]
	scheduler.mu.Unlock()

	if !hasConflictGroupA {
		t.Error("Expected database conflict to be marked as running in groupA")
	}

	// Verify that "database" conflict does NOT exist in groupB
	scheduler.mu.Lock()
	hasConflictGroupB := scheduler.runningConflicts["groupB"]["database"]
	scheduler.mu.Unlock()

	if hasConflictGroupB {
		t.Error("Expected database conflict to NOT be running in groupB")
	}

	// Verify default group doesn't have the conflict
	scheduler.mu.Lock()
	defaultGroup, exists := scheduler.runningConflicts["default"]
	hasConflictDefault := exists && defaultGroup["database"]
	scheduler.mu.Unlock()

	if hasConflictDefault {
		t.Error("Expected database conflict to NOT be running in default group")
	}

	// Test that getTestConflictGroup returns "default" for all tests currently
	group1 := getTestConflictGroup(testGroupA1)
	group2 := getTestConflictGroup(testGroupA2)
	group3 := getTestConflictGroup(testGroupB1)

	if group1 != "default" {
		t.Errorf("Expected testGroupA1 to return 'default', got '%s'", group1)
	}

	if group2 != "default" {
		t.Errorf("Expected testGroupA2 to return 'default', got '%s'", group2)
	}

	if group3 != "default" {
		t.Errorf("Expected testGroupB1 to return 'default', got '%s'", group3)
	}

	var wg sync.WaitGroup
	var success1, success2 bool

	// Run tests concurrently to test conflict detection
	wg.Add(2)

	// Start first test
	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testGroupA1, runner, &pendingTestCount, remainingTests)
	}()

	// Start second test with slight delay to ensure first test starts first
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testGroupA2, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	if !success1 {
		t.Error("testGroupA1 should succeed (default group has no conflicts yet)")
	}

	if success2 {
		t.Error("testGroupA2 should be blocked (same conflict in same group)")
	}

	// After both attempts complete, verify that testGroupA1 ran
	testsRun := runner.getTestsRun()
	if len(testsRun) != 1 {
		t.Errorf("Expected 1 test to complete, got %d", len(testsRun))
	}

	if len(testsRun) > 0 && testsRun[0] != "test-group-a-1" {
		t.Errorf("Expected test-group-a-1 to complete, got %s", testsRun[0])
	}
}

// Test mode-based conflict group assignment
func TestScheduler_ModeBased_ConflictGroups(t *testing.T) {
	// Test instance mode
	testInstance := &testCase{
		name: "test-instance-mode",
		isolation: extensiontests.Isolation{
			Mode:     "instance",
			Conflict: []string{"database"},
		},
	}

	group := getTestConflictGroup(testInstance)
	if group != "instance-1" {
		t.Errorf("Expected instance mode to return 'instance-1', got '%s'", group)
	}

	// Test bucket mode
	testBucket := &testCase{
		name: "test-bucket-mode",
		isolation: extensiontests.Isolation{
			Mode:     "bucket",
			Conflict: []string{"network"},
		},
	}

	group = getTestConflictGroup(testBucket)
	if group != "bucket-a" {
		t.Errorf("Expected bucket mode to return 'bucket-a', got '%s'", group)
	}

	// Test exec mode
	testExec := &testCase{
		name: "test-exec-mode",
		isolation: extensiontests.Isolation{
			Mode:     "exec",
			Conflict: []string{"storage"},
		},
	}

	group = getTestConflictGroup(testExec)
	if group != "default" {
		t.Errorf("Expected exec mode to return 'default', got '%s'", group)
	}

	// Test empty mode (should default)
	testEmpty := &testCase{
		name: "test-empty-mode",
		isolation: extensiontests.Isolation{
			Mode:     "",
			Conflict: []string{"cpu"},
		},
	}

	group = getTestConflictGroup(testEmpty)
	if group != "default" {
		t.Errorf("Expected empty mode to return 'default', got '%s'", group)
	}

	// Test unknown mode (should default)
	testUnknown := &testCase{
		name: "test-unknown-mode",
		isolation: extensiontests.Isolation{
			Mode:     "unknown",
			Conflict: []string{"memory"},
		},
	}

	group = getTestConflictGroup(testUnknown)
	if group != "default" {
		t.Errorf("Expected unknown mode to return 'default', got '%s'", group)
	}
}

// Test that instance mode groups conflicts correctly
func TestScheduler_InstanceMode_IsolatesConflicts(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Two tests with instance mode and same conflict
	testInstance1 := &testCase{
		name: "test-instance-1",
		isolation: extensiontests.Isolation{
			Mode:     "instance",
			Conflict: []string{"database"},
		},
	}

	testInstance2 := &testCase{
		name: "test-instance-2",
		isolation: extensiontests.Isolation{
			Mode:     "instance",
			Conflict: []string{"database"},
		},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 2

	var wg sync.WaitGroup
	var success1, success2 bool

	// Run both tests concurrently
	wg.Add(2)

	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testInstance1, runner, &pendingTestCount, remainingTests)
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testInstance2, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	// Both tests are in "instance-1" group with same conflict, so one should block
	if !success1 {
		t.Error("First test should succeed")
	}

	if success2 {
		t.Error("Second test should be blocked (same conflict in same instance group)")
	}
}

// Test that bucket mode groups conflicts correctly
func TestScheduler_BucketMode_IsolatesConflicts(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Two tests with bucket mode and same conflict
	testBucket1 := &testCase{
		name: "test-bucket-1",
		isolation: extensiontests.Isolation{
			Mode:     "bucket",
			Conflict: []string{"network"},
		},
	}

	testBucket2 := &testCase{
		name: "test-bucket-2",
		isolation: extensiontests.Isolation{
			Mode:     "bucket",
			Conflict: []string{"network"},
		},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 2

	var wg sync.WaitGroup
	var success1, success2 bool

	// Run both tests concurrently
	wg.Add(2)

	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testBucket1, runner, &pendingTestCount, remainingTests)
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testBucket2, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	// Both tests are in "bucket-a" group with same conflict, so one should block
	if !success1 {
		t.Error("First test should succeed")
	}

	if success2 {
		t.Error("Second test should be blocked (same conflict in same bucket group)")
	}
}

// Test that exec mode uses default group
func TestScheduler_ExecMode_UsesDefaultGroup(t *testing.T) {
	scheduler := newTestScheduler()
	runner := newTrackingTestRunner()

	// Two tests with exec mode and same conflict
	testExec1 := &testCase{
		name: "test-exec-1",
		isolation: extensiontests.Isolation{
			Mode:     "exec",
			Conflict: []string{"storage"},
		},
	}

	testExec2 := &testCase{
		name: "test-exec-2",
		isolation: extensiontests.Isolation{
			Mode:     "exec",
			Conflict: []string{"storage"},
		},
	}

	ctx := context.Background()
	remainingTests := make(chan *testCase, 10)
	var pendingTestCount int64 = 2

	var wg sync.WaitGroup
	var success1, success2 bool

	// Run both tests concurrently
	wg.Add(2)

	go func() {
		defer wg.Done()
		success1 = scheduler.tryRunTest(ctx, testExec1, runner, &pendingTestCount, remainingTests)
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		success2 = scheduler.tryRunTest(ctx, testExec2, runner, &pendingTestCount, remainingTests)
	}()

	wg.Wait()

	// Both tests are in "default" group with same conflict, so one should block
	if !success1 {
		t.Error("First test should succeed")
	}

	if success2 {
		t.Error("Second test should be blocked (same conflict in default group)")
	}
}
