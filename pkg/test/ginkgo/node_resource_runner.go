package ginkgo

import (
	"context"
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
}

func newNodeResourceScheduler(ctx context.Context, kubeClient kubernetes.Interface, tests []*testCase) (*nodeResourceScheduler, error) {
	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list worker nodes: %w", err)
	}

	var workerNames []string
	for _, node := range nodeList.Items {
		if _, has := node.Labels["node-role.kubernetes.io/control-plane"]; has {
			continue
		}
		if _, has := node.Labels["node-role.kubernetes.io/master"]; has {
			continue
		}
		workerNames = append(workerNames, node.Name)
	}
	sort.Strings(workerNames)

	if len(workerNames) == 0 {
		return nil, fmt.Errorf("no pure worker nodes available for NodeResource tests")
	}

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
		kubeClient:  kubeClient,
		workerNodes: workerNames,
		reservedBy:  make(map[string]string),
		tests:       tests,
		configs:     configs,
	}
	nrs.cond = sync.NewCond(&nrs.mu)

	logrus.Infof("NodeResource scheduler initialized: %d tests, %d worker nodes available", len(tests), len(workerNames))
	return nrs, nil
}

func (nrs *nodeResourceScheduler) GetNextTestToRun(ctx context.Context) *testCase {
	nrs.mu.Lock()
	defer nrs.mu.Unlock()

	// Watch for context cancellation to wake blocked workers
	go func() {
		<-ctx.Done()
		nrs.cond.Broadcast()
	}()

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
				return test
			}

			for _, nodeName := range reserved {
				nrs.reservedBy[nodeName] = cfg.label
			}

			nrs.tests = append(nrs.tests[:i], nrs.tests[i+1:]...)
			return test
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
	defer nrs.mu.Unlock()

	cfg := nrs.configs[test.name]

	var nodesToRelease []string
	for nodeName, reservedLabel := range nrs.reservedBy {
		if reservedLabel == cfg.label {
			nodesToRelease = append(nodesToRelease, nodeName)
		}
	}

	for _, nodeName := range nodesToRelease {
		if err := unlabelNode(context.Background(), nrs.kubeClient, nodeName); err != nil {
			logrus.Errorf("Failed to remove NodeResource label from node %s after test %s: %v", nodeName, test.name, err)
		}
		delete(nrs.reservedBy, nodeName)
	}

	nrs.cond.Broadcast()
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
		for _, test := range tests {
			test.failed = true
			test.testOutputBytes = []byte(fmt.Sprintf("NodeResource scheduler initialization failed: %v", err))
		}
		return
	}

	scheduler, err := newNodeResourceScheduler(ctx, kubeClient, tests)
	if err != nil {
		logrus.Errorf("Failed to initialize NodeResource scheduler: %v", err)
		for _, test := range tests {
			test.failed = true
			test.testOutputBytes = []byte(fmt.Sprintf("NodeResource scheduler initialization failed: %v", err))
		}
		return
	}

	parallelism := len(scheduler.workerNodes)
	if parallelism > len(tests) {
		parallelism = len(tests)
	}

	testSuiteProgress := newTestSuiteProgress(len(tests))
	testSuiteRunner := &testSuiteRunnerImpl{
		commandContext:        commandContext,
		testOutput:            testOutput,
		testSuiteProgress:     testSuiteProgress,
		maybeAbortOnFailureFn: maybeAbortOnFailureFn,
	}

	logrus.Infof("Starting NodeResource test execution: %d tests, %d workers, parallelism=%d",
		len(tests), len(scheduler.workerNodes), parallelism)

	// Periodic wakeup so blocked workers re-check node readiness
	// after disruptive tests leave nodes temporarily NotReady.
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

	cleanupCtx := context.Background()
	for nodeName := range scheduler.reservedBy {
		logrus.Warnf("Cleaning up orphaned NodeResource label on node %s", nodeName)
		if err := unlabelNode(cleanupCtx, scheduler.kubeClient, nodeName); err != nil {
			logrus.Errorf("Failed to clean up orphaned NodeResource label on node %s: %v", nodeName, err)
		}
	}
}

func isNodeResourceTest(test *testCase) bool {
	return strings.Contains(test.name, "[NodeResource:")
}
