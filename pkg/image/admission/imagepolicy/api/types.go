package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// ImagePolicyConfig is the configuration for controlling how images are used in the cluster.
type ImagePolicyConfig struct {
	unversioned.TypeMeta

	// ExecutionRules determine whether the use of an image is allowed in an object with a pod spec.
	// By default, these rules only apply to pods, but may be extended to other resource types.
	ExecutionRules []ImageExecutionPolicyRule
	// ConsumptionRules are applied when creating resources with pod specs.
	ConsumptionRules []ImageConsumptionPolicyRule
	// PlacementRules are applied when creating resources with pod specs.
	PlacementRules []ImagePlacementPolicyRule
}

// ImageExecutionPolicyRule determines whether a provided image may be used on the platform.
type ImageExecutionPolicyRule struct {
	ImageCondition

	// Resolve indicates that images referenced by this resource must be resolved
	Resolve bool
}

// ImageCondition defines the conditions for matching a particular image source. The conditions below
// are all required (logical AND). If Reject is specified, the condition is false if all conditions match,
// and true otherwise.
type ImageCondition struct {
	// Name is the name of this policy rule for reference. It must be unique across all rules.
	Name string
	// IgnoreNamespaceOverride prevents this condition from being overriden when the
	// `alpha.image.policy.openshift.io/ignore-rules` is set on a namespace and contains this rule name.
	IgnoreNamespaceOverride bool

	// Reject indicates this rule is inverted - if all conditions below match, then this rule is considered to
	// not match.
	Reject bool

	// OnResources determines which resources this applies to. Defaults to 'pods' for ImageExecutionPolicyRules.
	OnResources []unversioned.GroupResource

	// MatchIntegratedRegistry will only match image sources that originate from the configured integrated
	// registry.
	MatchIntegratedRegistry bool
	// MatchRegistries will match image references that point to the provided registries
	MatchRegistries []string

	// AllowResolutionFailure allows the subsequent conditions to be bypassed if the integrated registry does
	// not have access to image metadata (no image exists matching the image digest).
	AllowResolutionFailure bool

	// MatchDockerImageLabels checks against the resolved image for the presence of a Docker label
	MatchDockerImageLabels []ValueCondition
	// MatchImageLabels checks against the resolved image for a label.
	MatchImageLabels []ValueCondition
	// MatchImageAnnotations checks against the resolved image for an annotation.
	MatchImageAnnotations []ValueCondition
	// MatchSignatures checks against the provided signatures.
	// TODO: implement signature checking.
	MatchSignatures []SignatureMatch `json:"matchSignatures"`
}

// ValueCondition reflects whether the following key in a map is set or has a given value.
type ValueCondition struct {
	// Key is the name of a key in a map to retrieve.
	Key string
	// Set indicates the provided key exists in the map. This field is exclusive with Value.
	Set bool
	// Value indicates the provided key has the given value. This field is exclusive with Set.
	Value string
}

// SignatureMatch is not yet enabled
type SignatureMatch struct {
}

// ImageConsumptionPolicyRule, when matching an image, adds a counted resource to the object.
type ImageConsumptionPolicyRule struct {
	ImageCondition

	Add []ConsumeResourceEffect
}

type ConsumeResourceEffect struct {
	// Name is the name of a quantity to set on each matching container.
	// May not be specified if NameFromImageAnnotation or NameFromDockerImageLabel is set.
	Name string
	// NameFromImageAnnotation is the name of an image annotation to use to set the name of the resource
	// quantity on each matching container.
	// May not be specified if Name or NameFromDockerImageLabel is set.
	NameFromImageAnnotation string
	// NameFromDockerImageLabel is the name of a docker image label to use to set the name of the resource
	// quantity on each matching container.
	// May not be specified if Name or NameFromImageAnnotation is set.
	NameFromDockerImageLabel string

	// Quantity is the amount of quantity to set
	Quantity string
	// QuantityFromImageAnnotation is the key on an image annotation to use for the value of this resource.
	// If this value is specified and no annotation is found, the value of Quantity is used instead.
	QuantityFromImageAnnotation string
	// QuantityFromDockerImageLabel is the key on a docker image label to use for the value of this resource.
	// If this value is specified and no annotation is found, the value of Quantity is used instead.
	QuantityFromDockerImageLabel string
}

// ImagePlacementPolicyRule, when matching an image, applies the provided tolerations or taints.
type ImagePlacementPolicyRule struct {
	ImageCondition

	Constrain []ConstrainPodNodeSelectorEffect
	Tolerate  []TolerateNodeSelectorEffect
}

type ConstrainPodNodeSelectorEffect struct {
	Add map[string]string
}

type TolerateNodeSelectorEffect struct {
	Add api.Toleration
}
