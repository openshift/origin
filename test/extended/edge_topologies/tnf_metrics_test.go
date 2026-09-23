package edge_topologies

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	etcdv1 "github.com/openshift/api/etcd/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestParseTNFGaugesReturnsGaugeSamplesAndExcludesCounters(t *testing.T) {
	exposition := `
# HELP tnf_cluster_healthy Whether the TNF cluster is healthy.
# TYPE tnf_cluster_healthy gauge
tnf_cluster_healthy 1
# HELP tnf_node_healthy Whether a TNF node is healthy.
# TYPE tnf_node_healthy gauge
tnf_node_healthy{node="master-0"} 0
# HELP tnf_resource_managed Whether a TNF resource is managed.
# TYPE tnf_resource_managed gauge
tnf_resource_managed{node="master-0",resource="Etcd"} 1
# HELP tnf_resource_disruption_total TNF resource disruptions.
# TYPE tnf_resource_disruption_total counter
tnf_resource_disruption_total{node="master-0",resource="Etcd"} 4
`

	got, err := parseTNFGauges(exposition)
	if err != nil {
		t.Fatalf("parseTNFGauges() returned an error: %v", err)
	}

	want := map[metricKey]float64{
		{name: "tnf_cluster_healthy"}:                                      1,
		{name: "tnf_node_healthy", node: "master-0"}:                       0,
		{name: "tnf_resource_managed", node: "master-0", resource: "Etcd"}: 1,
	}
	if len(got) != len(want) {
		t.Fatalf("parseTNFGauges() returned %d samples, want %d: %v", len(got), len(want), got)
	}
	for key, wantValue := range want {
		if gotValue, ok := got[key]; !ok || gotValue != wantValue {
			t.Errorf("parseTNFGauges()[%+v] = %v, %t; want %v, true", key, gotValue, ok, wantValue)
		}
	}
}

func TestParseTNFGaugesRejectsMalformedExposition(t *testing.T) {
	if _, err := parseTNFGauges("tnf_cluster_healthy not-a-number\n"); err == nil {
		t.Fatal("parseTNFGauges() returned nil error for malformed exposition")
	}
}

func TestExpectedHealthyTNFGaugesContainsAll53Samples(t *testing.T) {
	got := expectedHealthyTNFGauges([]string{"master-0", "master-1"})
	if len(got) != 53 {
		t.Fatalf("expectedHealthyTNFGauges() returned %d samples, want 53", len(got))
	}
	for key, value := range got {
		if value != 1 {
			t.Errorf("expectedHealthyTNFGauges()[%+v] = %v, want 1", key, value)
		}
	}
}

func TestDisruptionExpectationsMatchValidatedJourneys(t *testing.T) {
	nodes := []string{"master-0", "master-1"}
	tests := []struct {
		name string
		got  map[metricKey]float64
		want int
	}{
		{name: "cluster maintenance", got: clusterMaintenanceTNFGauges(nodes), want: 16},
		{name: "resource unmanage", got: resourceUnmanagedTNFGauges(nodes), want: 4},
		{name: "node maintenance", got: nodeMaintenanceTNFGauges("master-1"), want: 9},
		{name: "fence disabled", got: fenceDisabledTNFGauges("master-0"), want: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.got) != test.want {
				t.Fatalf("expectation contains %d samples, want %d: %v", len(test.got), test.want, test.got)
			}
			for key, value := range test.got {
				if value != 0 {
					t.Errorf("expectation[%+v] = %v, want 0", key, value)
				}
			}
		})
	}
}

func TestParseFiringTNFAlerts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "no alerts", input: `{"status":"success","data":{"resultType":"vector","result":[]}}`},
		{name: "firing alert", input: `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"alertname":"TNFClusterInMaintenance"},"value":[1,"1"]}]}}`, want: []string{"TNFClusterInMaintenance"}},
		{name: "failed query", input: `{"status":"error","error":"bad query"}`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseFiringTNFAlerts(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("parseFiringTNFAlerts() returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFiringTNFAlerts() returned an error: %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("parseFiringTNFAlerts() = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("parseFiringTNFAlerts()[%d] = %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}

func TestTNFGaugeMismatchesReportsMissingAndUnexpectedValues(t *testing.T) {
	expected := map[metricKey]float64{
		{name: "tnf_cluster_healthy"}:                1,
		{name: "tnf_node_healthy", node: "master-0"}: 1,
	}
	actual := map[metricKey]float64{{name: "tnf_cluster_healthy"}: 0}

	got := tnfGaugeMismatches(actual, expected)
	if len(got) != 2 {
		t.Fatalf("tnfGaugeMismatches() returned %d mismatches, want 2: %v", len(got), got)
	}
}

func TestParseLoadedTNFAlertRules(t *testing.T) {
	response := `{"status":"success","data":{"groups":[{"name":"tnf.rules","rules":[{"name":"TNFClusterInMaintenance"},{"name":"etcdNoLeader"},{"name":"TNFNodeInMaintenance"}]}]}}`

	got, err := parseLoadedTNFAlertRules(response)
	if err != nil {
		t.Fatalf("parseLoadedTNFAlertRules() returned an error: %v", err)
	}
	want := []string{"TNFClusterInMaintenance", "TNFNodeInMaintenance"}
	if len(got) != len(want) {
		t.Fatalf("parseLoadedTNFAlertRules() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("parseLoadedTNFAlertRules()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTNFGaugesWithOverridesKeepsCompleteHealthyContract(t *testing.T) {
	nodes := []string{"master-0", "master-1"}
	overrides := resourceUnmanagedTNFGauges(nodes)

	got := tnfGaugesWithOverrides(expectedHealthyTNFGauges(nodes), overrides)
	if len(got) != 53 {
		t.Fatalf("tnfGaugesWithOverrides() returned %d samples, want 53", len(got))
	}
	for key, value := range got {
		want := float64(1)
		if override, found := overrides[key]; found {
			want = override
		}
		if value != want {
			t.Errorf("tnfGaugesWithOverrides()[%+v] = %v, want %v", key, value, want)
		}
	}
}

func TestRetryTNFOperationRetriesTransientFailures(t *testing.T) {
	attempts := 0
	err := retryTNFOperation(context.Background(), time.Millisecond, 100*time.Millisecond, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryTNFOperation() returned an error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("retryTNFOperation() made %d attempts, want 3", attempts)
	}
}

func TestWaitForTNFCommandKillsAndReapsProcessOnTimeout(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := waitForTNFCommand(ctx, cmd)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForTNFCommand() error = %v, want context deadline exceeded", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("waitForTNFCommand() returned before reaping the timed-out process")
	}
}

func TestTNFPCSRunnerBlocksNextCommandUntilRemoteCleanupCompletes(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "debug-test"}})
	// A successful Delete request is not proof that the namespace's remote
	// pods have terminated. Model the namespace controller still cleaning up.
	client.PrependReactor("delete", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	runner := &tnfPCSRunner{namespaces: client.CoreV1().Namespaces(), pendingNamespace: "debug-test"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := runner.run(ctx, "master-0", "property", "set", "maintenance-mode=false"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run() error = %v, want deadline exceeded while waiting for remote cleanup", err)
	}
	if runner.pendingNamespace != "debug-test" {
		t.Fatal("lost the namespace needed to retry cleanup")
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "create" {
			t.Fatal("started a new debug namespace before the previous one disappeared")
		}
	}

	// Once remote cleanup has completed, a fresh context can clear the barrier.
	if err := client.Tracker().Delete(corev1.SchemeGroupVersion.WithResource("namespaces"), "", "debug-test"); err != nil {
		t.Fatal(err)
	}
	if err := runner.cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup after namespace deletion: %v", err)
	}
	if runner.pendingNamespace != "" {
		t.Fatal("completed remote cleanup did not clear the pending namespace")
	}
}

func TestTNFPCSRunnerRetainsCleanupOnAPIError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("delete", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("API unavailable")
	})
	runner := &tnfPCSRunner{namespaces: client.CoreV1().Namespaces(), pendingNamespace: "debug-test"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := runner.cleanup(ctx); err == nil {
		t.Fatal("cleanup succeeded despite an unavailable API")
	}
	if runner.pendingNamespace != "debug-test" {
		t.Fatal("lost pending remote cleanup after API failure")
	}
}

func TestSingleTNFFencingAgentPrerequisite(t *testing.T) {
	started := etcdv1.PacemakerClusterFencingAgentStatus{
		Name:       "master-0_redfish",
		Conditions: []metav1.Condition{{Type: string(etcdv1.ResourceStartedConditionType), Status: metav1.ConditionTrue}},
	}
	stopped := etcdv1.PacemakerClusterFencingAgentStatus{Name: "master-0_backup"}
	backupStarted := started
	backupStarted.Name = "master-0_backup"
	for _, tc := range []struct {
		name    string
		agents  []etcdv1.PacemakerClusterFencingAgentStatus
		wantErr bool
	}{
		{name: "single started agent", agents: []etcdv1.PacemakerClusterFencingAgentStatus{started}},
		{name: "no agents", wantErr: true},
		{name: "single stopped agent", agents: []etcdv1.PacemakerClusterFencingAgentStatus{stopped}, wantErr: true},
		{name: "multiple started agents", agents: []etcdv1.PacemakerClusterFencingAgentStatus{started, backupStarted}, wantErr: true},
		{name: "additional configured agent", agents: []etcdv1.PacemakerClusterFencingAgentStatus{started, stopped}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pc := &etcdv1.PacemakerCluster{}
			pc.Status.Nodes = &[]etcdv1.PacemakerClusterNodeStatus{{NodeName: "master-0", FencingAgents: tc.agents}}
			got, err := singleTNFFencingAgent(pc, "master-0")
			if (err != nil) != tc.wantErr {
				t.Fatalf("singleTNFFencingAgent() error = %v, wantErr %t", err, tc.wantErr)
			}
			if !tc.wantErr && got != "master-0_redfish" {
				t.Fatalf("selected agent = %q, want master-0_redfish", got)
			}
		})
	}
	if _, err := singleTNFFencingAgent(&etcdv1.PacemakerCluster{}, "master-0"); err == nil {
		t.Fatal("accepted missing node status")
	}
}
