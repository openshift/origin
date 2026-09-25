package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyState describes the control-plane and infrastructure topology at one
// end of a topology transition.
// The topology controller determines which transitions are supported. Currently,
// it supports only transitions that change both topologies from SingleReplica to
// HighlyAvailable. Representing a topology here does not enable a transition to it.
type TopologyState struct {
	// controlPlaneTopology is the topology of the control-plane nodes. Valid values
	// are HighlyAvailable, HighlyAvailableArbiter, and SingleReplica.
	// External is not valid: transitions cannot involve an externally hosted control plane.
	// SingleReplica means a single instance of control-plane services is expected
	// to meet cluster needs. HighlyAvailable means multiple instances are expected
	// to provide redundancy. HighlyAvailableArbiter means two control-plane nodes
	// and a smaller arbiter node to maintain quorum.
	// See https://pkg.go.dev/github.com/openshift/api/config/v1#TopologyMode for
	// topology definitions.
	// controlPlaneTopology is required.
	// +kubebuilder:validation:Enum=HighlyAvailable;HighlyAvailableArbiter;SingleReplica
	// +required
	ControlPlaneTopology TopologyMode `json:"controlPlaneTopology,omitempty"`

	// infrastructureTopology is the topology of infrastructure services. Valid
	// values are SingleReplica and HighlyAvailable.
	// SingleReplica means operators expect a single instance of infrastructure
	// services to meet cluster needs.
	// HighlyAvailable means operators expect multiple instances of infrastructure
	// services to provide redundancy.
	// infrastructureTopology is required.
	// +kubebuilder:validation:Enum=SingleReplica;HighlyAvailable
	// +required
	InfrastructureTopology TopologyMode `json:"infrastructureTopology,omitempty"`
}

const (
	// TopologyTransitionsEvaluatedConditionType indicates whether supported transition types have been evaluated.
	//
	// The controller always reports this condition when topologyTransitionStatus is
	// set. It reports Unknown before evaluation and while checks are being refreshed,
	// True when evaluation finishes successfully (even if no transitions are supported
	// or available), and False when evaluation cannot finish. It retains the last
	// results while refreshing checks or reporting an evaluation failure. Clients
	// must treat those results as stale unless this condition is True.
	TopologyTransitionsEvaluatedConditionType = "TopologyTransitionsEvaluated"

	// TopologyTransitionCompletedConditionType indicates the status of the current or most recent transition.
	//
	// The controller always reports this condition when topologyTransitionStatus is
	// set. It reports Unknown before a transition is requested, True after both
	// topologies reach the requested transition's target and post-transition checks
	// pass, and False while the transition is in progress or blocked.
	// Reaching the target topology alone does not mean the transition is complete:
	// this condition remains False while post-transition checks are pending or failing.
	TopologyTransitionCompletedConditionType = "TopologyTransitionCompleted"

	// TopologyTransitionAvailableConditionType indicates if a type of transition is available based on
	// the most recently evaluated state of the cluster.
	//
	// TopologyTransitionAvailableConditionType is Unknown before evaluation, True when evaluations
	// for the given transition succeed, and False when one or more evaluations for a transition fail.
	// Because the controller clears transitions whenever TopologyTransitionsEvaluated is not True,
	// a reported value always reflects the current evaluation, never a stale one.
	TopologyTransitionAvailableConditionType = "TopologyTransitionAvailable"
)

// TopologyTransitionStatus reports availability of each type of topology transition and contains the
// status of any initiated transition.
// +kubebuilder:validation:MinProperties=1
type TopologyTransitionStatus struct {
	// conditions provides information on topology transition progress and the
	// evaluation of supported transition types. The controller always reports both
	// conditions once topologyTransitionStatus is set.
	//
	// Valid condition types are TopologyTransitionsEvaluated and
	// TopologyTransitionCompleted. Both conditions must be present: the controller
	// always reports each condition explicitly, using Unknown when a condition's
	// state is not yet known, rather than omitting it.
	//
	// +kubebuilder:validation:MinItems=2
	// +kubebuilder:validation:MaxItems=2
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type in ['TopologyTransitionsEvaluated', 'TopologyTransitionCompleted'])",message="conditions may only contain TopologyTransitionsEvaluated and TopologyTransitionCompleted"
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// transitions contains each supported transition type and its availability.
	// It is optional. The controller clears this list whenever
	// TopologyTransitionsEvaluated is not True, so its presence always reflects
	// current, trustworthy results: an omitted list while TopologyTransitionsEvaluated
	// is True means no supported transition options were found, and an omitted list
	// otherwise means no transition options have been reported yet.
	//
	// Between one and eight transition options must be present when the list is set.
	// This list reports transition options, not concurrent transitions. The topology
	// controller determines which transition options are supported.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	// +optional
	// +listType=atomic
	Transitions []TopologyTransition `json:"transitions,omitempty"`
}

type TopologyTransition struct {
	// source is the control-plane and infrastructure topology this transition was
	// evaluated from. It may differ from the current topology while status is
	// being refreshed. External control planes cannot be a transition source.
	// controlPlaneTopology is the topology of the control-plane nodes. Valid values
	// are HighlyAvailable, HighlyAvailableArbiter, and SingleReplica.
	// External is not valid: transitions cannot involve an externally hosted control plane.
	// SingleReplica means a single instance of control-plane services is expected
	// to meet cluster needs.
	// HighlyAvailable means multiple instances are expected to provide redundancy.
	// HighlyAvailableArbiter means two control-plane nodes and a smaller arbiter
	// node to maintain quorum. See
	// https://pkg.go.dev/github.com/openshift/api/config/v1#TopologyMode for topology definitions.
	// infrastructureTopology is the topology of infrastructure services. Valid
	// values are SingleReplica and HighlyAvailable.
	// SingleReplica means operators expect a single instance of infrastructure
	// services to meet cluster needs.
	// HighlyAvailable means operators expect multiple instances of infrastructure
	// services to provide redundancy.
	// source is required.
	// +required
	Source TopologyState `json:"source,omitempty,omitzero"`

	// target is the control-plane and infrastructure topology this transition would
	// move to.
	// controlPlaneTopology is the topology of the control-plane nodes. Valid values
	// are HighlyAvailable, HighlyAvailableArbiter, and SingleReplica.
	// External is not valid: transitions cannot involve an externally hosted control plane.
	// SingleReplica means a single instance of control-plane services is expected
	// to meet cluster needs.
	// HighlyAvailable means multiple instances are expected to provide redundancy.
	// HighlyAvailableArbiter means two control-plane nodes and a smaller arbiter
	// node to maintain quorum. See
	// https://pkg.go.dev/github.com/openshift/api/config/v1#TopologyMode for topology definitions.
	// infrastructureTopology is the topology of infrastructure services. Valid
	// values are SingleReplica and HighlyAvailable.
	// SingleReplica means operators expect a single instance of infrastructure
	// services to meet cluster needs.
	// HighlyAvailable means operators expect multiple instances of infrastructure
	// services to provide redundancy.
	// External control planes cannot be a transition target.
	// target is required.
	// +required
	Target TopologyState `json:"target,omitempty,omitzero"`

	// evaluations contains the availability condition for this transition and
	// conditions for the checks run against the cluster to determine availability.
	//
	// TopologyTransitionAvailable is required; other condition types report
	// individual checks. Between one and 32 conditions must be present, allowing
	// at most 31 individual checks in addition to the availability condition.
	// The controller defines individual check types, reasons, and messages, and
	// always reports each one explicitly, using Unknown when a check's result is
	// not yet known.
	//
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'TopologyTransitionAvailable')",message="evaluations must contain TopologyTransitionAvailable"
	// +required
	// +listType=map
	// +listMapKey=type
	Evaluations []metav1.Condition `json:"evaluations,omitempty"`
}
