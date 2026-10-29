package topology_transitions

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestSupportsTarget(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		topology configv1.TopologyState
		counts   nodeCounts
		want     bool
	}{
		{
			name:   "compact HA",
			counts: nodeCounts{ControlPlaneNodes: 3, SchedulableControlPlaneNodes: 3},
			want:   true,
		},
		{
			name:   "too few control-plane nodes",
			counts: nodeCounts{ControlPlaneNodes: 2, SchedulableControlPlaneNodes: 2},
		},
		{
			name:   "too many control-plane nodes",
			counts: nodeCounts{ControlPlaneNodes: 4, SchedulableControlPlaneNodes: 3},
		},
		{
			name:   "extra compute node",
			counts: nodeCounts{ControlPlaneNodes: 3, ComputeNodes: 1, SchedulableControlPlaneNodes: 3},
		},
		{
			name:   "extra arbiter node",
			counts: nodeCounts{ControlPlaneNodes: 3, ArbiterNodes: 1, SchedulableControlPlaneNodes: 3},
		},
		{
			name:   "unschedulable control-plane node",
			counts: nodeCounts{ControlPlaneNodes: 3, SchedulableControlPlaneNodes: 2},
		},
		{
			name:     "unsupported control-plane topology",
			topology: configv1.TopologyState{ControlPlaneTopology: configv1.SingleReplicaTopologyMode, InfrastructureTopology: configv1.HighlyAvailableTopologyMode},
			counts:   nodeCounts{ControlPlaneNodes: 3, SchedulableControlPlaneNodes: 3},
		},
		{
			name:     "unsupported infrastructure topology",
			topology: configv1.TopologyState{ControlPlaneTopology: configv1.HighlyAvailableTopologyMode, InfrastructureTopology: configv1.SingleReplicaTopologyMode},
			counts:   nodeCounts{ControlPlaneNodes: 3, SchedulableControlPlaneNodes: 3},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			target := transitionTarget{
				Topology:   tc.topology,
				nodeCounts: tc.counts,
			}
			if target.Topology == (configv1.TopologyState{}) {
				target.Topology = configv1.TopologyState{
					ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
					InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				}
			}

			if got := supportsTarget(target); got != tc.want {
				t.Errorf("supportsTarget(%+v) = %t, want %t", target, got, tc.want)
			}
		})
	}
}
