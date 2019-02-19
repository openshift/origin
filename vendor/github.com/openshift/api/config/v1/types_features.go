package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Features holds cluster-wide information about feature gates.  The canonical name is `cluster`
type Features struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec FeaturesSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status FeaturesStatus `json:"status"`
}

type FeatureSet string

var (
	// TechPreviewNoUpgrade turns on tech preview features that are not part of the normal supported platform. Turning
	// this feature set on CANNOT BE UNDONE and PREVENTS UPGRADES.
	TechPreviewNoUpgrade FeatureSet = "TechPreviewNoUpgrade"
)

type FeaturesSpec struct {
	// featureSet changes the list of features in the cluster.  The default is empty.  Be very careful adjusting this setting.
	// Turning on or off features may cause irreversible changes in your cluster which cannot be undone.
	FeatureSet FeatureSet `json:"featureSet,omitempty"`
}

type FeaturesStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type FeaturesList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Features `json:"items"`
}

var (
	// TechPreviewEnabledKubeFeatures  is a list of features to turn on when TechPreviewNoUpgrade is turned on.  Only the exceptions
	// are listed here.  The feature gates left in their default state not listed.
	TechPreviewEnabledKubeFeatures = []string{}

	// TechPreviewDisabledKubeFeatures is a list of features to turn on when TechPreviewNoUpgrade is turned on.  Only the exceptions
	// are listed here.  The feature gates left in their default state not listed.
	TechPreviewDisabledKubeFeatures = []string{}

	// DefaultEnabledKubeFeatures is a list of features to turn on when the default featureset is turned on.  Only the exceptions
	// are listed here.  The feature gates left in their default state not listed.
	DefaultEnabledKubeFeatures = []string{}

	// DefaultDisabledKubeFeatures is a list of features to turn off when the default featureset is turned on.  Only the exceptions
	// are listed here.  The feature gates left in their default state not listed.
	DefaultDisabledKubeFeatures = []string{
		"PersistentLocalVolumes", // disable local volumes for 4.0, owned by sig-storage/hekumar@redhat.com
	}
)
