package platformidentification

import (
	"context"
	"errors"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configfake "github.com/openshift/client-go/config/clientset/versioned/fake"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	clienttesting "k8s.io/client-go/testing"
)

func TestIsReducedTopology(t *testing.T) {
	tests := []struct {
		topology string
		want     bool
	}{
		{topology: TopologyDualReplica, want: true},
		{topology: TopologySingleReplica, want: true},
		{topology: TopologyHighlyAvailable, want: false},
		{topology: TopologyExternal, want: false},
		{topology: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.topology, func(t *testing.T) {
			if got := IsReducedTopology(tt.topology); got != tt.want {
				t.Errorf("IsReducedTopology(%q) = %v, want %v", tt.topology, got, tt.want)
			}
		})
	}
}

func infrastructureWithTopology(mode configv1.TopologyMode) *configv1.Infrastructure {
	return &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status:     configv1.InfrastructureStatus{ControlPlaneTopology: mode},
	}
}

func TestControlPlaneTopology(t *testing.T) {
	tests := []struct {
		name    string
		mode    configv1.TopologyMode
		want    string
		wantErr bool
	}{
		{name: "highly available", mode: configv1.HighlyAvailableTopologyMode, want: TopologyHighlyAvailable},
		{name: "single replica", mode: configv1.SingleReplicaTopologyMode, want: TopologySingleReplica},
		{name: "dual replica", mode: configv1.DualReplicaTopologyMode, want: TopologyDualReplica},
		{name: "external", mode: configv1.ExternalTopologyMode, want: TopologyExternal},
		{name: "unrecognized", mode: configv1.TopologyMode("Quadruple"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := configfake.NewSimpleClientset(infrastructureWithTopology(tt.mode))

			got, err := controlPlaneTopology(context.Background(), client.ConfigV1())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("controlPlaneTopology() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("controlPlaneTopology() returned unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("controlPlaneTopology() = %q, want %q", got, tt.want)
			}
		})
	}
}

// withFastBackoff shrinks the retry backoff so retry behavior can be asserted
// without the test sleeping for the real 15s.
func withFastBackoff(t *testing.T) {
	t.Helper()
	original := topologyReadBackoff
	topologyReadBackoff = wait.Backoff{Duration: time.Millisecond, Factor: 1.0, Steps: 4}
	t.Cleanup(func() { topologyReadBackoff = original })
}

func TestControlPlaneTopologyRetriesTransientErrors(t *testing.T) {
	withFastBackoff(t)

	client := configfake.NewSimpleClientset(infrastructureWithTopology(configv1.DualReplicaTopologyMode))
	attempts := 0
	client.PrependReactor("get", "infrastructures", func(action clienttesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts < 3 {
			return true, nil, kapierrs.NewServiceUnavailable("apiserver is restarting")
		}
		return false, nil, nil
	})

	got, err := controlPlaneTopology(context.Background(), client.ConfigV1())
	if err != nil {
		t.Fatalf("controlPlaneTopology() returned unexpected error: %v", err)
	}
	if got != TopologyDualReplica {
		t.Errorf("controlPlaneTopology() = %q, want %q", got, TopologyDualReplica)
	}
	if attempts != 3 {
		t.Errorf("made %d attempts, want 3", attempts)
	}
}

func TestControlPlaneTopologyDoesNotRetryNotFound(t *testing.T) {
	withFastBackoff(t)

	client := configfake.NewSimpleClientset()
	attempts := 0
	client.PrependReactor("get", "infrastructures", func(action clienttesting.Action) (bool, runtime.Object, error) {
		attempts++
		return true, nil, kapierrs.NewNotFound(schema.GroupResource{Group: "config.openshift.io", Resource: "infrastructures"}, "cluster")
	})

	if _, err := controlPlaneTopology(context.Background(), client.ConfigV1()); err == nil {
		t.Fatal("controlPlaneTopology() succeeded, want an error")
	}
	if attempts != 1 {
		t.Errorf("made %d attempts, want 1: a missing resource cannot be waited out", attempts)
	}
}

// TestResolveReducedTopologyFallsBackToReduced covers the safety property the
// callers depend on: a topology we cannot read must never be reported as highly
// available, because that would subject a recovering DualReplica or SingleReplica
// cluster to HA's strict error handling.
func TestResolveReducedTopologyFallsBackToReduced(t *testing.T) {
	withFastBackoff(t)

	client := configfake.NewSimpleClientset()
	client.PrependReactor("get", "infrastructures", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})

	reduced, topology, err := resolveReducedTopology(context.Background(), client.ConfigV1())
	if err == nil {
		t.Fatal("resolveReducedTopology() succeeded, want an error")
	}
	if !reduced {
		t.Error("resolveReducedTopology() reported a non-reduced topology it never managed to read")
	}
	if topology != "" {
		t.Errorf("resolveReducedTopology() = %q, want an empty topology alongside the error", topology)
	}
}

func TestResolveReducedTopology(t *testing.T) {
	tests := []struct {
		name        string
		mode        configv1.TopologyMode
		wantReduced bool
	}{
		{name: "dual replica is reduced", mode: configv1.DualReplicaTopologyMode, wantReduced: true},
		{name: "single replica is reduced", mode: configv1.SingleReplicaTopologyMode, wantReduced: true},
		{name: "highly available is not reduced", mode: configv1.HighlyAvailableTopologyMode, wantReduced: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := configfake.NewSimpleClientset(infrastructureWithTopology(tt.mode))

			reduced, _, err := resolveReducedTopology(context.Background(), client.ConfigV1())
			if err != nil {
				t.Fatalf("resolveReducedTopology() returned unexpected error: %v", err)
			}
			if reduced != tt.wantReduced {
				t.Errorf("resolveReducedTopology() reduced = %v, want %v", reduced, tt.wantReduced)
			}
		})
	}
}
