package topology_transitions

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestDetectChain(t *testing.T) {
	t.Cleanup(func() { exercisedTransitionName = "" })

	if err := detectChain("a"); err != nil {
		t.Fatalf("first call for a new transition should succeed, got: %v", err)
	}
	if err := detectChain("a"); err != nil {
		t.Fatalf("repeat calls for the same transition should succeed, got: %v", err)
	}
	if err := detectChain("b"); err == nil {
		t.Fatal("expected an error when a different transition tries to run in the same invocation, got nil")
	}
}

func TestControlPlaneTopologyPatch(t *testing.T) {
	tests := []struct {
		name string
		mode configv1.TopologyMode
		want string
	}{
		{
			name: "empty mode removes the field",
			want: `{"spec":{"controlPlaneTopology":null}}`,
		},
		{
			name: "non-empty mode sets the field",
			mode: configv1.HighlyAvailableTopologyMode,
			want: `{"spec":{"controlPlaneTopology":"HighlyAvailable"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(controlPlaneTopologyPatch(tt.mode)); got != tt.want {
				t.Errorf("controlPlaneTopologyPatch() = %s, want %s", got, tt.want)
			}
		})
	}
}
