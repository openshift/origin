package ginkgo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const nodeResourceLabelKey = "noderesource.test.openshift.io/name"

// nodeResourceSuiteName is the only suite that uses the NodeResource bucket
// and dedicated pool. Other suites may carry [NodeResource:...] tags on
// individual tests without triggering pool provisioning (see cmd_runsuite.go).
const nodeResourceSuiteName = "openshift/disruptive-longrunning"

// nodeResourceNoProgressDeadline is the idle stall limit when no test holds
// pool nodes. Must exceed the longest test (suite timeout 40m). In-flight
// tests holding reserved nodes do not count toward this deadline.
const nodeResourceNoProgressDeadline = 45 * time.Minute

// nodeResourceUnlabelTimeout bounds API waits when removing reservation labels.
const nodeResourceUnlabelTimeout = 2 * time.Minute

var nodeResourceTagRe = regexp.MustCompile(`\[NodeResource:numNodes=([^,]+),label=([^\]]+)\]`)

type nodeResourceConfig struct {
	numNodes int
	label    string
	isAll    bool
}

func parseNodeResourceTag(testName string) (*nodeResourceConfig, error) {
	match := nodeResourceTagRe.FindStringSubmatch(testName)
	if match == nil {
		return nil, fmt.Errorf("no [NodeResource:...] tag found in %q", testName)
	}
	numNodesStr := match[1]
	label := match[2]

	cfg := &nodeResourceConfig{label: label}
	if numNodesStr == "all" {
		cfg.isAll = true
		cfg.numNodes = -1
	} else {
		n, err := strconv.Atoi(numNodesStr)
		if err != nil {
			return nil, fmt.Errorf("invalid numNodes %q in test %q: %w", numNodesStr, testName, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("numNodes must be positive or 'all', got %q in test %q", numNodesStr, testName)
		}
		cfg.numNodes = n
	}
	return cfg, nil
}

type nodeResourceScheduler struct {
	mu          sync.Mutex
	cond        *sync.Cond
	kubeClient  kubernetes.Interface
	workerNodes []string
	reservedBy  map[string]string
	tests       []*testCase
	configs     map[string]*nodeResourceConfig
	// lastProgress is updated every time a test is successfully dispatched
	// or completes, and is used to detect a scheduler that has stalled with
	// no forward progress (see nodeResourceNoProgressDeadline).
	lastProgress time.Time
}

// newNodeResourceScheduler schedules [NodeResource] tests onto workerNames
// (typically nodes from createStaticNodeResourcePool).
func newNodeResourceScheduler(ctx context.Context, kubeClient kubernetes.Interface, tests []*testCase, workerNames []string) (*nodeResourceScheduler, error) {
	if len(workerNames) == 0 {
		return nil, fmt.Errorf("no worker nodes available for NodeResource tests")
	}
	workerNames = append([]string(nil), workerNames...)
	sort.Strings(workerNames)

	configs := make(map[string]*nodeResourceConfig, len(tests))
	for _, t := range tests {
		cfg, err := parseNodeResourceTag(t.name)
		if err != nil {
			return nil, err
		}
		needed := cfg.numNodes
		if cfg.isAll {
			needed = len(workerNames)
		}
		if needed > len(workerNames) {
			return nil, fmt.Errorf("test %q requires %d nodes but only %d workers available", t.name, needed, len(workerNames))
		}
		configs[t.name] = cfg
	}

	staleErrs := make([]error, len(workerNames))
	var staleWg sync.WaitGroup
	for i, nodeName := range workerNames {
		staleWg.Add(1)
		go func(idx int, name string) {
			defer staleWg.Done()
			node, err := kubeClient.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				staleErrs[idx] = fmt.Errorf("failed to get node %s: %w", name, err)
				return
			}
			if _, hasLabel := node.Labels[nodeResourceLabelKey]; hasLabel {
				logrus.Warnf("Removing stale %s label from node %s", nodeResourceLabelKey, name)
				if err := unlabelNode(ctx, kubeClient, name); err != nil {
					staleErrs[idx] = fmt.Errorf("failed to remove stale label from node %s: %w", name, err)
				}
			}
		}(i, nodeName)
	}
	staleWg.Wait()
	for _, err := range staleErrs {
		if err != nil {
			return nil, err
		}
	}

	nrs := &nodeResourceScheduler{
		kubeClient:   kubeClient,
		workerNodes:  workerNames,
		reservedBy:   make(map[string]string),
		tests:        tests,
		configs:      configs,
		lastProgress: time.Now(),
	}
	nrs.cond = sync.NewCond(&nrs.mu)

	logrus.Infof("NodeResource scheduler initialized: %d tests, %d worker nodes available", len(tests), len(workerNames))
	return nrs, nil
}

func (nrs *nodeResourceScheduler) GetNextTestToRun(ctx context.Context) *testCase {
	nrs.mu.Lock()
	defer nrs.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return nil
		}
		if len(nrs.tests) == 0 {
			return nil
		}

		freeNodes := nrs.getReadyFreeNodesLocked(ctx)

		for i, test := range nrs.tests {
			cfg := nrs.configs[test.name]

			// Skip if this label is already reserved by a running test
			if nrs.isLabelReservedLocked(cfg.label) {
				continue
			}

			needed := cfg.numNodes
			if cfg.isAll {
				needed = len(nrs.workerNodes)
				if len(freeNodes) != len(nrs.workerNodes) {
					continue
				}
			}

			if len(freeNodes) < needed {
				continue
			}

			reserved := freeNodes[:needed]

			if err := labelNodes(ctx, nrs.kubeClient, reserved, cfg.label); err != nil {
				logrus.Errorf("Failed to label nodes for test %s: %v, marking test failed", test.name, err)
				test.failed = true
				test.testOutputBytes = []byte(fmt.Sprintf("NodeResource scheduler failed to label nodes: %v", err))
				nrs.tests = append(nrs.tests[:i], nrs.tests[i+1:]...)
				nrs.lastProgress = time.Now()
				return test
			}

			for _, nodeName := range reserved {
				nrs.reservedBy[nodeName] = cfg.label
			}

			nrs.tests = append(nrs.tests[:i], nrs.tests[i+1:]...)
			nrs.lastProgress = time.Now()
			return test
		}

		// Tests holding pool nodes are in flight; waiting is expected.
		if len(nrs.reservedBy) > 0 {
			nrs.cond.Wait()
			continue
		}

		if stalled := time.Since(nrs.lastProgress); stalled > nodeResourceNoProgressDeadline {
			logrus.Errorf("NodeResource scheduler made no progress for %s with no tests in flight; failing remaining %d test(s) instead of hanging the suite", stalled.Round(time.Second), len(nrs.tests))
			markTestsFailed(nrs.tests, fmt.Sprintf(
				"NodeResource scheduler made no progress for %s while no test was running: no pool node became free/Ready to satisfy the next queued test (a node may be stuck NotReady). Failing remaining test(s) instead of hanging the suite.",
				stalled.Round(time.Second)))
			nrs.tests = nil
			nrs.cond.Broadcast()
			return nil
		}

		nrs.cond.Wait()
	}
}

func (nrs *nodeResourceScheduler) isLabelReservedLocked(label string) bool {
	for _, reservedLabel := range nrs.reservedBy {
		if reservedLabel == label {
			return true
		}
	}
	return false
}

func (nrs *nodeResourceScheduler) MarkTestComplete(test *testCase) {
	nrs.mu.Lock()

	cfg := nrs.configs[test.name]

	var nodesToRelease []string
	for nodeName, reservedLabel := range nrs.reservedBy {
		if reservedLabel == cfg.label {
			nodesToRelease = append(nodesToRelease, nodeName)
		}
	}

	for _, nodeName := range nodesToRelease {
		delete(nrs.reservedBy, nodeName)
	}

	nrs.lastProgress = time.Now()
	nrs.cond.Broadcast()
	nrs.mu.Unlock()

	// Unlabel outside the lock so slow API calls do not block dispatch.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), nodeResourceUnlabelTimeout)
	defer cancel()
	for _, nodeName := range nodesToRelease {
		if err := unlabelNode(cleanupCtx, nrs.kubeClient, nodeName); err != nil {
			logrus.Errorf("Failed to remove NodeResource label from node %s after test %s: %v", nodeName, test.name, err)
		}
	}
}

func (nrs *nodeResourceScheduler) getFreeNodesLocked() []string {
	var free []string
	for _, name := range nrs.workerNodes {
		if _, reserved := nrs.reservedBy[name]; !reserved {
			free = append(free, name)
		}
	}
	return free
}

func (nrs *nodeResourceScheduler) getReadyFreeNodesLocked(ctx context.Context) []string {
	free := nrs.getFreeNodesLocked()
	var ready []string
	for _, name := range free {
		node, err := nrs.kubeClient.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			logrus.Warnf("Failed to check readiness of node %s, skipping: %v", name, err)
			continue
		}
		if isNodeReady(node) {
			ready = append(ready, name)
		}
	}
	return ready
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func labelNodes(ctx context.Context, kubeClient kubernetes.Interface, nodeNames []string, label string) error {
	errs := make([]error, len(nodeNames))
	var wg sync.WaitGroup
	for i, nodeName := range nodeNames {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:%q}}}`, nodeResourceLabelKey, label))
			_, errs[idx] = kubeClient.CoreV1().Nodes().Patch(ctx, name, types.MergePatchType, patchData, metav1.PatchOptions{})
		}(i, nodeName)
	}
	wg.Wait()

	var firstErr error
	var successNodes []string
	for i, err := range errs {
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to label node %s: %w", nodeNames[i], err)
		}
		if err == nil {
			successNodes = append(successNodes, nodeNames[i])
		}
	}
	if firstErr != nil {
		var rollbackWg sync.WaitGroup
		for _, name := range successNodes {
			rollbackWg.Add(1)
			go func(n string) {
				defer rollbackWg.Done()
				if rollbackErr := unlabelNode(ctx, kubeClient, n); rollbackErr != nil {
					logrus.Errorf("Failed to rollback label on node %s during cleanup: %v", n, rollbackErr)
				}
			}(name)
		}
		rollbackWg.Wait()
		return firstErr
	}
	return nil
}

func unlabelNode(ctx context.Context, kubeClient kubernetes.Interface, nodeName string) error {
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, nodeResourceLabelKey))
	_, err := kubeClient.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	return err
}

func splitNodeResourceTests(tests []*testCase) (allNodeTests, singleNodeTests []*testCase) {
	for _, t := range tests {
		cfg, err := parseNodeResourceTag(t.name)
		if err != nil || !cfg.isAll {
			singleNodeTests = append(singleNodeTests, t)
			continue
		}
		allNodeTests = append(allNodeTests, t)
	}
	return allNodeTests, singleNodeTests
}

func runNodeResourceSchedulerPhase(
	ctx context.Context,
	tests []*testCase,
	parallelism int,
	kubeClient kubernetes.Interface,
	poolNodeNames []string,
	commandContext *commandContext,
	testOutput testOutputConfig,
	maybeAbortOnFailureFn testAbortFunc,
) {
	if len(tests) == 0 {
		return
	}
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > len(tests) {
		parallelism = len(tests)
	}

	scheduler, err := newNodeResourceScheduler(ctx, kubeClient, tests, poolNodeNames)
	if err != nil {
		logrus.Errorf("Failed to initialize NodeResource scheduler: %v", err)
		markTestsFailed(tests, fmt.Sprintf("NodeResource scheduler initialization failed: %v", err))
		return
	}

	testSuiteProgress := newTestSuiteProgress(len(tests))
	testSuiteRunner := &testSuiteRunnerImpl{
		commandContext:        commandContext,
		testOutput:            testOutput,
		testSuiteProgress:     testSuiteProgress,
		maybeAbortOnFailureFn: maybeAbortOnFailureFn,
	}

	logrus.Infof("Starting NodeResource phase: %d test(s), %d pool node(s), parallelism=%d",
		len(tests), len(poolNodeNames), parallelism)

	go func() {
		<-ctx.Done()
		scheduler.mu.Lock()
		scheduler.cond.Broadcast()
		scheduler.mu.Unlock()
	}()

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scheduler.cond.Broadcast()
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runTestsUntilDone(ctx, scheduler, testSuiteRunner)
		}()
	}
	wg.Wait()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), nodeResourceUnlabelTimeout)
	defer cancel()
	scheduler.mu.Lock()
	var orphaned []string
	for nodeName := range scheduler.reservedBy {
		orphaned = append(orphaned, nodeName)
	}
	scheduler.mu.Unlock()
	for _, nodeName := range orphaned {
		logrus.Warnf("Cleaning up orphaned NodeResource label on node %s", nodeName)
		if err := unlabelNode(cleanupCtx, scheduler.kubeClient, nodeName); err != nil {
			logrus.Errorf("Failed to clean up orphaned NodeResource label on node %s: %v", nodeName, err)
		}
	}
}

func executeNodeResourceTests(
	ctx context.Context,
	tests []*testCase,
	commandContext *commandContext,
	testOutput testOutputConfig,
	maybeAbortOnFailureFn testAbortFunc,
	restConfig *rest.Config,
) {
	if len(tests) == 0 {
		return
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logrus.Errorf("Failed to create kubernetes client for NodeResource tests: %v", err)
		markTestsFailed(tests, fmt.Sprintf("NodeResource scheduler initialization failed: %v", err))
		return
	}

	// Provision a dedicated worker pool once for this bucket; original cluster
	// workers stay free for the rest of the suite.
	pool, err := createStaticNodeResourcePool(ctx, restConfig, kubeClient, nodeResourcePoolSize())
	if pool != nil {
		defer teardownNodeResourcePool(context.Background(), restConfig, pool)
	}
	if err != nil {
		if errors.Is(err, errMachineAPIUnavailable) {
			// e.g. Single Node OpenShift — no Machine API to clone from.
			logrus.Infof("Machine API is not available on this cluster; skipping %d NodeResource test(s)", len(tests))
			markTestsSkipped(tests, "Machine API is not available on this cluster; NodeResource tests require a dedicated worker node pool")
			return
		}
		logrus.Errorf("Failed to provision dedicated NodeResource worker pool: %v", err)
		markTestsFailed(tests, fmt.Sprintf("Failed to provision dedicated NodeResource worker pool: %v", err))
		return
	}

	allNodeTests, singleNodeTests := splitNodeResourceTests(tests)

	logrus.Infof("NodeResource bucket: %d total test(s) (%d numNodes=all, %d single-node) on %d pool node(s)",
		len(tests), len(allNodeTests), len(singleNodeTests), len(pool.nodeNames))

	// Phase 1: numNodes=all tests need every pool node — run one at a time.
	runNodeResourceSchedulerPhase(ctx, allNodeTests, 1, kubeClient, pool.nodeNames,
		commandContext, testOutput, maybeAbortOnFailureFn)

	// Phase 2: single-node tests can run in parallel across the pool.
	runNodeResourceSchedulerPhase(ctx, singleNodeTests, len(pool.nodeNames), kubeClient, pool.nodeNames,
		commandContext, testOutput, maybeAbortOnFailureFn)
}

func isNodeResourceTest(test *testCase) bool {
	return strings.Contains(test.name, "[NodeResource:")
}

// markTestsFailed marks tests failed when pool or scheduler setup fails.
func markTestsFailed(tests []*testCase, reason string) {
	now := time.Now()
	for _, test := range tests {
		test.start = now
		test.end = now
		test.flake = false
		test.failed = true
		test.skipped = false
		test.success = false
		test.timedOut = false
		test.testOutputBytes = []byte(reason)
	}
}

// markTestsSkipped marks tests skipped when the pool cannot be provisioned.
func markTestsSkipped(tests []*testCase, reason string) {
	now := time.Now()
	for _, test := range tests {
		test.start = now
		test.end = now
		test.flake = false
		test.failed = false
		test.skipped = true
		test.success = false
		test.timedOut = false
		test.testOutputBytes = []byte(reason)
	}
}
