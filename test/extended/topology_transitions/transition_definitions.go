package topology_transitions

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
)

// transitionDefinition describes a supported transition and the state expected
// before the lane provisions the extra nodes.
type transitionDefinition struct {
	Name                    string
	Source                  configv1.InfrastructureStatus
	Target                  configv1.TopologyState
	SourceNodes             nodeCounts
	ExpectedPreflightChecks map[string]bool
}

var transitionDefinitions = []transitionDefinition{
	{
		Name: "SNO to compact HA",
		Source: configv1.InfrastructureStatus{
			ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
			InfrastructureTopology: configv1.SingleReplicaTopologyMode,
			PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
		},
		Target: configv1.TopologyState{
			ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
			InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
		},
		SourceNodes: nodeCounts{
			ControlPlaneNodes:            1,
			SchedulableControlPlaneNodes: 1,
		},
		ExpectedPreflightChecks: map[string]bool{
			"ControlPlaneNodeCountSatisfied":    false,
			"ControlPlaneNodesSchedulable":      false,
			"ControlPlaneNodesReady":            false,
			"ControlPlaneNodesAreWorkers":       false,
			"EtcdVotingMembersSatisfied":        false,
			"InfrastructureNodeCountSatisfied":  true,
			"EtcdQuorumAvailable":               true,
			"EtcdNotProgressing":                true,
			"ClusterOperatorsStable":            true,
			"NoClusterVersionUpgradeInProgress": true,
		},
	},
}

func (definition transitionDefinition) sourceTopology() configv1.TopologyState {
	return configv1.TopologyState{
		ControlPlaneTopology:   definition.Source.ControlPlaneTopology,
		InfrastructureTopology: definition.Source.InfrastructureTopology,
	}
}

func (definition transitionDefinition) matchesSource(infra *configv1.Infrastructure) bool {
	if infra.Status.ControlPlaneTopology != definition.Source.ControlPlaneTopology ||
		infra.Status.InfrastructureTopology != definition.Source.InfrastructureTopology {
		return false
	}
	if platform := definition.Source.PlatformStatus; platform != nil && platform.Type != "" {
		return infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == platform.Type
	}

	return true
}

func supportsTarget(target configv1.TopologyState) bool {
	for _, definition := range transitionDefinitions {
		if definition.Target == target {
			return true
		}
	}

	return false
}

func selectTransitionDefinition(infra *configv1.Infrastructure, target configv1.TopologyState) (*transitionDefinition, error) {
	var selected *transitionDefinition
	for i := range transitionDefinitions {
		definition := &transitionDefinitions[i]
		if definition.Target != target || !definition.matchesSource(infra) {
			continue
		}
		if selected != nil {
			return nil, fmt.Errorf("multiple transition definitions match the source and target: %q and %q", selected.Name, definition.Name)
		}

		selected = definition
	}
	if selected == nil {
		return nil, fmt.Errorf("no transition definition matches source control-plane=%s infrastructure=%s and target=%+v",
			infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology, target)
	}

	return selected, nil
}
