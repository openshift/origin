package topology_transitions

import (
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
)

var snoToHACompactTestSpec = TransitionSpec{
	Name:                     "sno-to-ha-compact",
	To:                       configv1.InfrastructureSpec{ControlPlaneTopology: configv1.HighlyAvailableTopologyMode},
	ToInfrastructureTopology: configv1.HighlyAvailableTopologyMode,
	HACompact:                true,
}

// TestValidateExactInfrastructureNodeCount checks exact inventory boundaries.
func TestValidateExactInfrastructureNodeCount(t *testing.T) {
	tests := []struct {
		name      string
		nodeCount int
		required  int
		wantErr   bool
	}{
		{name: "exact node count", nodeCount: 3, required: 3},
		{name: "extra unlabeled node", nodeCount: 4, required: 3, wantErr: true},
		{name: "too few nodes", nodeCount: 2, required: 3, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := &corev1.NodeList{Items: make([]corev1.Node, tt.nodeCount)}
			err := validateExactInfrastructureNodeCount(nodes, tt.required)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateExactInfrastructureNodeCount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseTransitionTarget checks required lane inputs and boolean parsing.
func TestParseTransitionTarget(t *testing.T) {
	tests := []struct {
		name           string
		controlPlane   string
		infrastructure string
		haCompact      string
		want           transitionTarget
		wantErr        bool
	}{
		{
			name:           "compact enabled",
			controlPlane:   string(configv1.HighlyAvailableTopologyMode),
			infrastructure: string(configv1.HighlyAvailableTopologyMode),
			haCompact:      "true",
			want: transitionTarget{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				HACompact:              true,
			},
		},
		{name: "control plane target is required", infrastructure: "HighlyAvailable", wantErr: true},
		{name: "infrastructure target is required", controlPlane: "HighlyAvailable", wantErr: true},
		{
			name:           "compact must be a boolean",
			controlPlane:   "HighlyAvailable",
			infrastructure: "HighlyAvailable",
			haCompact:      "sometimes",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTransitionTarget(tt.controlPlane, tt.infrastructure, tt.haCompact)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTransitionTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parseTransitionTarget() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestSelectTransition requires exactly one definition for the lane target.
func TestSelectTransition(t *testing.T) {
	tests := []struct {
		name        string
		specs       []TransitionSpec
		target      transitionTarget
		wantName    string
		wantErrText string
	}{
		{
			name:  "SNO to HA compact target selects the transition",
			specs: []TransitionSpec{snoToHACompactTestSpec},
			target: transitionTarget{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				HACompact:              true,
			},
			wantName: snoToHACompactTestSpec.Name,
		},
		{
			name:  "non-compact target does not select the compact transition",
			specs: []TransitionSpec{snoToHACompactTestSpec},
			target: transitionTarget{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
			},
			wantErrText: "no transition matches",
		},
		{
			name:  "infrastructure target must match",
			specs: []TransitionSpec{snoToHACompactTestSpec},
			target: transitionTarget{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.DualReplicaTopologyMode,
				HACompact:              true,
			},
			wantErrText: "no transition matches",
		},
		{
			name:  "control plane target must match",
			specs: []TransitionSpec{snoToHACompactTestSpec},
			target: transitionTarget{
				ControlPlaneTopology:   configv1.DualReplicaTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				HACompact:              true,
			},
			wantErrText: "no transition matches",
		},
		{
			name: "one match among other definitions",
			specs: []TransitionSpec{
				{Name: "other", To: configv1.InfrastructureSpec{ControlPlaneTopology: configv1.DualReplicaTopologyMode}},
				snoToHACompactTestSpec,
			},
			target: transitionTarget{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				HACompact:              true,
			},
			wantName: snoToHACompactTestSpec.Name,
		},
		{
			name: "two matching definitions are rejected",
			specs: []TransitionSpec{
				snoToHACompactTestSpec,
				{Name: "second-leg", To: snoToHACompactTestSpec.To, ToInfrastructureTopology: configv1.HighlyAvailableTopologyMode, HACompact: true},
			},
			target: transitionTarget{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				HACompact:              true,
			},
			wantErrText: "sno-to-ha-compact, second-leg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectTransition(tt.specs, tt.target)
			if tt.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("selectTransition() error = %v, want text %q", err, tt.wantErrText)
				}
				return
			}
			if err != nil || got.Name != tt.wantName {
				t.Errorf("selectTransition() = %q, %v, want %q, nil", got.Name, err, tt.wantName)
			}
		})
	}
}

// TestTransitionSpecMatchesFrom checks source topology and platform matching.
func TestTransitionSpecMatchesFrom(t *testing.T) {
	compactFrom := configv1.InfrastructureStatus{
		ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
		InfrastructureTopology: configv1.SingleReplicaTopologyMode,
		PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
	}

	tests := []struct {
		name  string
		spec  TransitionSpec
		infra *configv1.Infrastructure
		want  bool
	}{
		{
			name: "exact match",
			spec: TransitionSpec{From: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
			}},
			want: true,
		},
		{
			name: "wrong control plane topology",
			spec: TransitionSpec{From: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
			}},
			want: false,
		},
		{
			name: "wrong infrastructure topology",
			spec: TransitionSpec{From: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.DualReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
			}},
			want: false,
		},
		{
			name: "wrong platform",
			spec: TransitionSpec{From: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				PlatformStatus:         &configv1.PlatformStatus{Type: configv1.AWSPlatformType},
			}},
			want: false,
		},
		{
			name: "nil platform status when spec requires one",
			spec: TransitionSpec{From: compactFrom},
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
				InfrastructureTopology: configv1.SingleReplicaTopologyMode,
			}},
			want: false,
		},
		{
			name: "zero-value spec.From is a wildcard matching any status",
			spec: TransitionSpec{},
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
