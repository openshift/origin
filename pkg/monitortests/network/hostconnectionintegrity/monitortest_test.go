package hostconnectionintegrity

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/monitortests/network/hostconnectionintegrity/poller"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ret, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return ret
}

func reconcileAt(node string, at time.Time) monitorapi.Interval {
	return monitorapi.NewInterval(SourceGatewayReconcile, monitorapi.Info).
		Locator(monitorapi.NewLocator().NodeFromName(node)).
		Message(monitorapi.NewMessage().HumanMessage("gateway.go:532] Reconciling gateway with updates")).
		Build(at, at)
}

func udnTeardownAt(node string, at time.Time) monitorapi.Interval {
	return monitorapi.NewInterval(SourceUDNTeardown, monitorapi.Info).
		Locator(monitorapi.NewLocator().NodeFromName(node)).
		Message(monitorapi.NewMessage().HumanMessage("Delete OVN logical entities for layer3 network controller of network x")).
		Build(at, at)
}

// failureAt models a poller episode whose failed probe got the reset/stall at trigger: the last successful
// probe was 500ms earlier and the failed probe started 100ms before the trigger.
func failureAt(node, backend string, reason poller.FailureReason, trigger time.Time) monitorapi.Interval {
	return poller.BuildInterval(node, poller.Target{Backend: backend, Name: "api-int", URL: "https://api-int:6443/readyz"}, reason,
		trigger.Add(-500*time.Millisecond), trigger.Add(-100*time.Millisecond), trigger.Add(45*time.Second), "10.0.0.4:51700", "boom")
}

// ciCases are the 11 CI runs in OCPBUGS-128289: node, trigger time and reconcile time relative to the trigger.
var ciCases = []struct {
	run     string
	node    string
	trigger string
	delta   time.Duration
}{
	{"2102702899158061056", "master-1", "2026-09-23T12:42:44.455Z", -3 * time.Millisecond},
	{"2102702891675422720", "master-2", "2026-09-23T12:56:33.380Z", -23 * time.Millisecond},
	{"2102146429756641280", "master-2", "2026-09-21T23:33:40.284Z", 20 * time.Millisecond},
	{"2102070347296673792", "master-2", "2026-09-21T18:33:25.645Z", -23 * time.Millisecond},
	{"2102013577127792640", "master-0", "2026-09-21T14:44:48.189Z", 28 * time.Millisecond},
	{"2101812059271335936", "master-2", "2026-09-21T02:16:58.144Z", 1 * time.Millisecond},
	{"2101720834971275264", "master-2", "2026-09-20T19:43:20.984Z", 22 * time.Millisecond},
	{"2101200511553245184", "master-1", "2026-09-19T09:21:40.627Z", -7 * time.Millisecond},
	{"2101200506536857600", "master-2", "2026-09-19T09:18:07.474Z", -12 * time.Millisecond},
	{"2101122488078438400", "master-2", "2026-09-19T03:48:48.742Z", 17 * time.Millisecond},
	{"2100442614082834432", "master-1", "2026-09-17T06:25:57.620Z", -6 * time.Millisecond},
}

func TestCorrelateCICases(t *testing.T) {
	for _, c := range ciCases {
		t.Run(c.run, func(t *testing.T) {
			trigger := mustTime(t, c.trigger)
			for _, reason := range []poller.FailureReason{poller.ReasonReset, poller.ReasonStalled} {
				intervals := monitorapi.Intervals{
					udnTeardownAt(c.node, trigger.Add(-400*time.Millisecond)),
					reconcileAt(c.node, trigger.Add(c.delta)),
					// an unrelated reconcile on another node and one long before on the same node.
					reconcileAt("other", trigger),
					reconcileAt(c.node, trigger.Add(-30*time.Second)),
					failureAt(c.node, poller.BackendAPIIntSelf, reason, trigger),
				}
				findings := Correlate(intervals)
				if len(findings) != 1 {
					t.Fatalf("%s: expected 1 finding, got %d", reason, len(findings))
				}
				if findings[0].Node != c.node || findings[0].Reason != string(reason) {
					t.Errorf("unexpected finding %+v", findings[0])
				}
				junits := Evaluate(append(intervals, FindingToInterval(findings[0])))
				if len(junits) != 1 || junits[0].FailureOutput == nil {
					t.Fatalf("expected a failing junit, got %+v", junits)
				}
				if !strings.Contains(junits[0].FailureOutput.Output, "established connection blackholed after gateway reconcile: node="+c.node) {
					t.Errorf("failure output lacks the stable line: %s", junits[0].FailureOutput.Output)
				}
			}
		})
	}
}

func TestCorrelateNegatives(t *testing.T) {
	trigger := mustTime(t, "2026-09-17T06:25:57.620Z")
	node := "master-1"
	shutdown := monitorapi.NewInterval(monitorapi.APIServerGracefulShutdown, monitorapi.Info).
		Locator(monitorapi.Locator{Type: monitorapi.LocatorTypeContainer, Keys: map[monitorapi.LocatorKey]string{
			monitorapi.LocatorNamespaceKey: "openshift-kube-apiserver",
			monitorapi.LocatorPodKey:       "kube-apiserver-master-0",
		}}).
		Message(monitorapi.NewMessage().Reason("GracefulAPIServerShutdown")).
		Build(trigger.Add(-20*time.Second), trigger.Add(60*time.Second))
	ovsStall := monitorapi.NewInterval(monitorapi.SourceOVSVswitchdLog, monitorapi.Warning).
		Locator(monitorapi.NewLocator().NodeFromName(node)).
		Message(monitorapi.NewMessage().HumanMessage("ovs-vswitchd[1198]: ovs|01397|timeval|WARN|Unreasonably long 12161ms poll interval (356ms user, 11593ms system)")).
		Build(trigger.Add(5*time.Second), trigger.Add(18*time.Second))

	tests := []struct {
		name      string
		intervals monitorapi.Intervals
		want      int
	}{
		{
			name: "reconcile 1s after the failed probe",
			intervals: monitorapi.Intervals{
				reconcileAt(node, trigger.Add(time.Second)),
				failureAt(node, poller.BackendAPIIntSelf, poller.ReasonReset, trigger),
			},
		},
		{
			name: "reconcile 1s before the last successful probe",
			intervals: monitorapi.Intervals{
				reconcileAt(node, trigger.Add(-1500*time.Millisecond)),
				failureAt(node, poller.BackendAPIIntSelf, poller.ReasonStalled, trigger),
			},
		},
		{
			name: "reconcile on another node",
			intervals: monitorapi.Intervals{
				reconcileAt("master-2", trigger),
				failureAt(node, poller.BackendAPIIntSelf, poller.ReasonStalled, trigger),
			},
		},
		{
			name: "outage is not an established connection failure",
			intervals: monitorapi.Intervals{
				reconcileAt(node, trigger),
				failureAt(node, poller.BackendAPIIntSelf, poller.ReasonOutage, trigger),
			},
		},
		{
			name: "new connection failure is not an established connection failure",
			intervals: monitorapi.Intervals{
				reconcileAt(node, trigger),
				failureAt(node, poller.BackendAPIIntSelf, poller.ReasonNewConnectionFailed, trigger),
			},
		},
		{
			name: "api-int failure during a kube-apiserver graceful shutdown (OCPBUGS-100298)",
			intervals: monitorapi.Intervals{
				shutdown,
				reconcileAt(node, trigger),
				failureAt(node, poller.BackendAPIIntSelf, poller.ReasonStalled, trigger),
			},
		},
		{
			name: "peer kubelet failure is still reported during a kube-apiserver shutdown",
			intervals: monitorapi.Intervals{
				shutdown,
				reconcileAt(node, trigger),
				failureAt(node, poller.BackendPeerKubelet, poller.ReasonStalled, trigger),
			},
			want: 1,
		},
		{
			name: "failure during an ovs-vswitchd stall on the same node (OCPBUGS-99645)",
			intervals: monitorapi.Intervals{
				ovsStall,
				reconcileAt(node, trigger),
				failureAt(node, poller.BackendPeerKubelet, poller.ReasonStalled, trigger),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Correlate(tt.intervals); len(got) != tt.want {
				t.Errorf("expected %d findings, got %d: %+v", tt.want, len(got), got)
			}
		})
	}
}

func TestEvaluateUDNGating(t *testing.T) {
	trigger := mustTime(t, "2026-09-17T06:25:57.620Z")
	finding := FindingToInterval(Finding{Node: "master-1", Backend: poller.BackendAPIIntSelf, Reason: "Reset",
		Onset: trigger, OnsetUpTo: trigger, Recovered: trigger.Add(45 * time.Second), Reconcile: trigger})

	junits := Evaluate(monitorapi.Intervals{finding})
	if len(junits) != 1 || junits[0].FailureOutput != nil || !strings.Contains(junits[0].SystemOut, "not evaluated") {
		t.Errorf("without UDN teardown the test must pass and say it was not evaluated: %+v", junits[0])
	}

	junits = Evaluate(monitorapi.Intervals{udnTeardownAt("master-1", trigger)})
	if len(junits) != 1 || junits[0].FailureOutput != nil {
		t.Errorf("with UDN teardown but no finding the test must pass: %+v", junits[0])
	}

	junits = Evaluate(monitorapi.Intervals{udnTeardownAt("master-1", trigger), finding})
	if len(junits) != 1 || junits[0].FailureOutput == nil {
		t.Errorf("with UDN teardown and a finding the test must fail: %+v", junits[0])
	}
	if junits[0].Name != TestName {
		t.Errorf("unexpected test name %q", junits[0].Name)
	}
}

func TestOVNKubeControllerLogHandler(t *testing.T) {
	beginning := mustTime(t, "2026-09-17T05:30:00Z")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ovnkube-node-q6rnk"}, Spec: corev1.PodSpec{NodeName: "master-1"}}
	lines := []struct {
		at   string
		line string
	}{
		// real lines from ovnkube-node-q6rnk in run 2100442614082834432
		{"2026-09-17T06:25:57.237155Z", "I0917 06:25:57.237155    8894 layer2_user_defined_network_controller.go:512] Stopping controller for UDN cluster_udn_test-net-l8xd6"},
		{"2026-09-17T06:25:57.237175Z", "I0917 06:25:57.237175    8894 user_defined_node_network_controller.go:116] Stopping UDN node network controller for network cluster_udn_test-net-l8xd6"},
		{"2026-09-17T06:25:57.257981Z", "I0917 06:25:57.257981    8894 gateway.go:532] Reconciling gateway with updates"},
		{"2026-09-17T06:25:57.588063Z", "I0917 06:25:57.588063    8894 layer3_user_defined_network_controller.go:391] Delete OVN logical entities for layer3 network controller of network e2e-network-segmentation-5563_gryffindor"},
		{"2026-09-17T06:25:57.613996Z", "I0917 06:25:57.613996    8894 gateway.go:532] Reconciling gateway with updates"},
		{"2026-09-17T06:26:00.001792Z", "I0917 06:26:00.001792    8894 base_network_controller_pods.go:495] creating logical port"},
		// before the monitor started: ignored
		{"2026-09-17T05:10:00Z", "I0917 05:10:00.000000    8894 gateway.go:532] Reconciling gateway with updates"},
	}
	h := newOVNKubeControllerLogHandler(beginning)
	for _, l := range lines {
		h.HandleLogLine(podaccess.LogLineContent{Instant: mustTime(t, l.at), Pod: pod, Line: l.line})
	}
	got := h.Intervals()
	counts := map[monitorapi.IntervalSource]int{}
	for _, i := range got {
		counts[i.Source]++
		if i.Locator.Keys[monitorapi.LocatorNodeKey] != "master-1" {
			t.Errorf("interval not keyed by node: %v", i.Locator)
		}
	}
	if counts[SourceGatewayReconcile] != 2 || counts[SourceUDNTeardown] != 2 {
		t.Errorf("unexpected intervals %v", counts)
	}
}

func TestPeersFromNodes(t *testing.T) {
	nodes := []corev1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "master-0", Labels: map[string]string{"node-role.kubernetes.io/master": ""}},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeHostName, Address: "m0"}, {Type: corev1.NodeInternalIP, Address: "10.0.0.5"}}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.128.2"}}}},
	}
	peers, cp := peersFromNodes(nodes)
	if strings.Join(peers, ",") != "master-0=10.0.0.5,worker-a=10.0.128.2" || strings.Join(cp, ",") != "master-0" {
		t.Errorf("unexpected peers %v control plane %v", peers, cp)
	}
	d := buildDeployment("img", peers, cp, "https://api-int.example:6443")
	args := strings.Join(d.Spec.Template.Spec.Containers[0].Command, " ")
	if *d.Spec.Replicas != 2 || !strings.Contains(args, "--peers=master-0=10.0.0.5,worker-a=10.0.128.2") || !strings.Contains(args, "--api-int-url=https://api-int.example:6443") {
		t.Errorf("unexpected deployment %v %s", *d.Spec.Replicas, args)
	}
	if strings.Contains(strings.Join(deploymentTemplate.Spec.Template.Spec.Containers[0].Command, " "), "--peers") {
		t.Errorf("buildDeployment must not mutate the template")
	}
}
