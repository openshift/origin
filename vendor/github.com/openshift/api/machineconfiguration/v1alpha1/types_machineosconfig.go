package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machineosconfigs,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1773
// +openshift:enable:FeatureGate=OnClusterBuild
// +openshift:file-pattern=cvoRunLevel=0000_80,operatorName=machine-config,operatorOrdering=01
// +kubebuilder:metadata:labels=openshift.io/operator-managed=

// MachineOSConfig describes the configuration for a build process managed by the MCO
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type MachineOSConfig struct {
	metav1.TypeMeta   `json:",inline"`
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
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type MachineOSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []MachineOSConfig `json:"items"`
}

// MachineOSConfigSpec describes user-configurable options as well as information about a build process.
type MachineOSConfigSpec struct {
	// machineConfigPool is the pool which the build is for
	// +required
	MachineConfigPool MachineConfigPoolReference `json:"machineConfigPool"`
	// buildInputs is where user input options for the build live
	// +required
	BuildInputs BuildInputs `json:"buildInputs"`
	// buildOutputs is where user input options for the build live
	// +optional
	BuildOutputs BuildOutputs `json:"buildOutputs,omitempty"`
}

// MachineOSConfigStatus describes the status this config object and relates it to the builds associated with this MachineOSConfig
type MachineOSConfigStatus struct {
	// conditions are state related conditions for the config.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// observedGeneration represents the generation observed by the controller.
	// this field is updated when the user changes the configuration in BuildSettings or the MCP this object is associated with.
	// +required
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// currentImagePullspec is the fully qualified image pull spec used by the MCO to pull down the new OSImage. This must include sha256.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
	// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
	// +optional
	CurrentImagePullspec string `json:"currentImagePullspec,omitempty"`
}

// BuildInputs holds all of the information needed to trigger a build
type BuildInputs struct {
	// baseOSExtensionsImagePullspec is the base Extensions image used in the build process
	// the MachineOSConfig object will use the in cluster image registry configuration.
	// if you wish to use a mirror or any other settings specific to registries.conf, please specify those in the cluster wide registries.conf.
	// The format of the image pullspec is:
	// host[:port][/namespace]/name@sha256:<digest>
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
	// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
	// +optional
	BaseOSExtensionsImagePullspec string `json:"baseOSExtensionsImagePullspec,omitempty"`
	// baseOSImagePullspec is the base OSImage we use to build our custom image.
	// the MachineOSConfig object will use the in cluster image registry configuration.
	// if you wish to use a mirror or any other settings specific to registries.conf, please specify those in the cluster wide registries.conf.
	// The format of the image pullspec is:
	// host[:port][/namespace]/name@sha256:<digest>
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
	// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
	// +optional
	BaseOSImagePullspec string `json:"baseOSImagePullspec,omitempty"`
	// baseImagePullSecret is the secret used to pull the base image.
	// must live in the openshift-machine-config-operator namespace
	// +required
	BaseImagePullSecret ImageSecretObjectReference `json:"baseImagePullSecret"`
	// machineOSImageBuilder describes which image builder will be used in each build triggered by this MachineOSConfig
	// +required
	ImageBuilder *MachineOSImageBuilder `json:"imageBuilder"`
	// renderedImagePushSecret is the secret used to connect to a user registry.
	// the final image push and pull secrets should be separate for security concerns. If the final image push secret is somehow exfiltrated,
	// that gives someone the power to push images to the image repository. By comparison, if the final image pull secret gets exfiltrated,
	// that only gives someone to pull images from the image repository. It's basically the principle of least permissions.
	// this push secret will be used only by the MachineConfigController pod to push the image to the final destination. Not all nodes will need to push this image, most of them
	// will only need to pull the image in order to use it.
	// +required
	RenderedImagePushSecret ImageSecretObjectReference `json:"renderedImagePushSecret"`
	// renderedImagePushspec describes the location of the final image.
	// the MachineOSConfig object will use the in cluster image registry configuration.
	// if you wish to use a mirror or any other settings specific to registries.conf, please specify those in the cluster wide registries.conf.
	// The format of the image pushspec is:
	// host[:port][/namespace]/name:<tag> or svc_name.namespace.svc[:port]/repository/name:<tag>
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=447
	// +kubebuilder:validation:XValidation:rule=`((self.split(':').size() == 2 && self.split(':')[1].matches('^([a-zA-Z0-9-./:])+$')) || self.matches('^[^.]+\\.[^.]+\\.svc:\\d+\\/[^\\/]+\\/[^\\/]+:[^\\/]+$'))`,message="the OCI Image reference must end with a valid :<tag>, where '<digest>' is 64 characters long and '<tag>' is any valid string  Or it must be a valid .svc followed by a port, repository, image name, and tag."
	// +kubebuilder:validation:XValidation:rule=`((self.split(':').size() == 2 && self.split(':')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$')) || self.matches('^[^.]+\\.[^.]+\\.svc:\\d+\\/[^\\/]+\\/[^\\/]+:[^\\/]+$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme. Or it must be a valid .svc followed by a port, repository, image name, and tag."
	// +required
	RenderedImagePushspec string `json:"renderedImagePushspec"`
	// releaseVersion is associated with the base OS Image. This is the version of Openshift that the Base Image is associated with.
	// This field is populated from the machine-config-osimageurl configmap in the openshift-machine-config-operator namespace.
	// It will come in the format: 4.16.0-0.nightly-2024-04-03-065948 or any valid release. The MachineOSBuilder populates this field and validates that this is a valid stream.
	// This is used as a label in the dockerfile that builds the OS image.
	// +optional
	ReleaseVersion string `json:"releaseVersion,omitempty"`
	// containerFile describes the custom data the user has specified to build into the image.
	// this is also commonly called a Dockerfile and you can treat it as such. The content is the content of your Dockerfile.
	// +patchMergeKey=containerfileArch
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=containerfileArch
	// +kubebuilder:validation:MinItems=0
	// +kubebuilder:validation:MaxItems=7
	// +optional
	Containerfile []MachineOSContainerfile `json:"containerFile" patchStrategy:"merge" patchMergeKey:"containerfileArch"`
}

// BuildOutputs holds all information needed to handle booting the image after a build
// +union
type BuildOutputs struct {
	// currentImagePullSecret is the secret used to pull the final produced image.
	// must live in the openshift-machine-config-operator namespace
	// the final image push and pull secrets should be separate for security concerns. If the final image push secret is somehow exfiltrated,
	// that gives someone the power to push images to the image repository. By comparison, if the final image pull secret gets exfiltrated,
	// that only gives someone to pull images from the image repository. It's basically the principle of least permissions.
	// this pull secret will be used on all nodes in the pool. These nodes will need to pull the final OS image and boot into it using rpm-ostree or bootc.
	// +optional
	CurrentImagePullSecret ImageSecretObjectReference `json:"currentImagePullSecret,omitempty"`
}

type MachineOSImageBuilder struct {
	// imageBuilderType specifies the backend to be used to build the image.
	// +kubebuilder:default:=PodImageBuilder
	// +kubebuilder:validation:Enum:=PodImageBuilder
	// Valid options are: PodImageBuilder
	ImageBuilderType MachineOSImageBuilderType `json:"imageBuilderType"`
}

// MachineOSContainerfile contains all custom content the user wants built into the image
type MachineOSContainerfile struct {
	// containerfileArch describes the architecture this containerfile is to be built for
	// this arch is optional. If the user does not specify an architecture, it is assumed
	// that the content can be applied to all architectures, or in a single arch cluster: the only architecture.
	// +kubebuilder:validation:Enum:=arm64;amd64;ppc64le;s390x;aarch64;x86_64;noarch
	// +kubebuilder:default:=noarch
	// +optional
	ContainerfileArch ContainerfileArch `json:"containerfileArch,omitempty"`
	// content is the custom content to be built
	// +required
	Content string `json:"content"`
}

type ContainerfileArch string

const (
	// describes the arm64 architecture
	Arm64 ContainerfileArch = "arm64"
	// describes the amd64 architecture
	Amd64 ContainerfileArch = "amd64"
	// describes the ppc64le architecture
	Ppc ContainerfileArch = "ppc64le"
	// describes the s390x architecture
	S390 ContainerfileArch = "s390x"
	// describes the aarch64 architecture
	Aarch64 ContainerfileArch = "aarch64"
	// describes the fx86_64 architecture
	X86_64 ContainerfileArch = "x86_64"
	// describes a containerfile that can be applied to any arch
	NoArch ContainerfileArch = "noarch"
)

// Refers to the name of a MachineConfigPool (e.g., "worker", "infra", etc.):
// the MachineOSBuilder pod validates that the user has provided a valid pool
type MachineConfigPoolReference struct {
	// name of the MachineConfigPool object.
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`
	// +required
	Name string `json:"name"`
}

// Refers to the name of an image registry push/pull secret needed in the build process.
type ImageSecretObjectReference struct {
	// name is the name of the secret used to push or pull this MachineOSConfig object.
	// this secret must be in the openshift-machine-config-operator namespace.
	// +required
	Name string `json:"name"`
}

type MachineOSImageBuilderType string

const (
	// describes that the machine-os-builder will use a custom pod builder that uses buildah
	PodBuilder MachineOSImageBuilderType = "PodImageBuilder"
)
