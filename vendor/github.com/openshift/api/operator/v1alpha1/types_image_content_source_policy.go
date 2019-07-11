package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageContentSourcePolicy holds cluster-wide information about how to handle registry mirror rules.
// When multple policies are defined, the outcome of the behavior is defined on each field.
type ImageContentSourcePolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec ImageContentSourcePolicySpec `json:"spec"`
}

// ImageContentSourcePolicySpec is the specification of the ImageContentSourcePolicy CRD.
type ImageContentSourcePolicySpec struct {
	// repositoryDigestMirrors allows images referenced by image digests in pods to be
	// pulled from alternative mirrored repository locations. The image pull specification
	// provided to the pod will be compared to the source locations described in RepositoryDigestMirrors
	// and the image may be pulled down from any of the repositories in the list instead of the
	// specified repository allowing administrators to choose a potentially faster mirror.
	// Only image pull specifications that have an image disgest will have this behavior applied
	// to them - tags will continue to be pulled from the specified repository in the pull spec.
	// When multiple policies are defined, any overlaps found will be merged together when the mirror
	// rules are written to `/etc/containers/registries.conf`. For example, if policy A has sources `a, b, c`
	// and policy B has sources `c, d, e`. Then the mirror rule written to `registries.conf` will be `a, b, c, d, e`
	// where the duplicate `c` is removed.
	// +optional
	RepositoryDigestMirrors []RepositoryDigestMirrors `json:"repositoryDigestMirrors"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageContentSourcePolicyList lists the items in the ImageContentSourcePolicy CRD.
type ImageContentSourcePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []ImageContentSourcePolicy `json:"items"`
}

// RepositoryDigestMirrors holds cluster-wide information about how to handle mirros in the registries config.
// Note: the mirrors only work when pulling the images that are reference by their digests.
type RepositoryDigestMirrors struct {
	// sources are repositories that are mirrors of each other.
	// +optional
	Sources []string `json:"sources,omitempty"`
}
