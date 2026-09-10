package operatorloganalyzer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/monitortestframework"
)

func TestScanAllOperatorPodsDoesNotIgnoreFailedList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		cancel() // stop the retry loop after the first failed request
		return true, nil, apierrors.NewServiceUnavailable("apiserver is restarting")
	})

	if err := scanAllOperatorPods(ctx, client, false); err == nil {
		t.Fatal("scanAllOperatorPods() succeeded after its pod list failed")
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
			ctx, cancel := context.WithCancel(context.Background())
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				cancel() // stop the retry loop after the first failed request
				return true, nil, apierrors.NewServiceUnavailable("request to https://" + internalAPIHost + ":6443 failed")
			})

			analyzer := &operatorLogAnalyzer{
				kubeClient:      client,
				reducedTopology: tt.reducedTopology,
			}
			_, _, err := analyzer.CollectData(ctx, "", time.Time{}, time.Time{})
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
