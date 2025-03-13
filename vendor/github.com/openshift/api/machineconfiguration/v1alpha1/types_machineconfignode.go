package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineconfignodes,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2256
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +openshift:enable:FeatureGate=MachineConfigNodes
// +kubebuilder:printcolumn:name="PoolName",type="string",JSONPath=.spec.pool.name,priority=0
// +kubebuilder:printcolumn:name="DesiredConfig",type="string",JSONPath=.spec.configVersion.desired,priority=0
// +kubebuilder:printcolumn:name="CurrentConfig",type="string",JSONPath=.status.configVersion.current,priority=0
// +kubebuilder:printcolumn:name="Updated",type="string",JSONPath=.status.conditions[?(@.type=="Updated")].status,priority=0
// +kubebuilder:printcolumn:name="UpdatePrepared",type="string",JSONPath=.status.conditions[?(@.type=="UpdatePrepared")].status,priority=1
// +kubebuilder:printcolumn:name="UpdateExecuted",type="string",JSONPath=.status.conditions[?(@.type=="UpdateExecuted")].status,priority=1
// +kubebuilder:printcolumn:name="UpdatePostActionComplete",type="string",JSONPath=.status.conditions[?(@.type=="UpdatePostActionComplete")].status,priority=1
// +kubebuilder:printcolumn:name="UpdateComplete",type="string",JSONPath=.status.conditions[?(@.type=="UpdateComplete")].status,priority=1
// +kubebuilder:printcolumn:name="Resumed",type="string",JSONPath=.status.conditions[?(@.type=="Resumed")].status,priority=1
// +kubebuilder:printcolumn:name="UpdatedFilesAndOS",type="string",JSONPath=.status.conditions[?(@.type=="AppliedFilesAndOS")].status,priority=1
// +kubebuilder:printcolumn:name="CordonedNode",type="string",JSONPath=.status.conditions[?(@.type=="Cordoned")].status,priority=1
// +kubebuilder:printcolumn:name="DrainedNode",type="string",JSONPath=.status.conditions[?(@.type=="Drained")].status,priority=1
// +kubebuilder:printcolumn:name="RebootedNode",type="string",JSONPath=.status.conditions[?(@.type=="RebootedNode")].status,priority=1
// +kubebuilder:printcolumn:name="UncordonedNode",type="string",JSONPath=.status.conditions[?(@.type=="Uncordoned")].status,priority=1
// +kubebuilder:metadata:labels=openshift.io/operator-managed=

// MachineConfigNode describes the health of the Machines on the system
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:validation:XValidation:rule="self.metadata.name == self.spec.node.name",message="spec.node.name should match metadata.name"
type MachineConfigNode struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of the machine config node.
	// +required
	Spec MachineConfigNodeSpec `json:"spec"`

	// status describes the last observed state of this machine config node.
	// +optional
	Status MachineConfigNodeStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineConfigNodeList describes all of the MachinesStates on the system
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type MachineConfigNodeList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata"`

	// items contains a collection of MachineConfigNode resources.
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Items []MachineConfigNode `json:"items"`
}

// MCOObjectReference holds information about an object the MCO either owns
// or modifies in some way
type MCOObjectReference struct {
	// name is the name of the object being referenced. For example, this can represent a machine
	// config pool or node name.
	// Must be a lowercase RFC-1123 subdomain name (https://tools.ietf.org/html/rfc1123) consisting
	// of only lowercase alphanumeric characters, hyphens (-), and periods (.), and must start and end
	// with an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Name string `json:"name"`
}

// MachineConfigNodeSpec describes the MachineConfigNode we are managing.
type MachineConfigNodeSpec struct {
	// node contains a reference to the node for this machine config node.
	// +required
	Node MCOObjectReference `json:"node"`

	// pool contains a reference to the machine config pool that this machine config node's
	// referenced node belongs to.
	// +required
	Pool MCOObjectReference `json:"pool"`

	// configVersion holds the desired config version for the node targeted by this machine config node resource.
	// The desired version represents the machine config the node will attempt to update to and gets set before the machine config operator validates
	// the new machine config against the current machine config.
	// +required
	ConfigVersion MachineConfigNodeSpecMachineConfigVersion `json:"configVersion"`

	// pinnedImageSets is a user defined value that holds the names of the desired image sets that the node should pull and pin.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	// Tombstone: Functionality to correctly and consistely populate this field was not implemented in the MCO, so
	// when applying a PIS, this field is not being updated. Since this field is not being used, it is being removed
	// before this API is GAed.
	// PinnedImageSets []MachineConfigNodeSpecPinnedImageSet `json:"pinnedImageSets,omitempty"`
}

// MachineConfigNodeStatus holds the reported information on a particular machine config node.
type MachineConfigNodeStatus struct {
	// conditions represent the observations of a machine config node's current state.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=20
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// observedGeneration represents the generation of the MachineConfigNode object observed by the Machine Config Operator's controller.
	// This field is updated when the controller observes a change to the desiredConfig in the configVersion of the machine config node spec.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="observedGeneration must not decrease"
	// +kubebuilder:validation:Minimum=0
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// configVersion describes the current and desired machine config version for this node.
	// +required
	ConfigVersion MachineConfigNodeStatusMachineConfigVersion `json:"configVersion"`
	// pinnedImageSets describes the current and desired pinned image sets for this node.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	PinnedImageSets []MachineConfigNodeStatusPinnedImageSet `json:"pinnedImageSets,omitempty"`
}

// MachineConfigNodeStatusPinnedImageSet holds information about the current, desired, and failed pinned image sets for the observed machine config node.
// +kubebuilder:validation:XValidation:rule="has(self.desiredGeneration) && has(self.currentGeneration) ? self.desiredGeneration >= self.currentGeneration : true",message="desired generation must be greater than or equal to the current generation"
// +kubebuilder:validation:XValidation:rule="has(self.lastFailedGeneration) && has(self.desiredGeneration) ? self.desiredGeneration >= self.lastFailedGeneration : true",message="desired generation must be greater than or equal to the last failed generation"
// +kubebuilder:validation:XValidation:rule="has(self.lastFailedGeneration) ? has(self.lastFailedGenerationError) : true",message="last failed generation error must be defined on image pull and pin failure"
type MachineConfigNodeStatusPinnedImageSet struct {
	// name is the name of the pinned image set.
	// Must be a lowercase RFC-1123 subdomain name (https://tools.ietf.org/html/rfc1123) consisting
	// of only lowercase alphanumeric characters, hyphens (-), and periods (.), and must start and end
	// with an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Name string `json:"name"`
	// currentGeneration is the generation of the pinned image set that has most recently been successfully pulled and pinned on this node.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="currentGeneration must not decrease"
	// +kubebuilder:validation:Minimum=0
	// +optional
	CurrentGeneration int32 `json:"currentGeneration,omitempty"`
	// desiredGeneration is the generation of the pinned image set that is targeted to be pulled and pinned on this node.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="desiredGeneration must not decrease"
	// +kubebuilder:validation:Minimum=0
	// +optional
	DesiredGeneration int32 `json:"desiredGeneration,omitempty"`
	// lastFailedGeneration is the generation of the most recent pinned image set that failed to be pulled and pinned on this node.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="lastFailedGeneration must not decrease"
	// +kubebuilder:validation:Minimum=0
	// +optional
	LastFailedGeneration int32 `json:"lastFailedGeneration,omitempty"`
	// lastFailedGenerationError is the error explaining why the desired images failed to be pulled and pinned.
	// The error is an empty string if the image pull and pin is successful.
	// +kubebuilder:validation:MaxLength=32768
	// +optional
	LastFailedGenerationError string `json:"lastFailedGenerationError,omitempty"`
	// Previously, failures associated with pinning and pulling images where shared in a list of strings under `LastFailedGenerationErrors`.
	// This field is being removed and a `LastFailedGenerationError` field of type string is being added in its place as this field will
	// contain a single error and there is no need for a list anymore.
	// Tombstone: legacy field no longer needed
	// LastFailedGenerationErrors []string `json:"lastFailedGenerationErrors,omitempty"`
}

// MachineConfigNodeStatusMachineConfigVersion holds the current and desired config versions as last updated in the MCN status.
// When the current and desired versions do not match, the machine config pool is processing an upgrade and the machine config node will
// monitor the upgrade process.
// When the current and desired versions do match, the machine config node will ignore these events given that certain operations
// happen both during the MCO's upgrade mode and the daily operations mode.
type MachineConfigNodeStatusMachineConfigVersion struct {
	// current is the name of the machine config currently in use on the node.
	// This value is updated once the machine config daemon has completed the update of the configuration for the node.
	// This value should match the desired version unless an upgrade is in progress.
	// Must be a lowercase RFC-1123 subdomain name (https://tools.ietf.org/html/rfc1123) consisting
	// of only lowercase alphanumeric characters, hyphens (-), and periods (.), and must start and end
	// with an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +optional
	Current string `json:"current"`
	// desired is the MachineConfig the node wants to upgrade to.
	// This value gets set in the machine config node status once the machine config has been validated
	// against the current machine config.
	// Must be a lowercase RFC-1123 subdomain name (https://tools.ietf.org/html/rfc1123) consisting
	// of only lowercase alphanumeric characters, hyphens (-), and periods (.), and must start and end
	// with an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Desired string `json:"desired"`
}

// MachineConfigNodeSpecMachineConfigVersion holds the desired config version for the current observed machine config node.
// When Current is not equal to Desired, the MachineConfigOperator is in an upgrade phase and the machine config node will
// take account of upgrade related events. Otherwise, they will be ignored given that certain operations
// happen both during the MCO's upgrade mode and the daily operations mode.
type MachineConfigNodeSpecMachineConfigVersion struct {
	// desired is the name of the machine config that the the node should be upgraded to.
	// This value is set when the machine config pool generates a new version of its rendered configuration.
	// When this value is changed, the machine config daemon starts the node upgrade process.
	// This value gets set in the machine config node spec once the machine config has been targeted for upgrade and before it is validated.
	// Must be a lowercase RFC-1123 subdomain name (https://tools.ietf.org/html/rfc1123) consisting
	// of only lowercase alphanumeric characters, hyphens (-), and periods (.), and must start and end
	// with an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Desired string `json:"desired"`
}

// Tombstone: This struct defines the type of `Spec.PinnedImageSets`, which is being removed. Therefore, this field
// is also being tombstoned.
// MachineConfigNodeSpecPinnedImageSet holds information on the desired pinned image sets that the current observed machine config node
// should pin and pull.
// type MachineConfigNodeSpecPinnedImageSet struct {
// 	// name is the name of the pinned image set.
// 	// Must be a lowercase RFC-1123 subdomain name (https://tools.ietf.org/html/rfc1123) consisting
// 	// of only lowercase alphanumeric characters, hyphens (-), and periods (.), and must start and end
// 	// with an alphanumeric character, and be at most 253 characters in length.
// 	// +kubebuilder:validation:MaxLength:=253
// 	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
// 	// +required
// 	Name string `json:"name"`
// }

// StateProgress is each possible state for each possible MachineConfigNodeType
// Please note: These conditions are subject to change. Both additions and deletions may be made.
// +enum
type StateProgress string

const (
	// MachineConfigNodeUpdatePrepared describes a machine that is preparing in the daemon to trigger an update
	MachineConfigNodeUpdatePrepared StateProgress = "UpdatePrepared"
	// MachineConfigNodeUpdateExecuted describes a machine that has executed the body of the upgrade
	MachineConfigNodeUpdateExecuted StateProgress = "UpdateExecuted"
	// MachineConfigNodeUpdatePostActionComplete describes a machine that has executed its post update action
	MachineConfigNodeUpdatePostActionComplete StateProgress = "UpdatePostActionComplete"
	// MachineConfigNodeUpdateComplete describes a machine that has completed the core parts of an upgrade
	MachineConfigNodeUpdateComplete StateProgress = "UpdateComplete"
	// MachineConfigNodeUpdated describes a machine that is fully updated and has a matching desired and current config
	MachineConfigNodeUpdated StateProgress = "Updated"
	// MachineConfigNodeUpdateResumed describes a machine that has resumed normal processes
	MachineConfigNodeResumed StateProgress = "Resumed"
	// MachineConfigNodeUpdateDrained describes the part of the in progress phase where the node drains
	MachineConfigNodeUpdateDrained StateProgress = "Drained"
	// MachineConfigNodeUpdateFilesAndOS describes the part of the in progress phase where the nodes files and OS config change
	MachineConfigNodeUpdateFilesAndOS StateProgress = "AppliedFilesAndOS"
	// MachineConfigNodeUpdateCordoned describes the part of the in progress phase where the node cordons
	MachineConfigNodeUpdateCordoned StateProgress = "Cordoned"
	// MachineConfigNodeUpdateUncordoned describes the part of the completing phase where the node uncordons
	MachineConfigNodeUpdateUncordoned StateProgress = "Uncordoned"
	// MachineConfigNodeUpdateRebooted describes the part of the post action phase where the node reboots itself
	MachineConfigNodeUpdateRebooted StateProgress = "RebootedNode"
	// MachineConfigNodeNodeDegraded describes a machine that has failed to update to the desired machine config and is in a degraded state
	MachineConfigNodeNodeDegraded StateProgress = "NodeDegraded"
	// MachineConfigNodePinnedImageSetsProgressing describes a machine currently progressing to the desired pinned image sets
	MachineConfigNodePinnedImageSetsProgressing StateProgress = "PinnedImageSetsProgressing"
	// MachineConfigNodePinnedImageSetsDegraded describes a machine that has failed to progress to the desired pinned image sets
	MachineConfigNodePinnedImageSetsDegraded StateProgress = "PinnedImageSetsDegraded"
)
