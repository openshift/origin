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

type FeaturesSpec struct {
	// enabled holds a list of features that should be enabled.  You only need to enter in the exceptional
	// cases.  Most features that should be enabled are enabled out of the box.  Often times, setting a feature
	// enabled can cause your cluster to be unstable.  If that is the case, status will be updated.
	// Because of the nature of feature gates,
	// the full list isn't known by a single entity making static validation unrealistic.  You can watch
	// status of this resource to figure out where your feature was respected.
	// +optional
	Enabled []string `json:"enabled"`

	// disabled holds a list of features that should be disabled.  You only need to enter in the exceptional
	// cases.  Most features that should be disabled are disabled out of the box.  Often times, setting a feature
	// disabled can cause your cluster to be unstable.  If that is the case, status will be updated.
	// Because of the nature of feature gates,
	// the full list isn't known by a single entity making static validation unrealistic.  You can watch
	// status of this resource to figure out where your feature was respected.
	// +optional
	Disabled []string `json:"disabled"`
}

type FeaturesStatus struct {
	// featureConditions holds information about each enabled or disabled feature as aggregated from multiple
	// operators.  It is keyed by name.
	FeatureConditions map[string]FeatureCondition `json:"featureConditions"`
}

type FeatureCondition struct {
	// operatorCondition holds information about each operator that attempted to handle a particular feature.
	// It is keyed by the operator name and indicates success or failure with a message.  No entry for an operator
	// means that the operator did not know about or acknowledge your feature.
	OperatorConditions map[string]OperatorFeatureCondition `json:"operatorConditions"`
}

type OperatorFeatureCondition struct {
	// failure is a message indicating that the operator had trouble handling a feature and explaining why.
	// +optional
	Failure string `json:"failure,omitempty"`
	// success is a message indicating that the operator honored a feature.
	// +optional
	Success string `json:"success,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type FeaturesList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Features `json:"items"`
}
