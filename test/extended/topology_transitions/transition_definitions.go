package topology_transitions

import (
	"time"

	configv1 "github.com/openshift/api/config/v1"
)

// transitions lists the supported topology transition shapes.
var transitions = []TransitionSpec{snoToHACompact}

var snoToHACompact = TransitionSpec{
	Name: "sno-to-ha-compact",

	From: configv1.InfrastructureStatus{
		ControlPlaneTopology:   configv1.SingleReplicaTopologyMode,
		InfrastructureTopology: configv1.SingleReplicaTopologyMode,
		PlatformStatus:         &configv1.PlatformStatus{Type: configv1.NonePlatformType},
	},
	To: configv1.InfrastructureSpec{
		ControlPlaneTopology: configv1.HighlyAvailableTopologyMode,
	},
	ToInfrastructureTopology: configv1.HighlyAvailableTopologyMode,
	HACompact:                true,

	RequiredControlPlaneNodes:        3,
	RequiredInfrastructureNodes:      3,
	RequireDualRoleControlPlane:      true,
	RequiredEtcdVotingMembers:        3,
	ExpectedScheduleFailureSubstring: "insufficient schedulable control plane nodes",

	ClusterOperatorStabilityTimeout: 5 * time.Minute,
	AdmissionWaitTimeout:            5 * time.Minute,
	StatusConvergeTimeout:           5 * time.Minute,
	CompletionWaitTimeout:           45 * time.Minute,
	OperatorSettleTimeout:           20 * time.Minute,
}
