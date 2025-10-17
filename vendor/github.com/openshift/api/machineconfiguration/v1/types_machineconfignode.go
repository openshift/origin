package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineconfignodes,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2255
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +openshift:enable:FeatureGate=MachineConfigNodes
// +kubebuilder:printcolumn:name="PoolName",type="string",JSONPath=.spec.pool.name,priority=0
// +kubebuilder:printcolumn:name="DesiredConfig",type="string",JSONPath=.spec.configVersion.desired,priority=0
// +kubebuilder:printcolumn:name="CurrentConfig",type="string",JSONPath=.status.configVersion.current,priority=0
// +kubebuilder:printcolumn:name="Updated",type="string",JSONPath=.status.conditions[?(@.type=="Updated")].status,priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",priority=0
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
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
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
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
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

	// configImage is an optional field for configuring the OS image to be used for this node. This field will only exist if the node belongs to a pool opted into on-cluster image builds, and will override any MachineConfig referenced OSImageURL fields
	// When omitted, Image Mode is not be enabled and the node will follow the standard update process of creating a rendered MachineConfig and updating to its specifications.
	// When specified, Image Mode is enabled and will attempt to update the node to use the desired image. Following this, the node will follow the standard update process of creating a rendered MachineConfig and updating to its specifications.
	// +openshift:enable:FeatureGate=ImageModeStatusReporting
	// +optional
	ConfigImage MachineConfigNodeSpecConfigImage `json:"configImage,omitempty,omitzero"`
}

// MachineConfigNodeStatus holds the reported information on a particular machine config node.
type MachineConfigNodeStatus struct {
	// conditions represent the observations of a machine config node's current state. Valid types are:
	// UpdatePrepared, UpdateExecuted, UpdatePostActionComplete, UpdateComplete, Updated, Resumed,
	// Drained, AppliedFilesAndOS, Cordoned, Uncordoned, RebootedNode, NodeDegraded, PinnedImageSetsProgressing,
	// and PinnedImageSetsDegraded.
	// The following types are only available when the ImageModeStatusReporting feature gate is enabled: ImagePulledFromRegistry,
	// AppliedOSImage, AppliedFiles
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=20
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// observedGeneration represents the generation of the MachineConfigNode object observed by the Machine Config Operator's controller.
	// This field is updated when the controller observes a change to the desiredConfig in the configVersion of the machine config node spec.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="observedGeneration must not decrease"
	// +kubebuilder:validation:Minimum=1
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// configVersion describes the current and desired machine config version for this node.
	// +optional
	ConfigVersion *MachineConfigNodeStatusMachineConfigVersion `json:"configVersion,omitempty"`
	// configImage is an optional field for configuring the OS image to be used for this node. This field will only exist if the node belongs to a pool opted into on-cluster image builds, and will override any MachineConfig referenced OSImageURL fields.
	// When omitted, this means that the Image Mode feature is not being used and the node will be up to date with the specific current rendered config version for the nodes MachinePool.
	// When specified, the Image Mode feature is enabled and the contents of this field show the observed state of the node image.
	// When Image Mode is enabled and a new MachineConfig is applied such that a new OS image build is not created, only the configVersion field will change.
	// When Image Mode is enabled and a new MachineConfig is applied such that a new OS image build is created, then only the configImage field will change. It is also possible that both the configImage
	// and configVersion change during the same update.
	// +openshift:enable:FeatureGate=ImageModeStatusReporting
	// +optional
	ConfigImage MachineConfigNodeStatusConfigImage `json:"configImage,omitempty,omitzero"`
	// pinnedImageSets describes the current and desired pinned image sets for this node.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	PinnedImageSets []MachineConfigNodeStatusPinnedImageSet `json:"pinnedImageSets,omitempty"`
	// irreconcilableChanges is an optional field that contains the observed differences between this nodes
	// configuration and the target rendered MachineConfig.
	// This field will be set when there are changes to the target rendered MachineConfig that can only be applied to
	// new nodes joining the cluster.
	// Entries must be unique, keyed on the fieldPath field.
	// Must not exceed 32 entries.
	// +listType=map
	// +listMapKey=fieldPath
	// +openshift:enable:FeatureGate=IrreconcilableMachineConfig
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	// +optional
	IrreconcilableChanges []IrreconcilableChangeDiff `json:"irreconcilableChanges,omitempty"`
}

// IrreconcilableChangeDiff holds an individual diff between the initial install-time MachineConfig
// and the latest applied one caused by the presence of irreconcilable changes.
type IrreconcilableChangeDiff struct {
	// fieldPath is a required reference to the path in the latest rendered MachineConfig that differs from this nodes
	// configuration.
	// Must not be empty and must not exceed 70 characters in length.
	// Must begin with the prefix 'spec.' and only contain alphanumeric characters, square brackets ('[]'), or dots ('.').
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=70
	// +kubebuilder:validation:XValidation:rule="self.startsWith('spec.')",message="The fieldPath must start with `spec.`"
	// +kubebuilder:validation:XValidation:rule=`self.matches('^[\\da-zA-Z\\.\\[\\]]+$')`,message="The fieldPath must consist only of alphanumeric characters, brackets [] and dots ('.')."
	FieldPath string `json:"fieldPath,omitempty"`
	// diff is a required field containing the difference between the nodes current configuration and the latest
	// rendered MachineConfig for the field specified in fieldPath.
	// Must not be an empty string and must not exceed 4096 characters in length.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Diff string `json:"diff,omitempty"`
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
	// +kubebuilder:validation:Minimum=1
	// +optional
	CurrentGeneration int32 `json:"currentGeneration,omitempty"`
	// desiredGeneration is the generation of the pinned image set that is targeted to be pulled and pinned on this node.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="desiredGeneration must not decrease"
	// +kubebuilder:validation:Minimum=1
	// +optional
	DesiredGeneration int32 `json:"desiredGeneration,omitempty"`
	// lastFailedGeneration is the generation of the most recent pinned image set that failed to be pulled and pinned on this node.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="lastFailedGeneration must not decrease"
	// +kubebuilder:validation:Minimum=1
	// +optional
	LastFailedGeneration int32 `json:"lastFailedGeneration,omitempty"`
	// lastFailedGenerationError is the error explaining why the desired images failed to be pulled and pinned.
	// The error is an empty string if the image pull and pin is successful.
	// +kubebuilder:validation:MaxLength=32768
	// +optional
	LastFailedGenerationError string `json:"lastFailedGenerationError,omitempty"`
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

// MachineConfigNodeSpecConfigImage holds the desired image for the node.
// This structure is populated from the `machineconfiguration.openshift.io/desiredImage`
// annotation on the target node, which is set by the Machine Config Pool controller
// to signal the desired image pullspec for the node to update to.
type MachineConfigNodeSpecConfigImage struct {
	// desiredImage is a required field that configures the image that the node should be updated to use.
	// It must be a fully qualified OCI image pull spec of the format host[:port][/namespace]/name@sha256:, where the digest must be exactly 64 characters in length and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// desiredImage must not be an empty string and must not exceed 447 characters in length.
	// +required
	DesiredImage ImageDigestFormat `json:"desiredImage,omitempty"`
}

// MachineConfigNodeStatusConfigImage holds the observed state of the image
// on the node, including both the image targeted for an update and the image
// currently applied. This allows for monitoring the progress of the layering
// rollout. If Image Mode is enabled, desiredImage must be defined.
// +kubebuilder:validation:MinProperties:=1
type MachineConfigNodeStatusConfigImage struct {
	// currentImage is an optional field that represents the current image that is applied to the node.
	// When omitted, this means that no image updates have been applied to the node and it will be up to date with the specific current rendered config version.
	// When specified, this means that the node is currently using this image.
	// currentImage must be a fully qualified OCI image pull spec of the format host[:port][/namespace]/name@sha256:, where the digest must be exactly 64 characters in length and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// currentImage must not be an empty string and must not exceed 447 characters in length.
	// +optional
	CurrentImage ImageDigestFormat `json:"currentImage,omitzero,omitempty"`
	// desiredImage is an optional field that represents the currently observed state of image that the node should be updated to use.
	// When not specified, this means that Image Mode has been disabled and the node will up to date with the specific current rendered config version.
	// When specified, this means that Image Mode has been enabled and the node is actively progressing to update the node to this image.
	// If currentImage and desiredImage match, the node has been successfully updated to use the desired image.
	// desiredImage must be a fully qualified OCI image pull spec of the format host[:port][/namespace]/name@sha256:, where the digest must be exactly 64 characters in length and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// desiredImage must not be an empty string and must not exceed 447 characters in length.
	// +optional
	DesiredImage ImageDigestFormat `json:"desiredImage,omitzero,omitempty"`
}

// StateProgress is each possible state for each possible MachineConfigNodeType
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
	// MachineConfigNodeUpdateFiles describes the part of the in progress phase where the nodes files changes
	MachineConfigNodeUpdateFiles StateProgress = "AppliedFiles"
	// MachineConfigNodeUpdateOS describes the part of the in progress phase where the OS config changes
	MachineConfigNodeUpdateOS StateProgress = "AppliedOSImage"
	// MachineConfigNodeUpdateFilesAndOS describes the part of the in progress phase where the nodes files and OS config change
	MachineConfigNodeUpdateFilesAndOS StateProgress = "AppliedFilesAndOS"
	// MachineConfigNodeImagePulledFromRegistry describes the part of the in progress phase where the update image is pulled from the registry
	MachineConfigNodeImagePulledFromRegistry StateProgress = "ImagePulledFromRegistry"
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
