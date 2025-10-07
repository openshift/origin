package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineosbuilds,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2090
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +kubebuilder:metadata:labels=openshift.io/operator-managed=
// +kubebuilder:printcolumn:name="Prepared",type="string",JSONPath=.status.conditions[?(@.type=="Prepared")].status
// +kubebuilder:printcolumn:name="Building",type="string",JSONPath=.status.conditions[?(@.type=="Building")].status
// +kubebuilder:printcolumn:name="Succeeded",type="string",JSONPath=.status.conditions[?(@.type=="Succeeded")].status
// +kubebuilder:printcolumn:name="Interrupted",type="string",JSONPath=.status.conditions[?(@.type=="Interrupted")].status
// +kubebuilder:printcolumn:name="Failed",type="string",JSONPath=.status.conditions[?(@.type=="Failed")].status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MachineOSBuild describes a build process managed and deployed by the MCO
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type MachineOSBuild struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of the machine os build.
	// It is immutable once set.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="machineOSBuildSpec is immutable once set"
	// +required
	Spec MachineOSBuildSpec `json:"spec"`

	// status describes the last observed state of this machine os build.
	// +optional
	Status MachineOSBuildStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineOSBuildList describes all of the Builds on the system
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type MachineOSBuildList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items contains a collection of MachineOSBuild resources.
	// +optional
	Items []MachineOSBuild `json:"items,omitempty"`
}

// MachineOSBuildSpec describes information about a build process primarily populated from a MachineOSConfig object.
type MachineOSBuildSpec struct {
	// machineConfig points to the rendered MachineConfig resource to be included in this image build.
	// +required
	MachineConfig MachineConfigReference `json:"machineConfig"`
	// machineOSConfig references the MachineOSConfig resource that this image build extends.
	// +required
	MachineOSConfig MachineOSConfigReference `json:"machineOSConfig"`
	// renderedImagePushSpec is set by the Machine Config Operator from the MachineOSConfig object this build is attached to.
	// This field describes the location of the final image, which will be pushed by the build once complete.
	// The format of the image push spec is: host[:port][/namespace]/name:<tag> or svc_name.namespace.svc[:port]/repository/name:<tag>.
	// The length of the push spec must be between 1 to 447 characters.
	// +required
	RenderedImagePushSpec ImageTagFormat `json:"renderedImagePushSpec"`
}

// MachineOSBuildStatus describes the state of a build and other helpful information.
// +kubebuilder:validation:XValidation:rule="has(self.buildEnd) ? has(self.buildStart) && timestamp(self.buildStart) < timestamp(self.buildEnd) : true",message="buildEnd must be after buildStart"
type MachineOSBuildStatus struct {
	// conditions are state related conditions for the build. Valid types are:
	// Prepared, Building, Failed, Interrupted, and Succeeded.
	// Once a Build is marked as Failed, Interrupted or Succeeded, no future conditions can be set.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:XValidation:rule="oldSelf.exists(x, x.type=='Failed' && x.status=='True') ? self==oldSelf : true",message="once a Failed condition is set, conditions are immutable"
	// +kubebuilder:validation:XValidation:rule="oldSelf.exists(x, x.type=='Interrupted' && x.status=='True') ? self==oldSelf : true",message="once an Interrupted condition is set, conditions are immutable"
	// +kubebuilder:validation:XValidation:rule="oldSelf.exists(x, x.type=='Succeeded' && x.status=='True') ? self==oldSelf : true",message="once an Succeeded condition is set, conditions are immutable"
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// builder describes the image builder backend used for this build.
	// +optional
	Builder *MachineOSBuilderReference `json:"builder,omitempty"`
	// relatedObjects is a list of references to ephemeral objects such as ConfigMaps or Secrets that are meant to be consumed while the build process runs.
	// After a successful build or when this MachineOSBuild is deleted, these ephemeral objects will be removed.
	// In the event of a failed build, the objects will remain until the build is removed to allow for inspection.
	// +kubebuilder:validation:MaxItems=10
	// +listType=map
	// +listMapKey=name
	// +listMapKey=resource
	// +optional
	RelatedObjects []ObjectReference `json:"relatedObjects,omitempty"`
	// buildStart is the timestamp corresponding to the build controller initiating the build backend for this MachineOSBuild.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="buildStart is immutable once set"
	// +optional
	BuildStart *metav1.Time `json:"buildStart,omitempty"`
	// buildEnd is the timestamp corresponding to completion of the builder backend.
	// When omitted the build has either not been started, or is in progress.
	// It will be populated once the build completes, fails or is interrupted.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="buildEnd is immutable once set"
	// +optional
	BuildEnd *metav1.Time `json:"buildEnd,omitempty"`
	// digestedImagePushSpec describes the fully qualified push spec produced by this build.
	// The format of the push spec is: host[:port][/namespace]/name@sha256:<digest>,
	// where the digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the whole spec must be between 1 to 447 characters.
	// +optional
	DigestedImagePushSpec ImageDigestFormat `json:"digestedImagePushSpec,omitempty"`
}

// MachineOSBuilderReference describes which ImageBuilder backend to use for this build
// +union
// +kubebuilder:validation:XValidation:rule="has(self.imageBuilderType) && self.imageBuilderType == 'Job' ?  has(self.job) : !has(self.job)",message="job is required when imageBuilderType is Job, and forbidden otherwise"
type MachineOSBuilderReference struct {
	// imageBuilderType describes the type of image builder used to build this image.
	// Valid values are Job only.
	// When set to Job, a pod based builder, using buildah, is launched to build the specified image.
	// +unionDiscriminator
	// +required
	ImageBuilderType MachineOSImageBuilderType `json:"imageBuilderType"`

	// job is a reference to the job object that is managing the image build.
	// This is required if the imageBuilderType is Job, and forbidden otherwise.
	// +unionMember
	// +optional
	Job *ObjectReference `json:"job,omitempty"`
}

// BuildProgess highlights some of the key phases of a build to be tracked in Conditions.
type BuildProgress string

const (
	// prepared indicates that the build has finished preparing. A build is prepared
	// by gathering the build inputs, validating them, and making sure we can do an update as specified.
	MachineOSBuildPrepared BuildProgress = "Prepared"
	// building indicates that the build has been kicked off with the specified image builder
	MachineOSBuilding BuildProgress = "Building"
	// failed indicates that during the build or preparation process, the build failed.
	MachineOSBuildFailed BuildProgress = "Failed"
	// interrupted indicates that the user stopped the build process by modifying part of the build config
	MachineOSBuildInterrupted BuildProgress = "Interrupted"
	// succeeded indicates that the build has completed and the image is ready to roll out.
	MachineOSBuildSucceeded BuildProgress = "Succeeded"
)

// Refers to the name of a rendered MachineConfig (e.g., "rendered-worker-ec40d2965ff81bce7cd7a7e82a680739", etc.):
// the build targets this MachineConfig, this is often used to tell us whether we need an update.
type MachineConfigReference struct {
	// name is the name of the rendered MachineConfig object.
	// This value should be between 10 and 253 characters, and must contain only lowercase
	// alphanumeric characters, hyphens and periods, and should start and end with an alphanumeric character.
	// +kubebuilder:validation:MinLength:=10
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Name string `json:"name"`
}

// ObjectReference contains enough information to let you inspect or modify the referred object.
type ObjectReference struct {
	// group of the referent.
	// The name must contain only lowercase alphanumeric characters, '-' or '.' and start/end with an alphanumeric character.
	// Example: "", "apps", "build.openshift.io", etc.
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +kubebuilder:validation:MaxLength:=253
	// +required
	Group string `json:"group"`
	// resource of the referent.
	// This value should consist of at most 63 characters, and of only lowercase alphanumeric characters and hyphens,
	// and should start with an alphabetic character and end with an alphanumeric character.
	// Example: "deployments", "deploymentconfigs", "pods", etc.
	// +required
	// +kubebuilder:validation:XValidation:rule=`!format.dns1035Label().validate(self).hasValue()`,message="a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Resource string `json:"resource"`
	// namespace of the referent.
	// This value should consist of at most 63 characters, and of only lowercase alphanumeric characters and hyphens,
	// and should start and end with an alphanumeric character.
	// +kubebuilder:validation:XValidation:rule=`!format.dns1123Label().validate(self).hasValue()`,message="the value must consist of only lowercase alphanumeric characters and hyphens"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// name of the referent.
	// The name must contain only lowercase alphanumeric characters, '-' or '.' and start/end with an alphanumeric character.
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`
}

// MachineOSConfigReference refers to the MachineOSConfig this build is based off of
type MachineOSConfigReference struct {
	// name of the MachineOSConfig.
	// The name must contain only lowercase alphanumeric characters, '-' or '.' and start/end with an alphanumeric character.
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`
}
