package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineosconfigs,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2090
// +openshift:enable:FeatureGate=OnClusterBuild
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +kubebuilder:metadata:labels=openshift.io/operator-managed=

// MachineOSConfig describes the configuration for a build process managed by the MCO
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type MachineOSConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the configuration of the machineosconfig
	// +required
	Spec MachineOSConfigSpec `json:"spec"`

	// status describes the status of the machineosconfig
	// +optional
	Status MachineOSConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineOSConfigList describes all configurations for image builds on the system
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type MachineOSConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items contains a collection of MachineOSConfig resources.
	// +optional
	Items []MachineOSConfig `json:"items"`
}

// MachineOSConfigSpec describes user-configurable options as well as information about a build process.
type MachineOSConfigSpec struct {
	// machineConfigPool is the pool which the build is for.
	// The Machine Config Operator will perform the build and roll out the built image to the specified pool.
	// +required
	MachineConfigPool MachineConfigPoolReference `json:"machineConfigPool"`
	// imageBuilder describes which image builder will be used in each build triggered by this MachineOSConfig.
	// Currently supported type(s): Job
	// +required
	ImageBuilder MachineOSImageBuilder `json:"imageBuilder"`
	// baseImagePullSecret is the secret used to pull the base image.
	// Must live in the openshift-machine-config-operator namespace if provided.
	// Defaults to using the cluster-wide pull secret if not specified. This is provided during install time of the cluster, and lives in the openshift-config namespace as a secret.
	// +optional
	BaseImagePullSecret *ImageSecretObjectReference `json:"baseImagePullSecret,omitempty"`
	// renderedImagePushSecret is the secret used to connect to a user registry.
	// The final image push and pull secrets should be separate and assume the principal of least privilege.
	// The push secret with write privilege is only required to be present on the node hosting the MachineConfigController pod.
	// The pull secret with read only privileges is required on all nodes.
	// By separating the two secrets, the risk of write credentials becoming compromised is reduced.
	// +required
	RenderedImagePushSecret ImageSecretObjectReference `json:"renderedImagePushSecret"`
	// renderedImagePushSpec describes the location of the final image.
	// The MachineOSConfig object will use the in cluster image registry configuration.
	// If you wish to use a mirror or any other settings specific to registries.conf, please specify those in the cluster wide registries.conf via the cluster image.config, ImageContentSourcePolicies, ImageDigestMirrorSet, or ImageTagMirrorSet objects.
	// The format of the image push spec is: host[:port][/namespace]/name:<tag> or svc_name.namespace.svc[:port]/repository/name:<tag>.
	// The length of the push spec must be between 1 to 447 characters.
	// +required
	RenderedImagePushSpec ImageTagFormat `json:"renderedImagePushSpec"`
	// containerFile describes the custom data the user has specified to build into the image.
	// This is also commonly called a Dockerfile and you can treat it as such. The content is the content of your Dockerfile.
	// See https://github.com/containers/common/blob/main/docs/Containerfile.5.md for the spec reference.
	// This is a list indexed by architecture name (e.g. AMD64), and allows specifying one containerFile per arch, up to 4.
	// +patchMergeKey=containerfileArch
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=containerfileArch
	// +kubebuilder:validation:MinItems=0
	// +kubebuilder:validation:MaxItems=4
	// +optional
	Containerfile []MachineOSContainerfile `json:"containerFile" patchStrategy:"merge" patchMergeKey:"containerfileArch"`
}

// MachineOSConfigStatus describes the status this config object and relates it to the builds associated with this MachineOSConfig
type MachineOSConfigStatus struct {
	// conditions are state related conditions for the object.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	// TODO(jerzhang): add godoc after conditions are finalized. Also consider adding printer columns.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// observedGeneration represents the generation of the MachineOSConfig object observed by the Machine Config Operator's build controller.
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf", message="observedGeneration must not move backwards"
	// +kubebuilder:validation:Minimum=0
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// currentImagePullSpec is the fully qualified image pull spec used by the MCO to pull down the new OSImage. This includes the sha256 image digest.
	// This is generated when the Machine Config Operator's build controller successfully completes the build, and is populated from the corresponding
	// MachineOSBuild object's FinalImagePushSpec. This may change after completion in reaction to spec changes that would cause a new image build,
	// but will not be removed.
	// The format of the image pull spec is: host[:port][/namespace]/name@sha256:<digest>,
	// where the digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
	// The length of the whole spec must be between 1 to 447 characters.
	// +optional
	CurrentImagePullSpec ImageDigestFormat `json:"currentImagePullSpec,omitempty"`
	// machineOSBuild is a reference to the MachineOSBuild object for this MachineOSConfig, which contains the status for the image build.
	// +optional
	MachineOSBuild *ObjectReference `json:"machineOSBuild,omitempty"`
}

type MachineOSImageBuilder struct {
	// imageBuilderType specifies the backend to be used to build the image.
	// +kubebuilder:validation:Enum:=Job
	// Valid options are: Job
	// +required
	ImageBuilderType MachineOSImageBuilderType `json:"imageBuilderType"`
}

// MachineOSContainerfile contains all custom content the user wants built into the image
type MachineOSContainerfile struct {
	// containerfileArch describes the architecture this containerfile is to be built for.
	// This arch is optional. If the user does not specify an architecture, it is assumed
	// that the content can be applied to all architectures, or in a single arch cluster: the only architecture.
	// +kubebuilder:validation:Enum:=ARM64;AMD64;PPC64LE;S390X;NoArch
	// +kubebuilder:default:=NoArch
	// +optional
	ContainerfileArch ContainerfileArch `json:"containerfileArch,omitempty"`
	// content is an embedded Containerfile/Dockerfile that defines the contents to be built into your image.
	// See https://github.com/containers/common/blob/main/docs/Containerfile.5.md for the spec reference.
	// for example, this would add the tree package to your hosts:
	//   FROM configs AS final
	//   RUN rpm-ostree install tree && \
	//     ostree container commit
	// This is a required field and can have a maximum length of **4096** characters.
	// +required
	// +kubebuilder:validation:MaxLength=4096
	Content string `json:"content"`
}

// +enum
type ContainerfileArch string

const (
	// describes the arm64 architecture
	Arm64 ContainerfileArch = "ARM64"
	// describes the amd64 architecture
	Amd64 ContainerfileArch = "AMD64"
	// describes the ppc64le architecture
	Ppc ContainerfileArch = "PPC64LE"
	// describes the s390x architecture
	S390 ContainerfileArch = "S390X"
	// describes a containerfile that can be applied to any arch
	NoArch ContainerfileArch = "NoArch"
)

// Refers to the name of a MachineConfigPool (e.g., "worker", "infra", etc.):
// the MachineOSBuilder pod validates that the user has provided a valid pool
type MachineConfigPoolReference struct {
	// name of the MachineConfigPool object.
	// This value should be at most 253 characters, and must contain only lowercase
	// alphanumeric characters, hyphens and periods, and should start and end with an alphanumeric character.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Name string `json:"name"`
}

// Refers to the name of an image registry push/pull secret needed in the build process.
type ImageSecretObjectReference struct {
	// name is the name of the secret used to push or pull this MachineOSConfig object.
	// Must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.
	// This secret must be in the openshift-machine-config-operator namespace.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +required
	Name string `json:"name"`
}

// ImageTagFormat is a type that conforms to the format host[:port][/namespace]/name:<tag> or svc_name.namespace.svc[:port]/repository/name:<tag>.
// The length of the field must be between 1 to 447 characters.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=447
// +kubebuilder:validation:XValidation:rule=`self.matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?(/[a-zA-Z0-9-_]{1,61})*/[a-zA-Z0-9-_.]+:[a-zA-Z0-9._-]+$') || self.matches('^[^.]+\\.[^.]+\\.svc:\\d+\\/[^\\/]+\\/[^\\/]+:[^\\/]+$')`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme. Or it must be a valid .svc followed by a port, repository, image name, and tag."
type ImageTagFormat string

// ImageDigestFormat is a type that conforms to the format host[:port][/namespace]/name@sha256:<digest>.
// The digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
// The length of the field must be between 1 to 447 characters.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=447
// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
type ImageDigestFormat string

// +enum
type MachineOSImageBuilderType string

const (
	// describes that the machine-os-builder will use a Job to spin up a custom pod builder that uses buildah
	JobBuilder MachineOSImageBuilderType = "Job"
)
