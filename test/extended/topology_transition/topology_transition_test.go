package topology_transition

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestTransitionSpecMatchesFrom(t *testing.T) {
	compactFrom := configv1.InfrastructureStatus{
		ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
		InfrastructureTopology: configv1.SingleReplicaTopologyMode,
		PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
	}

	tests := []struct {
		name  string
		spec  transitionSpec
		infra *configv1.Infrastructure
		want  bool
	}{
		{
			name: "exact match",
			spec: transitionSpec{from: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
			}},
			want: true,
		},
		{
			name: "wrong control plane topology",
			spec: transitionSpec{from: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
			}},
			want: false,
		},
		{
			name: "wrong platform",
			spec: transitionSpec{from: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.AWSPlatformType},
			}},
			want: false,
		},
		{
			name: "nil platform status when spec requires one",
			spec: transitionSpec{from: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
			}},
			want: false,
		},
		{
			name: "zero-value spec.from is a wildcard matching any status",
			spec: transitionSpec{},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.matchesFrom(tt.infra); got != tt.want {
				t.Errorf("matchesFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}
