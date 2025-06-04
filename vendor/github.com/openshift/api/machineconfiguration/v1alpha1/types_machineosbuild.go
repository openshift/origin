package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineosbuilds,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1773
// +openshift:enable:FeatureGate=OnClusterBuild
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +kubebuilder:metadata:labels=openshift.io/operator-managed=
// +kubebuilder:printcolumn:name="Prepared",type="string",JSONPath=.status.conditions[?(@.type=="Prepared")].status
// +kubebuilder:printcolumn:name="Building",type="string",JSONPath=.status.conditions[?(@.type=="Building")].status
// +kubebuilder:printcolumn:name="Succeeded",type="string",JSONPath=.status.conditions[?(@.type=="Succeeded")].status
// +kubebuilder:printcolumn:name="Interrupted",type="string",JSONPath=.status.conditions[?(@.type=="Interrupted")].status
// +kubebuilder:printcolumn:name="Failed",type="string",JSONPath=.status.conditions[?(@.type=="Failed")].status

// MachineOSBuild describes a build process managed and deployed by the MCO
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type MachineOSBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of the machine os build
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="machineOSBuildSpec is immutable once set"
	// +kubebuilder:validation:Required
	Spec MachineOSBuildSpec `json:"spec"`

	// status describes the lst observed state of this machine os build
	// +optional
	Status MachineOSBuildStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineOSBuildList describes all of the Builds on the system
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type MachineOSBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MachineOSBuild `json:"items"`
}

// MachineOSBuildSpec describes information about a build process primarily populated from a MachineOSConfig object.
type MachineOSBuildSpec struct {
	// configGeneration tracks which version of MachineOSConfig this build is based off of
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	ConfigGeneration int64 `json:"configGeneration"`
	// desiredConfig is the desired config we want to build an image for.
	// +kubebuilder:validation:Required
	DesiredConfig RenderedMachineConfigReference `json:"desiredConfig"`
	// machineOSConfig is the config object which the build is based off of
	// +kubebuilder:validation:Required
	MachineOSConfig MachineOSConfigReference `json:"machineOSConfig"`
	// version tracks the newest MachineOSBuild for each MachineOSConfig
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	Version int64 `json:"version"`
	// renderedImagePushspec is set from the MachineOSConfig
	// The format of the image pullspec is:
	// host[:port][/namespace]/name:<tag> or svc_name.namespace.svc[:port]/repository/name:<tag>
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`((self.split(':').size() == 2 && self.split(':')[1].matches('^([a-zA-Z0-9-./:])+$')) || self.matches('^[^.]+\\.[^.]+\\.svc:\\d+\\/[^\\/]+\\/[^\\/]+:[^\\/]+$'))`,message="the OCI Image reference must end with a valid :<tag>, where '<digest>' is 64 characters long and '<tag>' is any valid string  Or it must be a valid .svc followed by a port, repository, image name, and tag."
	// +kubebuilder:validation:XValidation:rule=`((self.split(':').size() == 2 && self.split(':')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$')) || self.matches('^[^.]+\\.[^.]+\\.svc:\\d+\\/[^\\/]+\\/[^\\/]+:[^\\/]+$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme. Or it must be a valid .svc followed by a port, repository, image name, and tag."
	// +kubebuilder:validation:Required
	RenderedImagePushspec string `json:"renderedImagePushspec"`
}

// MachineOSBuildStatus describes the state of a build and other helpful information.
type MachineOSBuildStatus struct {
	// conditions are state related conditions for the build. Valid types are:
	// Prepared, Building, Failed, Interrupted, and Succeeded
	// once a Build is marked as Failed, no future conditions can be set. This is enforced by the MCO.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// ImageBuilderType describes the image builder set in the MachineOSConfig
	// +optional
	BuilderReference *MachineOSBuilderReference `json:"builderReference"`
	// relatedObjects is a list of objects that are related to the build process.
	RelatedObjects []ObjectReference `json:"relatedObjects,omitempty"`
	// buildStart describes when the build started.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="buildStart is immutable once set"
	// +kubebuilder:validation:Required
	BuildStart *metav1.Time `json:"buildStart"`
	// buildEnd describes when the build ended.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="buildEnd is immutable once set"
	//+optional
	BuildEnd *metav1.Time `json:"buildEnd,omitempty"`
	// finalImagePushSpec describes the fully qualified pushspec produced by this build that the final image can be. Must be in sha format.
	// +kubebuilder:validation:XValidation:rule=`((self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$')))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
	// +optional
	FinalImagePushspec string `json:"finalImagePullspec,omitempty"`
}

// MachineOSBuilderReference describes which ImageBuilder backend to use for this build/
// +union
// +kubebuilder:validation:XValidation:rule="has(self.imageBuilderType) && self.imageBuilderType == 'PodImageBuilder' ?  true : !has(self.buildPod)",message="buildPod is required when imageBuilderType is PodImageBuilder, and forbidden otherwise"
type MachineOSBuilderReference struct {
	// ImageBuilderType describes the image builder set in the MachineOSConfig
	// +unionDiscriminator
	ImageBuilderType MachineOSImageBuilderType `json:"imageBuilderType"`

	// relatedObjects is a list of objects that are related to the build process.
	// +unionMember,optional
	PodImageBuilder *ObjectReference `json:"buildPod,omitempty"`
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
type RenderedMachineConfigReference struct {
	// name is the name of the rendered MachineConfig object.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// ObjectReference contains enough information to let you inspect or modify the referred object.
type ObjectReference struct {
	// group of the referent.
	// +kubebuilder:validation:Required
	Group string `json:"group"`
	// resource of the referent.
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`
	// namespace of the referent.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// name of the referent.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// MachineOSConfigReference refers to the MachineOSConfig this build is based off of
type MachineOSConfigReference struct {
	// name of the MachineOSConfig
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}
