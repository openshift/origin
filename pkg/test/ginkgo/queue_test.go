package ginkgo

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	_ "embed"

	extensiontests "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"k8s.io/apimachinery/pkg/util/sets"
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

// tryScheduleAndRunNext is a test helper that attempts to get and run the next test from a scheduler
// Returns the test that was executed, or nil if no test could run
// This matches the production code pattern where workers poll from the scheduler
func tryScheduleAndRunNext(ctx context.Context, scheduler TestScheduler, runner testSuiteRunner) *testCase {
	// Try to get next runnable test from scheduler (matches production logic)
	test := scheduler.GetNextTestToRun(ctx)

	if test == nil {
		return nil // No runnable test available or context cancelled
	}

	// Run the test (matches production logic)
	runner.RunOneTest(ctx, test)

	// Mark test as complete (matches production logic)
	scheduler.MarkTestComplete(test)

	return test
}

// Test basic conflict detection - tests with same conflict should not run simultaneously
func TestScheduler_ConflictPrevention(t *testing.T) {
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

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{test1, test2, test3})
	ctx := context.Background()

	// Use 2 workers to process 3 tests (matches production pattern with limited parallelism)
	// This ensures test2 waits in queue while test1 and test3 run
	var wg sync.WaitGroup
	wg.Add(2)

	// Start 2 workers that will process all tests
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// All tests should have completed (test2 runs after test1 finishes)
	testsRun := runner.getTestsRun()
	if len(testsRun) != 3 {
		t.Errorf("Expected all 3 tests to complete, got %d: %v", len(testsRun), testsRun)
	}

	// Verify test1 and test2 didn't run simultaneously (conflict prevents this)
	if runner.wereTestsRunningSimultaneously("test1", "test2") {
		t.Error("test1 and test2 should not have run simultaneously due to conflict")
	}

	// Verify test1 and test3 could run simultaneously (different conflicts)
	if !runner.wereTestsRunningSimultaneously("test1", "test3") {
		t.Error("test1 and test3 should have been able to run simultaneously (different conflicts)")
	}
}

// Test multiple conflicts per test
func TestScheduler_MultipleConflicts(t *testing.T) {
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

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{test1, test2, test3, test4})
	ctx := context.Background()

	// Use 2 workers to process 4 tests (matches production pattern)
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// All 4 tests should have completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 4 {
		t.Errorf("Expected all 4 tests to complete, got %d: %v", len(testsRun), testsRun)
	}

	// Verify test1 doesn't run simultaneously with test2 or test3 (due to conflicts)
	if runner.wereTestsRunningSimultaneously("test1", "test2") {
		t.Error("test1 and test2 should not run simultaneously (database conflict)")
	}
	if runner.wereTestsRunningSimultaneously("test1", "test3") {
		t.Error("test1 and test3 should not run simultaneously (network conflict)")
	}

	// Verify test1 and test4 can run simultaneously (no conflicts)
	if !runner.wereTestsRunningSimultaneously("test1", "test4") {
		t.Error("test1 and test4 should be able to run simultaneously (different conflicts)")
	}
}

// Test no conflicts - tests should run in parallel
func TestScheduler_NoConflicts(t *testing.T) {
	runner := newTrackingTestRunner()

	// Tests with no isolation conflicts
	test1 := &testCase{name: "test1", isolation: extensiontests.Isolation{}}
	test2 := &testCase{name: "test2", isolation: extensiontests.Isolation{}}
	test3 := &testCase{name: "test3", isolation: extensiontests.Isolation{}}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{test1, test2, test3})
	ctx := context.Background()

	// All tests should be able to run
	ran1 := tryScheduleAndRunNext(ctx, scheduler, runner)
	ran2 := tryScheduleAndRunNext(ctx, scheduler, runner)
	ran3 := tryScheduleAndRunNext(ctx, scheduler, runner)

	if ran1 == nil || ran2 == nil || ran3 == nil {
		t.Error("All tests without conflicts should be able to run")
	}

	// Check all tests completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 3 {
		t.Errorf("Expected 3 tests to complete, got %d", len(testsRun))
	}
}

// Test conflict cleanup after test completion
func TestScheduler_ConflictCleanup(t *testing.T) {
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

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{test1, test2})

	// test1 should run
	ran1 := tryScheduleAndRunNext(ctx, scheduler, runner)
	if ran1 == nil || ran1.name != "test1" {
		t.Error("test1 should have been able to run")
	}

	// test2 should now be able to run (test1 completed and cleaned up conflicts)
	ran2 := tryScheduleAndRunNext(ctx, scheduler, runner)
	if ran2 == nil || ran2.name != "test2" {
		t.Error("test2 should be able to run after test1 completed")
	}

	// Verify both tests completed
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}
}

// Test context cancellation
func TestScheduler_ContextCancellation(t *testing.T) {
	runner := newTrackingTestRunner()

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	test := &testCase{
		name: "test1",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
		},
	}

	// Create scheduler with test (matches production)
	scheduler := newTestScheduler([]*testCase{test})

	// Cancel context before trying to get test
	cancel()

	// GetNextTestToRun should respect context cancellation and return nil
	ranTest := tryScheduleAndRunNext(ctx, scheduler, runner)
	if ranTest != nil {
		t.Error("No test should run when context is cancelled")
	}

	// Verify no tests ran
	testsRun := runner.getTestsRun()
	if len(testsRun) != 0 {
		t.Errorf("Expected 0 tests to run with cancelled context, got %d", len(testsRun))
	}
}

// Test basic taint and toleration - tests without toleration cannot run with active taints
func TestScheduler_TaintTolerationBasic(t *testing.T) {
	runner := newTrackingTestRunner()

	// Test with taint (no conflicts)
	testWithTaint := &testCase{
		name: "test-with-taint",
		isolation: extensiontests.Isolation{
			Taint: []string{"gpu"},
		},
	}

	// Test without toleration (blocked until testWithTaint completes)
	testWithoutToleration := &testCase{
		name:      "test-without-toleration",
		isolation: extensiontests.Isolation{},
	}

	// Test with toleration (can run with testWithTaint)
	testWithToleration := &testCase{
		name: "test-with-toleration",
		isolation: extensiontests.Isolation{
			Toleration: []string{"gpu"},
		},
	}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testWithTaint, testWithoutToleration, testWithToleration})
	ctx := context.Background()

	// Use 2 workers so they can run tests in parallel when possible
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// All tests should complete
	testsRun := runner.getTestsRun()
	if len(testsRun) != 3 {
		t.Errorf("Expected all 3 tests to complete, got %d", len(testsRun))
	}

	// testWithTaint and testWithToleration can run simultaneously (toleration allows it)
	if !runner.wereTestsRunningSimultaneously("test-with-taint", "test-with-toleration") {
		t.Error("testWithTaint and testWithToleration should run simultaneously (toleration permits)")
	}

	// testWithTaint and testWithoutToleration should NOT run simultaneously (no toleration)
	if runner.wereTestsRunningSimultaneously("test-with-taint", "test-without-toleration") {
		t.Error("testWithTaint and testWithoutToleration should not run simultaneously (missing toleration)")
	}
}

// Test multiple taints and tolerations
func TestScheduler_MultipleTaintsTolerations(t *testing.T) {
	runner := newTrackingTestRunner()

	// Test with multiple taints
	testWithMultipleTaints := &testCase{
		name: "test-multiple-taints",
		isolation: extensiontests.Isolation{
			Taint: []string{"gpu", "exclusive"},
		},
	}

	// Test with partial toleration (blocked until testWithMultipleTaints completes)
	testPartialToleration := &testCase{
		name: "test-partial-toleration",
		isolation: extensiontests.Isolation{
			Toleration: []string{"gpu"}, // Missing "exclusive" toleration
		},
	}

	// Test with full toleration (can run with testWithMultipleTaints)
	testFullToleration := &testCase{
		name: "test-full-toleration",
		isolation: extensiontests.Isolation{
			Toleration: []string{"gpu", "exclusive"},
		},
	}

	// Test with extra toleration (can run with testWithMultipleTaints)
	testExtraToleration := &testCase{
		name: "test-extra-toleration",
		isolation: extensiontests.Isolation{
			Toleration: []string{"gpu", "exclusive", "extra"},
		},
	}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testWithMultipleTaints, testPartialToleration, testFullToleration, testExtraToleration})
	ctx := context.Background()

	// Use 3 workers to process 4 tests
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// All tests should complete
	testsRun := runner.getTestsRun()
	if len(testsRun) != 4 {
		t.Errorf("Expected all 4 tests to complete, got %d", len(testsRun))
	}

	// Tests with full/extra toleration can run simultaneously with testWithMultipleTaints
	if !runner.wereTestsRunningSimultaneously("test-multiple-taints", "test-full-toleration") {
		t.Error("testWithMultipleTaints and testFullToleration should run simultaneously")
	}
	if !runner.wereTestsRunningSimultaneously("test-multiple-taints", "test-extra-toleration") {
		t.Error("testWithMultipleTaints and testExtraToleration should run simultaneously")
	}

	// Test with partial toleration should NOT run simultaneously (missing "exclusive")
	if runner.wereTestsRunningSimultaneously("test-multiple-taints", "test-partial-toleration") {
		t.Error("testWithMultipleTaints and testPartialToleration should not run simultaneously (partial toleration)")
	}
}

// Test taint cleanup after test completion
func TestScheduler_TaintCleanup(t *testing.T) {
	runner := newTrackingTestRunner()

	testWithTaint := &testCase{
		name: "test-with-taint",
		isolation: extensiontests.Isolation{
			Taint: []string{"gpu"},
		},
	}

	testWithoutToleration := &testCase{
		name:      "test-without-toleration",
		isolation: extensiontests.Isolation{},
	}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testWithTaint, testWithoutToleration})
	ctx := context.Background()

	// First test with taint should run and complete
	ranTest1 := tryScheduleAndRunNext(ctx, scheduler, runner)
	if ranTest1 == nil {
		t.Error("Test with taint should have been able to run")
	}

	// After first test completes, taint should be cleaned up and second test should run
	ranTest2 := tryScheduleAndRunNext(ctx, scheduler, runner)
	if ranTest2 == nil {
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
	runner := newTrackingTestRunner()

	testWithBoth := &testCase{
		name: "test-with-both",
		isolation: extensiontests.Isolation{
			Conflict: []string{"database"},
			Taint:    []string{"gpu"},
		},
	}

	// This test conflicts with database but has GPU toleration
	testConflictingTolerated := &testCase{
		name: "test-conflicting-tolerated",
		isolation: extensiontests.Isolation{
			Conflict:   []string{"database"}, // Conflicts with first test
			Toleration: []string{"gpu"},      // Can tolerate first test's taint
		},
	}

	// This test doesn't conflict but lacks toleration
	testNonConflictingIntolerated := &testCase{
		name: "test-non-conflicting-intolerated",
		isolation: extensiontests.Isolation{
			Conflict: []string{"network"}, // Different conflict
			// Cannot tolerate first test's taint
		},
	}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testWithBoth, testConflictingTolerated, testNonConflictingIntolerated})
	ctx := context.Background()

	// Use 2 workers
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// All tests should complete
	testsRun := runner.getTestsRun()
	if len(testsRun) != 3 {
		t.Errorf("Expected all 3 tests to complete, got %d", len(testsRun))
	}

	// testWithBoth and testConflictingTolerated should NOT run simultaneously (conflict prevents it)
	if runner.wereTestsRunningSimultaneously("test-with-both", "test-conflicting-tolerated") {
		t.Error("testWithBoth and testConflictingTolerated should not run simultaneously (conflict)")
	}

	// testWithBoth and testNonConflictingIntolerated should NOT run simultaneously (taint prevents it)
	if runner.wereTestsRunningSimultaneously("test-with-both", "test-non-conflicting-intolerated") {
		t.Error("testWithBoth and testNonConflictingIntolerated should not run simultaneously (taint)")
	}
}

// Test no taints - all tests should run freely
func TestScheduler_NoTaints(t *testing.T) {
	runner := newTrackingTestRunner()

	// Tests with no taints or tolerations
	test1 := &testCase{name: "test1", isolation: extensiontests.Isolation{}}
	test2 := &testCase{name: "test2", isolation: extensiontests.Isolation{}}
	test3 := &testCase{name: "test3", isolation: extensiontests.Isolation{}}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{test1, test2, test3})
	ctx := context.Background()

	// All tests should be able to run
	ranTest1 := tryScheduleAndRunNext(ctx, scheduler, runner)
	ranTest2 := tryScheduleAndRunNext(ctx, scheduler, runner)
	ranTest3 := tryScheduleAndRunNext(ctx, scheduler, runner)

	if ranTest1 == nil || ranTest2 == nil || ranTest3 == nil {
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

	// Block tests that have taints or tolerations (those are the ones we want to keep running)
	// Don't block tests that have neither (they should be blocked by scheduler)
	if len(test.isolation.Taint) > 0 || len(test.isolation.Toleration) > 0 {
		<-r.blockChan
	}
}

// Test taint reference counting - multiple tests with same taint
func TestScheduler_TaintReferenceCounting(t *testing.T) {
	scheduler := newTestScheduler([]*testCase{}).(*testScheduler)

	runner := newTrackingTestRunner()

	// Two tests both applying the same taint "gpu"
	testWithTaint1 := &testCase{
		name: "test-with-taint-1",
		isolation: extensiontests.Isolation{
			Taint:      []string{"gpu"},
			Toleration: []string{"gpu"}, // Can tolerate its own taint
		},
	}

	testWithTaint2 := &testCase{
		name: "test-with-taint-2",
		isolation: extensiontests.Isolation{
			Taint:      []string{"gpu"},
			Toleration: []string{"gpu"}, // Can tolerate its own taint
		},
	}

	// Test that cannot tolerate gpu
	testIntolerant := &testCase{
		name:      "test-intolerant",
		isolation: extensiontests.Isolation{
			// Cannot tolerate gpu taint
		},
	}

	ctx := context.Background()

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

	// Test the full execution with actual tests through the scheduler
	// Create a new scheduler with all three tests for sequential execution
	execScheduler := newTestScheduler([]*testCase{testWithTaint1, testWithTaint2, testIntolerant})

	ranTest1 := tryScheduleAndRunNext(ctx, execScheduler, runner)
	ranTest2 := tryScheduleAndRunNext(ctx, execScheduler, runner)
	ranTest3 := tryScheduleAndRunNext(ctx, execScheduler, runner)

	if ranTest1 == nil || ranTest2 == nil || ranTest3 == nil {
		t.Error("All tests should succeed when run sequentially (each completes before next starts)")
	}
}

// Test conflict groups - tests in different groups should not check conflicts against each other
func TestScheduler_ConflictGroups(t *testing.T) {
	scheduler := newTestScheduler([]*testCase{}).(*testScheduler)
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

	// Since getTestConflictGroup currently returns "default" for all tests,
	// we'll test the data structure behavior directly

	// Manually set up scheduler state to simulate different groups
	scheduler.mu.Lock()
	scheduler.runningConflicts["groupA"] = sets.New[string]()
	scheduler.runningConflicts["groupB"] = sets.New[string]()
	scheduler.runningConflicts["groupA"].Insert("database") // Mark database as running in groupA
	scheduler.mu.Unlock()

	// Verify that "database" conflict exists in groupA
	scheduler.mu.Lock()
	hasConflictGroupA := scheduler.runningConflicts["groupA"].Has("database")
	scheduler.mu.Unlock()

	if !hasConflictGroupA {
		t.Error("Expected database conflict to be marked as running in groupA")
	}

	// Verify that "database" conflict does NOT exist in groupB
	scheduler.mu.Lock()
	hasConflictGroupB := scheduler.runningConflicts["groupB"].Has("database")
	scheduler.mu.Unlock()

	if hasConflictGroupB {
		t.Error("Expected database conflict to NOT be running in groupB")
	}

	// Verify default group doesn't have the conflict
	scheduler.mu.Lock()
	defaultGroup, exists := scheduler.runningConflicts["default"]
	hasConflictDefault := exists && defaultGroup.Has("database")
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

	// Create scheduler with tests (matches production)
	execScheduler := newTestScheduler([]*testCase{testGroupA1, testGroupA2})

	var wg sync.WaitGroup
	wg.Add(2)

	// Use 2 workers to process tests
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, execScheduler, runner)
		}()
	}

	wg.Wait()

	// Both tests should complete (testGroupA2 runs after testGroupA1)
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}

	// They should not run simultaneously (same conflict)
	if runner.wereTestsRunningSimultaneously("test-group-a-1", "test-group-a-2") {
		t.Error("testGroupA1 and testGroupA2 should not run simultaneously (same conflict in default group)")
	}
}

// Test conflict group assignment - now simplified to always return "default"
func TestScheduler_ModeBased_ConflictGroups(t *testing.T) {
	// Test that all modes now return "default" (simplified behavior)
	testCases := []struct {
		name string
		mode string
	}{
		{"instance mode", "instance"},
		{"bucket mode", "bucket"},
		{"exec mode", "exec"},
		{"empty mode", ""},
		{"unknown mode", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			test := &testCase{
				name: "test-" + tc.name,
				isolation: extensiontests.Isolation{
					Mode:     tc.mode,
					Conflict: []string{"test-conflict"},
				},
			}

			group := getTestConflictGroup(test)
			if group != "default" {
				t.Errorf("Expected %s to return 'default', got '%s'", tc.name, group)
			}
		})
	}
}

// Test that instance mode groups conflicts correctly
func TestScheduler_InstanceMode_IsolatesConflicts(t *testing.T) {
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

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testInstance1, testInstance2})
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// Both tests should complete (testInstance2 runs after testInstance1)
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}

	// Both tests are in "default" group with same conflict, so they should not run simultaneously
	if runner.wereTestsRunningSimultaneously("test-instance-1", "test-instance-2") {
		t.Error("testInstance1 and testInstance2 should not run simultaneously (same conflict in default group)")
	}
}

// Test that bucket mode groups conflicts correctly
func TestScheduler_BucketMode_IsolatesConflicts(t *testing.T) {
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

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testBucket1, testBucket2})
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// Both tests should complete (testBucket2 runs after testBucket1)
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}

	// Both tests are in "default" group with same conflict, so they should not run simultaneously
	if runner.wereTestsRunningSimultaneously("test-bucket-1", "test-bucket-2") {
		t.Error("testBucket1 and testBucket2 should not run simultaneously (same conflict in default group)")
	}
}

// Test that exec mode uses default group
func TestScheduler_ExecMode_UsesDefaultGroup(t *testing.T) {
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

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{testExec1, testExec2})
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, runner)
		}()
	}

	wg.Wait()

	// Both tests should complete (testExec2 runs after testExec1)
	testsRun := runner.getTestsRun()
	if len(testsRun) != 2 {
		t.Errorf("Expected 2 tests to complete, got %d", len(testsRun))
	}

	// Both tests are in "default" group with same conflict, so they should not run simultaneously
	if runner.wereTestsRunningSimultaneously("test-exec-1", "test-exec-2") {
		t.Error("testExec1 and testExec2 should not run simultaneously (same conflict in default group)")
	}
}

// Test that queue maintains test order when conflicts are resolved
func TestQueue_MaintainsOrderWithConflicts(t *testing.T) {
	// Create tests with dependencies: test1 and test2 conflict, test3 doesn't conflict
	// Expected behavior: scheduler should skip test2 and return test3, maintaining test2's position
	test1 := &testCase{
		name:      "test1-conflict-db",
		isolation: extensiontests.Isolation{Conflict: []string{"database"}},
	}
	test2 := &testCase{
		name:      "test2-conflict-db",
		isolation: extensiontests.Isolation{Conflict: []string{"database"}},
	}
	test3 := &testCase{
		name:      "test3-no-conflict",
		isolation: extensiontests.Isolation{},
	}

	// Create scheduler with all tests (matches production)
	scheduler := newTestScheduler([]*testCase{test1, test2, test3})
	ctx := context.Background()

	// Step 1: Get test1 and mark it as running (but don't run it yet to keep conflict active)
	firstTest := scheduler.GetNextTestToRun(ctx)
	if firstTest == nil || firstTest.name != "test1-conflict-db" {
		t.Errorf("Expected first call to return test1-conflict-db, got %v", firstTest)
	}
	// Note: GetNextTestToRun already marked the conflict as running

	// Step 2: Try to get next test while test1 is "running"
	// Should skip test2 (conflicts with running test1) and return test3
	secondTest := scheduler.GetNextTestToRun(ctx)
	if secondTest == nil || secondTest.name != "test3-no-conflict" {
		t.Errorf("Expected second call to return test3-no-conflict (skipping blocked test2), got %v", secondTest)
	}

	// Step 3: Clean up test1's conflict (simulate test1 completing)
	scheduler.MarkTestComplete(test1)

	// Step 4: Now test2 should be runnable and returned (it maintained its position in queue)
	thirdTest := scheduler.GetNextTestToRun(ctx)
	if thirdTest == nil || thirdTest.name != "test2-conflict-db" {
		t.Errorf("Expected third call to return test2-conflict-db (now unblocked), got %v", thirdTest)
	}

	// Clean up test2
	scheduler.MarkTestComplete(test2)

	// All tests have been retrieved and completed successfully
}
