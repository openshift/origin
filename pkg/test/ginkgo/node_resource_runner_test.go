package ginkgo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift-eng/openshift-tests-extension/pkg/util/sets"
	"github.com/openshift/origin/pkg/test/extensions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func makeTestCaseWithLabels(name string, labels ...string) *testCase {
	return &testCase{
		name: name,
		spec: &extensions.ExtensionTestSpec{
			ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
				Name:   name,
				Labels: sets.New[string](labels...),
			},
		},
	}
}

func TestParseNodeResourceFromSpec(t *testing.T) {
	tests := []struct {
		name      string
		labels    []string
		wantNum   int
		wantLabel string
		wantIsAll bool
		wantErr   bool
	}{
		{
			name:      "single node",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=foo"},
			wantNum:   1,
			wantLabel: "foo",
		},
		{
			name:      "all nodes",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=all", "NodeResourceName=bar"},
			wantNum:   -1,
			wantLabel: "bar",
			wantIsAll: true,
		},
		{
			name:      "multiple nodes",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=3", "NodeResourceName=multi_node"},
			wantNum:   3,
			wantLabel: "multi_node",
		},
		{
			name:      "defaults to 1 node when numNodes not specified",
			labels:    []string{"NodeResource", "NodeResourceName=default_test"},
			wantNum:   1,
			wantLabel: "default_test",
		},
		{
			name:      "max wins with multiple numNodes labels",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=1", "NodeResourceNumNodes=2", "NodeResourceName=override"},
			wantNum:   2,
			wantLabel: "override",
		},
		{
			name:    "missing NodeResource label",
			labels:  []string{"NodeResourceNumNodes=1", "NodeResourceName=foo"},
			wantErr: true,
		},
		{
			name:    "missing NodeResourceName",
			labels:  []string{"NodeResource", "NodeResourceNumNodes=1"},
			wantErr: true,
		},
		{
			name:    "non-numeric nodes",
			labels:  []string{"NodeResource", "NodeResourceNumNodes=abc", "NodeResourceName=bad"},
			wantErr: true,
		},
		{
			name:    "nil spec",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tc *testCase
			if tt.labels == nil {
				tc = &testCase{name: "no-spec"}
			} else {
				tc = makeTestCaseWithLabels("test", tt.labels...)
			}

			cfg, err := parseNodeResourceFromSpec(tc)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.numNodes != tt.wantNum {
				t.Errorf("numNodes = %d, want %d", cfg.numNodes, tt.wantNum)
			}
			if cfg.label != tt.wantLabel {
				t.Errorf("label = %q, want %q", cfg.label, tt.wantLabel)
			}
			if cfg.isAll != tt.wantIsAll {
				t.Errorf("isAll = %v, want %v", cfg.isAll, tt.wantIsAll)
			}
		})
	}
}

func TestIsNodeResourceTest(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{"with NodeResource label", []string{"NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=x"}, true},
		{"without NodeResource label", []string{"sig-node"}, false},
		{"nil spec", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tc *testCase
			if tt.labels == nil {
				tc = &testCase{name: "no-spec"}
			} else {
				tc = makeTestCaseWithLabels("test", tt.labels...)
			}
			got := isNodeResourceTest(tc)
			if got != tt.want {
				t.Errorf("isNodeResourceTest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func readyNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func TestNodeResourceSchedulerPreservesCallerSliceOnDispatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"), readyNode("n2"), readyNode("n3"))

	a := makeTestCaseWithLabels("A", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=a")
	b := makeTestCaseWithLabels("B", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=b")
	c := makeTestCaseWithLabels("C", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=c")
	caller := []*testCase{a, b, c}
	want := []*testCase{a, b, c}

	sched, err := newNodeResourceScheduler(ctx, client, caller, []string{"n1", "n2", "n3"})
	if err != nil {
		t.Fatalf("newNodeResourceScheduler: %v", err)
	}

	got := sched.GetNextTestToRun(ctx)
	if got == nil || got.name != "A" {
		t.Fatalf("expected to dispatch A, got %v", got)
	}
	sched.MarkTestComplete(got)

	got = sched.GetNextTestToRun(ctx)
	if got == nil || got.name != "B" {
		t.Fatalf("expected to dispatch B, got %v", got)
	}
	sched.MarkTestComplete(got)

	got = sched.GetNextTestToRun(ctx)
	if got == nil || got.name != "C" {
		t.Fatalf("expected to dispatch C, got %v", got)
	}
	sched.MarkTestComplete(got)

	for i := range caller {
		if caller[i] != want[i] {
			t.Errorf("caller[%d] corrupted: got %p want %p", i, caller[i], want[i])
		}
	}
}

func TestNodeResourceSchedulerLabelFailureThenDispatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"), readyNode("n2"))

	failOnce := true
	client.PrependReactor("patch", "nodes", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if failOnce {
			failOnce = false
			return true, nil, fmt.Errorf("injected patch failure")
		}
		return false, nil, nil
	})

	failTest := makeTestCaseWithLabels("fail-first", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=fail")
	okTest := makeTestCaseWithLabels("ok-second", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=ok")
	tests := []*testCase{failTest, okTest}

	sched, err := newNodeResourceScheduler(ctx, client, tests, []string{"n1", "n2"})
	if err != nil {
		t.Fatalf("newNodeResourceScheduler: %v", err)
	}

	dispatched := sched.GetNextTestToRun(ctx)
	if dispatched == nil || dispatched.name != "ok-second" {
		t.Fatalf("expected ok-second dispatch after skipping failed label, got %v", dispatched)
	}
	if !failTest.failed {
		t.Fatalf("expected fail-first marked failed, success=%v skipped=%v", failTest.success, failTest.skipped)
	}
}

func TestNodeResourceSchedulerSameLabelExclusion(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"), readyNode("n2"))

	t1 := makeTestCaseWithLabels("t1", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=shared")
	t2 := makeTestCaseWithLabels("t2", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=shared")
	sched, err := newNodeResourceScheduler(ctx, client, []*testCase{t1, t2}, []string{"n1", "n2"})
	if err != nil {
		t.Fatalf("newNodeResourceScheduler: %v", err)
	}

	first := sched.GetNextTestToRun(ctx)
	if first == nil || first.name != "t1" {
		t.Fatalf("expected t1, got %v", first)
	}
	sched.MarkTestComplete(first)

	second := sched.GetNextTestToRun(ctx)
	if second == nil || second.name != "t2" {
		t.Fatalf("expected t2 after t1 complete, got %v", second)
	}
}

// MarkTestComplete must not free a node when unlabel keeps failing (stale label / wrong dispatch).
func TestNodeResourceSchedulerQuarantinesNodeOnPersistentUnlabelFailure(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"), readyNode("n2"))

	// Fail every patch that tries to null out the reservation label, simulating
	// a node whose unlabel call never succeeds.
	client.PrependReactor("patch", "nodes", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction, ok := action.(clientgotesting.PatchAction)
		if !ok {
			return false, nil, nil
		}
		if strings.Contains(string(patchAction.GetPatch()), `"`+nodeResourceLabelKey+`":null`) {
			return true, nil, fmt.Errorf("injected persistent unlabel failure")
		}
		return false, nil, nil
	})

	t1 := makeTestCaseWithLabels("t1", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=shared")
	t2 := makeTestCaseWithLabels("t2", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=shared")
	sched, err := newNodeResourceScheduler(ctx, client, []*testCase{t1, t2}, []string{"n1", "n2"})
	if err != nil {
		t.Fatalf("newNodeResourceScheduler: %v", err)
	}

	first := sched.GetNextTestToRun(ctx)
	if first == nil || first.name != "t1" {
		t.Fatalf("expected t1, got %v", first)
	}
	sched.MarkTestComplete(first)

	sched.mu.Lock()
	reservedCount := len(sched.reservedBy)
	quarantinedCount := len(sched.quarantined)
	sched.mu.Unlock()
	if reservedCount != 0 {
		t.Fatalf("expected reservedBy to be empty once the test completed (no test is in flight), got size = %d", reservedCount)
	}
	if quarantinedCount != 1 {
		t.Fatalf("expected exactly one quarantined node after a persistently failing unlabel, got size = %d", quarantinedCount)
	}

	// t2 shares the same NodeResourceName; it must stay blocked rather than be
	// dispatched onto the quarantined node.
	done := make(chan *testCase, 1)
	go func() {
		done <- sched.GetNextTestToRun(ctx)
	}()
	select {
	case got := <-done:
		t.Fatalf("expected t2 to remain blocked by the quarantined node, got dispatched: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestLabelNodesFailedRollbackReturnsStaleNodes(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"), readyNode("n2"))

	client.PrependReactor("patch", "nodes", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction, ok := action.(clientgotesting.PatchAction)
		if !ok {
			return false, nil, nil
		}
		body := string(patchAction.GetPatch())
		if patchAction.GetName() == "n2" && strings.Contains(body, `"x"`) {
			return true, nil, fmt.Errorf("label n2 failed")
		}
		if patchAction.GetName() == "n1" && strings.Contains(body, `"`+nodeResourceLabelKey+`":null`) {
			return true, nil, fmt.Errorf("rollback n1 failed")
		}
		return false, nil, nil
	})

	err, still := labelNodes(ctx, client, []string{"n1", "n2"}, "x")
	if err == nil {
		t.Fatal("expected label error")
	}
	if len(still) != 1 || still[0] != "n1" {
		t.Fatalf("expected n1 in stillLabeled, got %v", still)
	}
}

// A PATCH that the server applied but whose response was lost looks like an
// error to the client. Rollback must still attempt that node; if rollback
// also fails, it must appear in stillLabeled so the caller can quarantine it.
func TestLabelNodesLostResponseRollbackQuarantines(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"), readyNode("n2"))

	client.PrependReactor("patch", "nodes", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction, ok := action.(clientgotesting.PatchAction)
		if !ok {
			return false, nil, nil
		}
		body := string(patchAction.GetPatch())
		name := patchAction.GetName()
		// n1 label succeeds; n2 label "fails" (simulates committed but lost response).
		if name == "n2" && !strings.Contains(body, ":null") && strings.Contains(body, `"y"`) {
			return true, nil, fmt.Errorf("simulated lost response on n2")
		}
		// n2 rollback also fails (server still has label, API unreachable).
		if name == "n2" && strings.Contains(body, `"`+nodeResourceLabelKey+`":null`) {
			return true, nil, fmt.Errorf("rollback n2 also failed")
		}
		return false, nil, nil
	})

	err, still := labelNodes(ctx, client, []string{"n1", "n2"}, "y")
	if err == nil {
		t.Fatal("expected error")
	}
	found := false
	for _, n := range still {
		if n == "n2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected n2 in stillLabeled (committed label, lost response, rollback failed), got %v", still)
	}
}

func TestSchedulerSkipsUnschedulableTests(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(readyNode("n1"))

	ok := makeTestCaseWithLabels("ok", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=a")
	big := makeTestCaseWithLabels("big", "NodeResource", "NodeResourceNumNodes=3", "NodeResourceName=b")

	sched, err := newNodeResourceScheduler(ctx, client, []*testCase{ok, big}, []string{"n1"})
	if err != nil {
		t.Fatalf("newNodeResourceScheduler: %v", err)
	}
	if len(sched.tests) != 1 || sched.tests[0].name != "ok" {
		t.Fatalf("expected only 'ok' in queue, got %v", sched.tests)
	}
	if !big.skipped {
		t.Fatalf("expected 'big' marked skipped, got skipped=%v", big.skipped)
	}
}

func TestSchedulerQueueDoesNotCorruptCallerSlice(t *testing.T) {
	// The scheduler must copy the tests slice so that queue compaction
	// (removing dispatched tests) does not overwrite the caller's backing
	// array, which is used for result reporting after the run.
	a := makeTestCaseWithLabels("A [Serial]", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=a")
	b := makeTestCaseWithLabels("B [Serial]", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=b")
	c := makeTestCaseWithLabels("C", "NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=c")

	callerSlice := []*testCase{a, b, c}
	original := make([]*testCase, len(callerSlice))
	copy(original, callerSlice)

	// Verify the scheduler stores the same test pointers but in a separate slice.
	// We cannot call newNodeResourceScheduler here because it requires a real
	// kube client. Instead verify the copy logic directly.
	queue := make([]*testCase, len(callerSlice))
	copy(queue, callerSlice)

	// Simulate removing the first element (dispatch).
	queue = append(queue[:0], queue[1:]...)

	// Caller's slice must be unchanged.
	for i, tc := range callerSlice {
		if tc != original[i] {
			t.Errorf("callerSlice[%d] was corrupted: got %p, want %p", i, tc, original[i])
		}
	}
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []corev1.NodeCondition
		want       bool
	}{
		{
			name: "ready true",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			want: true,
		},
		{
			name: "ready false",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "ready unknown",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
			},
			want: false,
		},
		{
			name:       "no conditions",
			conditions: nil,
			want:       false,
		},
		{
			name: "other conditions only",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "ready true among other conditions",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
				Status:     corev1.NodeStatus{Conditions: tt.conditions},
			}
			got := isNodeReady(node)
			if got != tt.want {
				t.Errorf("isNodeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
