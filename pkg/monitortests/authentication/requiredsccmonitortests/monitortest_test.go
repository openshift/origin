package requiredsccmonitortests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
)

func TestRequiredSCCResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		namespace      string
		pods           []string
		annotations    map[string]string
		wantDiagnostic string
		wantFails      int
		wantPasses     int
		wantState      monitor.ResultState
	}{
		{
			name: "HyperShift system workloads flake", namespace: "kube-system",
			pods:      []string{"kas-connection-checker", "konnectivity-agent", "global-pull-secret-syncer", "kube-apiserver-proxy"},
			wantFails: 1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "DNS workloads awaiting pinning flake", namespace: "openshift-dns",
			pods:      []string{"dns-default", "node-resolver"},
			wantFails: 1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "ingress workloads awaiting pinning flake", namespace: "openshift-ingress",
			pods:      []string{"istiod-openshift-gateway"},
			wantFails: 1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "multus workloads awaiting pinning flake", namespace: "openshift-multus",
			pods:      []string{"multus", "multus-additional-cni-plugins", "network-metrics-daemon"},
			wantFails: 1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "network operator workloads awaiting pinning flake", namespace: "openshift-network-operator",
			pods:      []string{"iptables-alerter"},
			wantFails: 1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "OVN workloads awaiting pinning flake", namespace: "openshift-ovn-kubernetes",
			pods:      []string{"ovnkube-node"},
			wantFails: 1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "unlisted OpenShift namespace fails", namespace: "openshift-authentication",
			pods:      []string{"oauth-openshift"},
			wantFails: 1, wantState: monitor.Failed,
		},
		{
			name: "unlisted kube namespace fails", namespace: "kube-unlisted",
			pods:      []string{"unannotated"},
			wantFails: 1, wantState: monitor.Failed,
		},
		{
			name: "validated pod still needs pinning in system namespace", namespace: "kube-system",
			pods: []string{"unpinned"}, annotations: map[string]string{securityv1.ValidatedSCCAnnotation: "restricted-v2"},
			wantDiagnostic: "suggested required-scc: 'restricted-v2'",
			wantFails:      1, wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "validated pod still needs pinning in unlisted namespace", namespace: "openshift-authentication",
			pods: []string{"unpinned"}, annotations: map[string]string{securityv1.ValidatedSCCAnnotation: "restricted-v2"},
			wantDiagnostic: "suggested required-scc: 'restricted-v2'",
			wantFails:      1, wantState: monitor.Failed,
		},
		{
			name: "pinned pod passes", namespace: "openshift-authentication",
			pods:        []string{"pinned"},
			annotations: map[string]string{securityv1.RequiredSCCAnnotation: "restricted-v2", securityv1.ValidatedSCCAnnotation: "restricted-v2"},
			wantPasses:  1, wantState: monitor.Succeeded,
		},
		{
			name: "pinned pod in pending namespace passes without a flake", namespace: "openshift-dns",
			pods:        []string{"pinned"},
			annotations: map[string]string{securityv1.RequiredSCCAnnotation: "privileged", securityv1.ValidatedSCCAnnotation: "privileged"},
			wantPasses:  1, wantState: monitor.Succeeded,
		},
		{
			name: "disallowed nonstandard SCC remains a failure", namespace: "openshift-authentication",
			pods:        []string{"wrong-namespace"},
			annotations: map[string]string{securityv1.RequiredSCCAnnotation: "node-exporter", securityv1.ValidatedSCCAnnotation: "node-exporter"},
			wantFails:   1, wantState: monitor.Failed,
		},
		{
			name: "empty namespace passes", namespace: "openshift-authentication",
			wantPasses: 1, wantState: monitor.Succeeded,
		},
		{
			name: "user namespace is out of scope", namespace: "application",
			pods: []string{"app"}, wantState: monitor.Succeeded,
		},
		{
			name: "must-gather namespace is out of scope", namespace: "openshift-must-gather-example",
			pods: []string{"gather"}, wantState: monitor.Succeeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []runtime.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tt.namespace}}}
			for _, name := range tt.pods {
				objects = append(objects, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: name, Namespace: tt.namespace, Annotations: tt.annotations,
				}})
			}
			checker := &requiredSCCAnnotationChecker{kubeClient: fake.NewSimpleClientset(objects...)}
			_, results, err := checker.CollectData(t.Context(), t.TempDir(), time.Time{}, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			wantName := fmt.Sprintf("[sig-auth] all workloads in ns/%s must set the '%s' annotation", tt.namespace, securityv1.RequiredSCCAnnotation)
			fails, passes := 0, 0
			for _, result := range results {
				if result.Name != wantName {
					t.Errorf("JUnit name = %q, want %q", result.Name, wantName)
				}
				if result.SkipMessage != nil {
					t.Fatalf("unexpected skipped result: %+v", result)
				}
				if result.FailureOutput == nil {
					passes++
					continue
				}
				fails++
				if tt.wantDiagnostic != "" && !strings.Contains(result.FailureOutput.Output, tt.wantDiagnostic) {
					t.Errorf("failure diagnostics = %q, want substring %q", result.FailureOutput.Output, tt.wantDiagnostic)
				}
				if result.SystemOut != result.FailureOutput.Output {
					t.Error("JUnit system output must retain the failure diagnostics")
				}
				for _, pod := range tt.pods {
					if !strings.Contains(result.FailureOutput.Output, "'"+pod+"'") {
						t.Errorf("failure diagnostics do not name pod %q: %s", pod, result.FailureOutput.Output)
					}
				}
			}
			if fails != tt.wantFails || passes != tt.wantPasses {
				t.Errorf("failed/passing JUnits = %d/%d, want %d/%d", fails, passes, tt.wantFails, tt.wantPasses)
			}
			if state := runSCCMonitor(t, checker); state != tt.wantState {
				t.Errorf("monitor result = %s, want %s", state, tt.wantState)
			}
		})
	}
}

func TestRequiredSCCMixedPods(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		namespace  string
		wantPasses int
		wantState  monitor.ResultState
	}{
		{namespace: "openshift-authentication", wantState: monitor.Failed},
		{namespace: "openshift-dns", wantPasses: 1, wantState: monitor.Succeeded},
	} {
		t.Run(tt.namespace, func(t *testing.T) {
			t.Parallel()
			client := fake.NewSimpleClientset(
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tt.namespace}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: "a-pinned", Namespace: tt.namespace,
					Annotations: map[string]string{securityv1.RequiredSCCAnnotation: "restricted-v2", securityv1.ValidatedSCCAnnotation: "restricted-v2"},
				}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "z-unpinned", Namespace: tt.namespace}},
			)
			checker := &requiredSCCAnnotationChecker{kubeClient: client}
			_, results, err := checker.CollectData(t.Context(), t.TempDir(), time.Time{}, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			wantName := fmt.Sprintf("[sig-auth] all workloads in ns/%s must set the '%s' annotation", tt.namespace, securityv1.RequiredSCCAnnotation)
			fails, passes := 0, 0
			for _, result := range results {
				if result.Name != wantName || result.SkipMessage != nil {
					t.Fatalf("unexpected JUnit: %+v", result)
				}
				if result.FailureOutput == nil {
					passes++
					continue
				}
				fails++
				if !strings.Contains(result.FailureOutput.Output, "'z-unpinned'") || strings.Contains(result.FailureOutput.Output, "'a-pinned'") {
					t.Errorf("failure diagnostics should identify only the unpinned pod: %s", result.FailureOutput.Output)
				}
			}
			if fails != 1 || passes != tt.wantPasses {
				t.Errorf("failed/passing JUnits = %d/%d, want 1/%d", fails, passes, tt.wantPasses)
			}
			if state := runSCCMonitor(t, checker); state != tt.wantState {
				t.Errorf("monitor result = %s, want %s", state, tt.wantState)
			}
		})
	}
}

func TestRequiredSCCFlakeDoesNotHideOtherNamespaceFailure(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "konnectivity-agent", Namespace: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-authentication"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oauth-openshift", Namespace: "openshift-authentication"}},
	)
	if state := runSCCMonitor(t, &requiredSCCAnnotationChecker{kubeClient: client}); state != monitor.Failed {
		t.Fatalf("monitor result = %s, want %s despite the kube-system flake", state, monitor.Failed)
	}
}

func TestRequiredSCCCollectionErrors(t *testing.T) {
	t.Parallel()

	for _, resource := range []string{"namespaces", "pods"} {
		t.Run(resource, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}})
			listErr := errors.New("test API list error")
			client.PrependReactor("list", resource, func(kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, listErr
			})
			checker := &requiredSCCAnnotationChecker{kubeClient: client}
			_, results, err := checker.CollectData(t.Context(), t.TempDir(), time.Time{}, time.Time{})
			if !errors.Is(err, listErr) {
				t.Fatalf("expected list error, got results=%v err=%v", results, err)
			}
			if len(results) != 0 {
				t.Errorf("unexpected namespace results on collection error: %v", results)
			}
			if state := runSCCMonitor(t, checker); state != monitor.Failed {
				t.Errorf("monitor result = %s, want %s on collection error", state, monitor.Failed)
			}
		})
	}
}

// Preserve the fake client during startup; collection and result aggregation
// still run through the production checker and monitor lifecycle.
type checkerWithFakeClient struct {
	*requiredSCCAnnotationChecker
}

func (*checkerWithFakeClient) StartCollection(context.Context, *rest.Config, monitorapi.RecorderWriter) error {
	return nil
}

func runSCCMonitor(t *testing.T, checker *requiredSCCAnnotationChecker) monitor.ResultState {
	t.Helper()
	registry := monitortestframework.NewMonitorTestRegistry()
	if err := registry.AddMonitorTest("required-scc-annotation-checker", "Authentication", &checkerWithFakeClient{checker}); err != nil {
		t.Fatal(err)
	}
	m := monitor.NewMonitor(monitor.NewRecorder(), nil, t.TempDir(), registry)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := m.Stop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return state
}
