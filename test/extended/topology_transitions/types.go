package topology_transitions

import (
	"fmt"
	"strconv"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
)

// transitionTarget contains the exact node counts requested by the lane.
type transitionTarget struct {
	Topology configv1.TopologyState
	nodeCounts
}

type nodeCounts struct {
	ControlPlaneNodes            int
	ComputeNodes                 int
	ArbiterNodes                 int
	SchedulableControlPlaneNodes int
}

func transitionTargetFromEnvironment(getenv func(string) string) (transitionTarget, error) {
	target := transitionTarget{
		Topology: configv1.TopologyState{
			ControlPlaneTopology:   configv1.TopologyMode(strings.TrimSpace(getenv("TARGET_CONTROL_PLANE_TOPOLOGY"))),
			InfrastructureTopology: configv1.TopologyMode(strings.TrimSpace(getenv("TARGET_INFRASTRUCTURE_TOPOLOGY"))),
		},
	}
	if target.Topology.ControlPlaneTopology == "" || target.Topology.InfrastructureTopology == "" {
		return target, fmt.Errorf("TARGET_CONTROL_PLANE_TOPOLOGY and TARGET_INFRASTRUCTURE_TOPOLOGY must be set")
	}

	for _, count := range []struct {
		name  string
		value *int
	}{
		{"TARGET_CONTROL_PLANE_NODES", &target.ControlPlaneNodes},
		{"TARGET_COMPUTE_NODES", &target.ComputeNodes},
		{"TARGET_ARBITER_NODES", &target.ArbiterNodes},
		{"TARGET_CONTROL_PLANE_NODES_SCHEDULABLE", &target.SchedulableControlPlaneNodes},
	} {
		value, err := strconv.Atoi(strings.TrimSpace(getenv(count.name)))
		if err != nil || value < 0 {
			return target, fmt.Errorf("%s must be set to a non-negative integer", count.name)
		}
		*count.value = value
	}
	if target.SchedulableControlPlaneNodes > target.ControlPlaneNodes {
		return target, fmt.Errorf("TARGET_CONTROL_PLANE_NODES_SCHEDULABLE cannot exceed TARGET_CONTROL_PLANE_NODES")
	}

	return target, nil
}
