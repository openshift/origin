package operatorloganalyzer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/monitortestframework"
)

func TestScanAllOperatorPodsReturnsContextCancellationAfterTransientListFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		cancel() // stop the retry loop after the first failed request
		return true, nil, apierrors.NewServiceUnavailable("apiserver is restarting")
	})

	err := scanAllOperatorPods(ctx, client, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("scanAllOperatorPods() error = %v, want context cancellation", err)
	}
}

func TestScanAllOperatorPodsRetriesTransientListErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	attempts := 0
	client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts < 3 {
			return true, nil, apierrors.NewServiceUnavailable("apiserver is restarting")
		}
		return false, nil, nil
	})

	if err := scanAllOperatorPods(context.Background(), client, false); err != nil {
		t.Fatalf("scanAllOperatorPods() returned unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("scanAllOperatorPods() made %d list attempts, want 3", attempts)
	}
}

func TestCollectDataDoesNotExposeAPIServerURL(t *testing.T) {
	const internalAPIHost = "api-int.mycluster.example.com"

	tests := []struct {
		name            string
		reducedTopology bool
		wantFlake       bool
	}{
		{name: "reduced topology returns a sanitized flake", reducedTopology: true, wantFlake: true},
		{name: "highly available topology returns a sanitized hard error", reducedTopology: false, wantFlake: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewServiceUnavailable("request to https://" + internalAPIHost + ":6443 failed")
			})

			analyzer := &operatorLogAnalyzer{
				kubeClient:      client,
				reducedTopology: tt.reducedTopology,
			}
			_, _, err := analyzer.CollectData(context.Background(), "", time.Time{}, time.Time{})
			if err == nil {
				t.Fatal("CollectData() succeeded, want an error")
			}

			var flakeErr *monitortestframework.FlakeError
			if got := errors.As(err, &flakeErr); got != tt.wantFlake {
				t.Errorf("CollectData() flake classification = %v, want %v", got, tt.wantFlake)
			}
			if strings.Contains(err.Error(), internalAPIHost) {
				t.Errorf("CollectData() exposed the apiserver address: %q", err)
			}
		})
	}
}

func TestCollectDataClassifiesPodNotFoundByTopology(t *testing.T) {
	tests := []struct {
		name            string
		reducedTopology bool
		wantFlake       bool
	}{
		{name: "reduced topology returns a flake", reducedTopology: true, wantFlake: true},
		{name: "highly available topology returns a hard error", reducedTopology: false, wantFlake: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operatorPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "openshift-test-operator",
					Name:      "test-operator-abcde",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "operator"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			}
			client := fake.NewSimpleClientset(operatorPod)
			client.PrependReactor("get", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, operatorPod.Name)
			})

			analyzer := &operatorLogAnalyzer{
				kubeClient:      client,
				reducedTopology: tt.reducedTopology,
			}
			_, _, err := analyzer.CollectData(context.Background(), "", time.Time{}, time.Time{})
			if err == nil {
				t.Fatal("CollectData() succeeded after an operator pod disappeared")
			}

			var flakeErr *monitortestframework.FlakeError
			if got := errors.As(err, &flakeErr); got != tt.wantFlake {
				t.Errorf("CollectData() flake classification = %v, want %v", got, tt.wantFlake)
			}
		})
	}
}

func TestIsTransientScrapeErrorRecognizesReducedTopologyRecoveryErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "kubelet proxy authorization while a node is restarting",
			err:  errors.New("Internal error occurred: Authorization error (user=system:kube-apiserver, verb=get, resource=nodes, subresource=proxy)"),
			want: true,
		},
		{
			name: "API storage is reinitializing",
			err:  errors.New("Internal error occurred: storage is (re)initializing"),
			want: true,
		},
		{
			name: "unrelated authorization error remains strict",
			err:  errors.New("Authorization error (user=system:anonymous, verb=get, resource=secrets)"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientScrapeError(tt.err); got != tt.want {
				t.Errorf("isTransientScrapeError() = %v, want %v", got, tt.want)
			}
		})
	}
}
