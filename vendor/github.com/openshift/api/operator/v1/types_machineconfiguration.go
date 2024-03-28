package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineconfigurations,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1453
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01

// MachineConfiguration provides information to configure an operator to manage Machine Configuration.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type MachineConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// spec is the specification of the desired behavior of the Machine Config Operator
	// +kubebuilder:validation:Required
	Spec MachineConfigurationSpec `json:"spec"`

	// status is the most recently observed status of the Machine Config Operator
	// +optional
	Status MachineConfigurationStatus `json:"status"`
}

type MachineConfigurationSpec struct {
	StaticPodOperatorSpec `json:",inline"`

	// TODO(jkyros): This is where we put our knobs and dials

	// managedBootImages allows configuration for the management of boot images for machine
	// resources within the cluster. This configuration allows users to select resources that should
	// be updated to the latest boot images during cluster upgrades, ensuring that new machines
	// always boot with the current cluster version's boot image. When omitted, no boot images
	// will be updated.
	// +openshift:enable:FeatureGate=ManagedBootImages
	// +optional
	ManagedBootImages ManagedBootImages `json:"managedBootImages"`
}

type MachineConfigurationStatus struct {
	StaticPodOperatorStatus `json:",inline"`

	// TODO(jkyros): This is where we can put additional bespoke status fields
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineConfigurationList is a collection of items
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type MachineConfigurationList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	// Items contains the items
	Items []MachineConfiguration `json:"items"`
}

type ManagedBootImages struct {
	// machineManagers can be used to register machine management resources for boot image updates. The Machine Config Operator
	// will watch for changes to this list. Only one entry is permitted per type of machine management resource.
	// +optional
	// +listType=map
	// +listMapKey=resource
	// +listMapKey=apiGroup
	MachineManagers []MachineManager `json:"machineManagers"`
}

// MachineManager describes a target machine resource that is registered for boot image updates. It stores identifying information
// such as the resource type and the API Group of the resource. It also provides granular control via the selection field.
type MachineManager struct {
	// resource is the machine management resource's type.
	// The only current valid value is machinesets.
	// machinesets means that the machine manager will only register resources of the kind MachineSet.
	// +kubebuilder:validation:Required
	Resource MachineManagerMachineSetsResourceType `json:"resource"`

	// apiGroup is name of the APIGroup that the machine management resource belongs to.
	// The only current valid value is machine.openshift.io.
	// machine.openshift.io means that the machine manager will only register resources that belong to OpenShift machine API group.
	// +kubebuilder:validation:Required
	APIGroup MachineManagerMachineSetsAPIGroupType `json:"apiGroup"`

	// selection allows granular control of the machine management resources that will be registered for boot image updates.
	// +kubebuilder:validation:Required
	Selection MachineManagerSelector `json:"selection"`
}

// +kubebuilder:validation:XValidation:rule="has(self.mode) && self.mode == 'Partial' ?  has(self.partial) : !has(self.partial)",message="Partial is required when type is partial, and forbidden otherwise"
// +union
type MachineManagerSelector struct {
	// mode determines how machine managers will be selected for updates.
	// Valid values are All and Partial.
	// All means that every resource matched by the machine manager will be updated.
	// Partial requires specified selector(s) and allows customisation of which resources matched by the machine manager will be updated.
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	Mode MachineManagerSelectorMode `json:"mode"`

	// partial provides label selector(s) that can be used to match machine management resources.
	// Only permitted when mode is set to "Partial".
	// +optional
	Partial *PartialSelector `json:"partial,omitempty"`
}

// PartialSelector provides label selector(s) that can be used to match machine management resources.
type PartialSelector struct {
	// machineResourceSelector is a label selector that can be used to select machine resources like MachineSets.
	// +kubebuilder:validation:Required
	MachineResourceSelector *metav1.LabelSelector `json:"machineResourceSelector,omitempty"`
}

// MachineManagerSelectorMode is a string enum used in the MachineManagerSelector union discriminator.
// +kubebuilder:validation:Enum:="All";"Partial"
type MachineManagerSelectorMode string

const (
	// All represents a configuration mode that registers all resources specified by the parent MachineManager for boot image updates.
	All MachineManagerSelectorMode = "All"

	// Partial represents a configuration mode that will register resources specified by the parent MachineManager only
	// if they match with the label selector.
	Partial MachineManagerSelectorMode = "Partial"
)

// MachineManagerManagedResourceType is a string enum used in the MachineManager type to describe the resource
// type to be registered.
// +kubebuilder:validation:Enum:="machinesets"
type MachineManagerMachineSetsResourceType string

const (
	// MachineSets represent the MachineSet resource type, which manage a group of machines and belong to the Openshift machine API group.
	MachineSets MachineManagerMachineSetsResourceType = "machinesets"
)

// MachineManagerManagedAPIGroupType is a string enum used in in the MachineManager type to describe the APIGroup
// of the resource type being registered.
// +kubebuilder:validation:Enum:="machine.openshift.io"
type MachineManagerMachineSetsAPIGroupType string

const (
	// MachineAPI represent the traditional MAPI Group that a machineset may belong to.
	// This feature only supports MAPI machinesets at this time.
	MachineAPI MachineManagerMachineSetsAPIGroupType = "machine.openshift.io"
)
