package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageDigestMirrorSet holds cluster-wide information about how to handle registry mirror rules on using digest pull specification.
// When multiple policies are defined, the outcome of the behavior is defined on each field.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1126
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=config-operator,operatorOrdering=01
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=imagedigestmirrorsets,scope=Cluster,shortName=idms
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations=release.openshift.io/bootstrap-required=true
type ImageDigestMirrorSet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec ImageDigestMirrorSetSpec `json:"spec"`
	// status contains the observed state of the resource.
	// +optional
	Status ImageDigestMirrorSetStatus `json:"status,omitempty"`
}

// ImageDigestMirrorSetSpec is the specification of the ImageDigestMirrorSet CRD.
type ImageDigestMirrorSetSpec struct {
	// imageDigestMirrors allows images referenced by image digests in pods to be
	// pulled from alternative mirrored repository locations. The image pull specification
	// provided to the pod will be compared to the source locations described in imageDigestMirrors
	// and the image may be pulled down from any of the mirrors in the list instead of the
	// specified repository allowing administrators to choose a potentially faster mirror.
	// To use mirrors to pull images using tag specification, users should configure
	// a list of mirrors using "ImageTagMirrorSet" CRD.
	//
	// If the image pull specification matches the repository of "source" in multiple imagedigestmirrorset objects,
	// only the objects which define the most specific namespace match will be used.
	// For example, if there are objects using quay.io/libpod and quay.io/libpod/busybox as
	// the "source", only the objects using quay.io/libpod/busybox are going to apply
	// for pull specification quay.io/libpod/busybox.
	// Each “source” repository is treated independently; configurations for different “source”
	// repositories don’t interact.
	//
	// If the "mirrors" is not specified, the image will continue to be pulled from the specified
	// repository in the pull spec.
	//
	// When multiple policies are defined for the same “source” repository, the sets of defined
	// mirrors will be merged together, preserving the relative order of the mirrors, if possible.
	// For example, if policy A has mirrors `a, b, c` and policy B has mirrors `c, d, e`, the
	// mirrors will be used in the order `a, b, c, d, e`.  If the orders of mirror entries conflict
	// (e.g. `a, b` vs. `b, a`) the configuration is not rejected but the resulting order is unspecified.
	// Users who want to use a specific order of mirrors, should configure them into one list of mirrors using the expected order.
	// +optional
	// +listType=atomic
	ImageDigestMirrors []ImageDigestMirrors `json:"imageDigestMirrors"`
}

type ImageDigestMirrorSetStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageDigestMirrorSetList lists the items in the ImageDigestMirrorSet CRD.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ImageDigestMirrorSetList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ImageDigestMirrorSet `json:"items"`
}

// +kubebuilder:validation:Pattern=`^((?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])(?:(?:\.(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))+)?(?::[0-9]+)?)(?:(?:/[a-z0-9]+(?:(?:(?:[._]|__|[-]*)[a-z0-9]+)+)?)+)?$`
type ImageMirror string

// MirrorSourcePolicy defines the fallback policy if fails to pull image from the mirrors.
// +kubebuilder:validation:Enum=NeverContactSource;AllowContactingSource
type MirrorSourcePolicy string

const (
	// NeverContactSource prevents image pull from the specified repository in the pull spec if the image pull from the mirror list fails.
	NeverContactSource MirrorSourcePolicy = "NeverContactSource"

	// AllowContactingSource allows falling back to the specified repository in the pull spec if the image pull from the mirror list fails.
	AllowContactingSource MirrorSourcePolicy = "AllowContactingSource"
)

// ImageDigestMirrors holds cluster-wide information about how to handle mirrors in the registries config.
type ImageDigestMirrors struct {
	// source matches the repository that users refer to, e.g. in image pull specifications. Setting source to a registry hostname
	// e.g. docker.io. quay.io, or registry.redhat.io, will match the image pull specification of corressponding registry.
	// "source" uses one of the following formats:
	// host[:port]
	// host[:port]/namespace[/namespace…]
	// host[:port]/namespace[/namespace…]/repo
	// [*.]host
	// for more information about the format, see the document about the location field:
	// https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md#choosing-a-registry-toml-table
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\*(?:\.(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))+$|^((?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])(?:(?:\.(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))+)?(?::[0-9]+)?)(?:(?:/[a-z0-9]+(?:(?:(?:[._]|__|[-]*)[a-z0-9]+)+)?)+)?$`
	Source string `json:"source"`
	// mirrors is zero or more locations that may also contain the same images. No mirror will be configured if not specified.
	// Images can be pulled from these mirrors only if they are referenced by their digests.
	// The mirrored location is obtained by replacing the part of the input reference that
	// matches source by the mirrors entry, e.g. for registry.redhat.io/product/repo reference,
	// a (source, mirror) pair *.redhat.io, mirror.local/redhat causes a mirror.local/redhat/product/repo
	// repository to be used.
	// The order of mirrors in this list is treated as the user's desired priority, while source
	// is by default considered lower priority than all mirrors.
	// If no mirror is specified or all image pulls from the mirror list fail, the image will continue to be
	// pulled from the repository in the pull spec unless explicitly prohibited by "mirrorSourcePolicy"
	// Other cluster configuration, including (but not limited to) other imageDigestMirrors objects,
	// may impact the exact order mirrors are contacted in, or some mirrors may be contacted
	// in parallel, so this should be considered a preference rather than a guarantee of ordering.
	// "mirrors" uses one of the following formats:
	// host[:port]
	// host[:port]/namespace[/namespace…]
	// host[:port]/namespace[/namespace…]/repo
	// for more information about the format, see the document about the location field:
	// https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md#choosing-a-registry-toml-table
	// +optional
	// +listType=set
	Mirrors []ImageMirror `json:"mirrors,omitempty"`
	// mirrorSourcePolicy defines the fallback policy if fails to pull image from the mirrors.
	// If unset, the image will continue to be pulled from the the repository in the pull spec.
	// sourcePolicy is valid configuration only when one or more mirrors are in the mirror list.
	// +optional
	MirrorSourcePolicy MirrorSourcePolicy `json:"mirrorSourcePolicy,omitempty"`
}
