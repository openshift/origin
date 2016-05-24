package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
)

// ImagePolicyConfig is the configuration for control of images running on the platform.
type ImagePolicyConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ExecutionRules determine whether the use of an image is allowed in an object with a pod spec.
	// By default, these rules only apply to pods, but may be extended to other resource types.
	// If all execution rules are negations, the default behavior is allow all. If any execution rule
	// is an allow, the default behavior is to reject all.
	ExecutionRules []ImageExecutionPolicyRule `json:"executionRules"`

	// TODO: enable additional rule types
	// ConsumptionRules are applied when creating resources with pod specs.
	// ConsumptionRules []ImageConsumptionPolicyRule `json:"consumptionRules"`
	// PlacementRules are applied when creating resources with pod specs.
	// PlacementRules []ImagePlacementPolicyRule `json:"placementRules"`
}

// ImageExecutionPolicyRule determines whether a provided image may be used on the platform.
type ImageExecutionPolicyRule struct {
	ImageCondition `json:",inline"`

	// Resolve indicates that images referenced by this resource must be resolved
	Resolve bool `json:"resolve"`
}

// ImageCondition defines the conditions for matching a particular image source. The conditions below
// are all required (logical AND). If Reject is specified, the condition is false if all conditions match,
// and true otherwise.
type ImageCondition struct {
	// Name is the name of this policy rule for reference. It must be unique across all rules.
	Name string `json:"name"`
	// IgnoreNamespaceOverride prevents this condition from being overriden when the
	// `alpha.image.policy.openshift.io/ignore-rules` is set on a namespace and contains this rule name.
	IgnoreNamespaceOverride bool `json:"ignoreNamespaceOverride"`

	// Reject indicates this rule is inverted - if all conditions below match, then this rule is considered to
	// not match.
	Reject bool `json:"reject"`

	// OnResources determines which resources this applies to. Defaults to 'pods' for ImageExecutionPolicyRules.
	OnResources []unversioned.GroupResource `json:"onResources"`

	// MatchIntegratedRegistry will only match image sources that originate from the configured integrated
	// registry.
	MatchIntegratedRegistry bool `json:"matchIntegratedRegistry"`
	// MatchRegistries will match image references that point to the provided registries
	MatchRegistries []string `json:"matchRegistries"`

	// AllowResolutionFailure allows the subsequent conditions to be bypassed if the integrated registry does
	// not have access to image metadata (no image exists matching the image digest).
	AllowResolutionFailure bool `json:"allowResolutionFailure"`

	// MatchDockerImageLabels checks against the resolved image for the presence of a Docker label
	MatchDockerImageLabels []ValueCondition `json:"matchDockerImageLabels"`
	// MatchImageLabels checks against the resolved image for a label.
	MatchImageLabels []ValueCondition `json:"matchImageLabels"`
	// MatchImageAnnotations checks against the resolved image for an annotation.
	MatchImageAnnotations []ValueCondition `json:"matchImageAnnotations"`

	// TODO: support signature resolution
	//MatchSignatures        []SignatureMatch `json:"matchSignatures"`
}

// ValueCondition reflects whether the following key in a map is set or has a given value.
type ValueCondition struct {
	// Key is the name of a key in a map to retrieve.
	Key string `json:"key"`
	// Set indicates the provided key exists in the map. This field is exclusive with Value.
	Set bool `json:"set"`
	// Value indicates the provided key has the given value. This field is exclusive with Set.
	Value string `json:"value"`
}

// SignatureMatch is not yet enabled
type SignatureMatch struct {
}

// ImageConsumptionPolicyRule, when matching an image, adds a counted resource to the object.
type ImageConsumptionPolicyRule struct {
	ImageCondition `json:",inline"`

	// Add defines an array of resources that are applied if this rule matches. The resources
	// are added to each container (or other resource bearing object).
	Add []ConsumeResourceEffect `json:"add"`
}

// ConsumeResourceEffect alters a resource by adding / setting resources.
type ConsumeResourceEffect struct {
	// Name is the name of a quantity to set on each matching container.
	// May not be specified if NameFromImageAnnotation or NameFromDockerImageLabel is set.
	Name string `json:"name"`
	// NameFromImageAnnotation is the name of an image annotation to use to set the name of the resource
	// quantity on each matching container.
	// May not be specified if Name or NameFromDockerImageLabel is set.
	NameFromImageAnnotation string `json:"nameFromImageAnnotation"`
	// NameFromDockerImageLabel is the name of a docker image label to use to set the name of the resource
	// quantity on each matching container.
	// May not be specified if Name or NameFromImageAnnotation is set.
	NameFromDockerImageLabel string `json:"nameFromDockerImageLabel"`

	// Quantity is the amount of quantity to set
	Quantity string `json:"quantity"`
	// QuantityFromImageAnnotation is the key on an image annotation to use for the value of this resource.
	// If this value is specified and no annotation is found, the value of Quantity is used instead.
	QuantityFromImageAnnotation string `json:"quantityFromImageAnnotation"`
	// QuantityFromDockerImageLabel is the key on a docker image label to use for the value of this resource.
	// If this value is specified and no annotation is found, the value of Quantity is used instead.
	QuantityFromDockerImageLabel string `json:"quantityFromDockerImageLabel"`
}

// ImagePlacementPolicyRule, when matching an image, applies the provided tolerations or taints.
type ImagePlacementPolicyRule struct {
	ImageCondition `json:",inline"`

	// Constrain restricts the node selector for this resource.
	Constrain []ConstrainPodNodeSelectorEffect `json:"constrain"`
	// Tolerate adds tolerations to this resource.
	Tolerate []TolerateNodeSelectorEffect `json:"tolerate"`
}

// ConstrainPodNodeSelectorEffect alters a resource by applying a node selector.
type ConstrainPodNodeSelectorEffect struct {
	// Add is a set of labels to overwrite the labels on the resource.
	Add map[string]string `json:"add"`
}

// TolerateNodeSelectorEffect alters a resource by applying a node toleration.
type TolerateNodeSelectorEffect struct {
	// Add is the toleration to add.
	Add v1.Toleration `json:"add"`
}
