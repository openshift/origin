package helpers

import (
	"context"
	"errors"
	stdnet "net"
	"net/url"
	"os"
	"syscall"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	configfake "github.com/openshift/client-go/config/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func etcdPod(name, revision string, ready bool) *corev1.Pod {
	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "openshift-etcd",
			Labels:    map[string]string{"etcd": "true", "revision": revision},
		},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readyStatus}},
		},
	}
}

func etcdClusterOperator(available, progressing, degraded configv1.ConditionStatus) *configv1.ClusterOperator {
	return &configv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: "etcd"},
		Status: configv1.ClusterOperatorStatus{
			Conditions: []configv1.ClusterOperatorStatusCondition{
				{Type: configv1.OperatorAvailable, Status: available},
				{Type: configv1.OperatorProgressing, Status: progressing},
				{Type: configv1.OperatorDegraded, Status: degraded},
			},
		},
	}
}

func TestSettledEtcdRevision(t *testing.T) {
	settledOperator := etcdClusterOperator(configv1.ConditionTrue, configv1.ConditionFalse, configv1.ConditionFalse)

	tests := []struct {
		name             string
		operator         *configv1.ClusterOperator
		pods             []*corev1.Pod
		expectedRevision string
	}{
		{
			name:             "all pods ready on one revision and operator settled",
			operator:         settledOperator,
			pods:             []*corev1.Pod{etcdPod("etcd-a", "37", true), etcdPod("etcd-b", "37", true), etcdPod("etcd-c", "37", true)},
			expectedRevision: "37",
		},
		{
			name:     "operator progressing on a new revision before any pod has rolled",
			operator: etcdClusterOperator(configv1.ConditionTrue, configv1.ConditionTrue, configv1.ConditionFalse),
			pods:     []*corev1.Pod{etcdPod("etcd-a", "37", true), etcdPod("etcd-b", "37", true), etcdPod("etcd-c", "37", true)},
		},
		{
			name:     "operator degraded",
			operator: etcdClusterOperator(configv1.ConditionTrue, configv1.ConditionFalse, configv1.ConditionTrue),
			pods:     []*corev1.Pod{etcdPod("etcd-a", "37", true)},
		},
		{
			name:     "pods on multiple revisions",
			operator: settledOperator,
			pods:     []*corev1.Pod{etcdPod("etcd-a", "38", true), etcdPod("etcd-b", "37", true), etcdPod("etcd-c", "37", true)},
		},
		{
			name:     "pod not ready",
			operator: settledOperator,
			pods:     []*corev1.Pod{etcdPod("etcd-a", "37", false), etcdPod("etcd-b", "37", true)},
		},
		{
			name:     "no etcd pods",
			operator: settledOperator,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := kubefake.NewSimpleClientset()
			for _, pod := range tc.pods {
				if _, err := kubeClient.CoreV1().Pods("openshift-etcd").Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			configClient := configfake.NewSimpleClientset(tc.operator)

			revision, reason := settledEtcdRevision(context.TODO(), kubeClient.CoreV1().Pods("openshift-etcd"), configClient.ConfigV1().ClusterOperators())
			if revision != tc.expectedRevision {
				t.Errorf("expected revision %q, got %q (reason: %s)", tc.expectedRevision, revision, reason)
			}
			if len(revision) == 0 && len(reason) == 0 {
				t.Errorf("expected a reason when etcd is not settled")
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "net/http: TLS handshake timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestTransientAPIError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "TLS handshake timeout from a GET",
			err:       &url.Error{Op: "Get", URL: "https://api.example.com:6443/apis/machine.openshift.io/v1/namespaces/openshift-machine-api/controlplanemachinesets/cluster", Err: timeoutError{}},
			transient: true,
		},
		{
			name:      "connection refused",
			err:       &url.Error{Op: "Get", URL: "https://api.example.com:6443", Err: &stdnet.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}}},
			transient: true,
		},
		{
			name:      "service unavailable",
			err:       apierrors.NewServiceUnavailable("unavailable"),
			transient: true,
		},
		{
			name:      "request timeout",
			err:       apierrors.NewTimeoutError("request did not complete within the allotted timeout", 0),
			transient: true,
		},
		{
			name: "not found",
			err:  apierrors.NewNotFound(schema.GroupResource{Group: "machine.openshift.io", Resource: "controlplanemachinesets"}, "cluster"),
		},
		{
			name: "forbidden",
			err:  apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "etcd", errors.New("denied")),
		},
		{
			name: "nil",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := transientAPIError(tc.err); got != tc.transient {
				t.Errorf("expected transient=%v, got %v for %v", tc.transient, got, tc.err)
			}
		})
	}
}
