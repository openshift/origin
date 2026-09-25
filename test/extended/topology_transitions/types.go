package topology_transitions

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
)

const (
	targetControlPlaneTopologyEnvVar   = "TARGET_CONTROL_PLANE_TOPOLOGY"
	targetInfrastructureTopologyEnvVar = "TARGET_INFRASTRUCTURE_TOPOLOGY"
	targetHACompactEnvVar              = "TARGET_HA_COMPACT"
)

// TransitionSpec describes one supported topology transition and its test
// preconditions.
type TransitionSpec struct {
	Name string
	From configv1.InfrastructureStatus
	To   configv1.InfrastructureSpec

	// InfrastructureTopology is status-only; the controller derives it from
	// the requested control-plane topology.
	ToInfrastructureTopology configv1.TopologyMode
	HACompact                bool

	RequiredControlPlaneNodes        int
	RequiredInfrastructureNodes      int // zero skips an exact total-node-count check
	RequireDualRoleControlPlane      bool
	AllowDedicatedWorkers            bool
	RequiredEtcdVotingMembers        int
	ExpectedScheduleFailureSubstring string

	ClusterOperatorStabilityTimeout time.Duration
	AdmissionWaitTimeout            time.Duration
	StatusConvergeTimeout           time.Duration
	CompletionWaitTimeout           time.Duration
	OperatorSettleTimeout           time.Duration
}

// transitionTarget selects transition specs using lane-provided target data.
type transitionTarget struct {
	ControlPlaneTopology   configv1.TopologyMode
	InfrastructureTopology configv1.TopologyMode
	HACompact              bool
}

// parseTransitionTarget reads the lane target values. HA compact defaults to
// false when its environment variable is empty.
func parseTransitionTarget(controlPlane, infrastructure, haCompact string) (transitionTarget, error) {
	controlPlane = strings.TrimSpace(controlPlane)
	infrastructure = strings.TrimSpace(infrastructure)
	if controlPlane == "" {
		return transitionTarget{}, fmt.Errorf("%s must be set", targetControlPlaneTopologyEnvVar)
	}
	if infrastructure == "" {
		return transitionTarget{}, fmt.Errorf("%s must be set", targetInfrastructureTopologyEnvVar)
	}

	target := transitionTarget{
		ControlPlaneTopology:   configv1.TopologyMode(controlPlane),
		InfrastructureTopology: configv1.TopologyMode(infrastructure),
	}
	if strings.TrimSpace(haCompact) == "" {
		return target, nil
	}
	compact, err := strconv.ParseBool(strings.TrimSpace(haCompact))
	if err != nil {
		return transitionTarget{}, fmt.Errorf("%s must be a boolean: %w", targetHACompactEnvVar, err)
	}
	target.HACompact = compact
	return target, nil
}

// matchingTransitions selects definitions that match the requested target.
func matchingTransitions(specs []TransitionSpec, target transitionTarget) []TransitionSpec {
	var matches []TransitionSpec
	for _, spec := range specs {
		if spec.To.ControlPlaneTopology != target.ControlPlaneTopology ||
			spec.ToInfrastructureTopology != target.InfrastructureTopology ||
			spec.HACompact != target.HACompact {
			continue
		}
		matches = append(matches, spec)
	}
	return matches
}

// matchesFrom checks the starting topology and platform fields declared by spec.
func (spec TransitionSpec) matchesFrom(infra *configv1.Infrastructure) bool {
	if spec.From.ControlPlaneTopology != "" && infra.Status.ControlPlaneTopology != spec.From.ControlPlaneTopology {
		return false
	}
	if spec.From.InfrastructureTopology != "" && infra.Status.InfrastructureTopology != spec.From.InfrastructureTopology {
		return false
	}
	if spec.From.PlatformStatus != nil && spec.From.PlatformStatus.Type != "" {
		if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.Type != spec.From.PlatformStatus.Type {
			return false
		}
	}
	return true
}
