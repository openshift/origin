package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
)

// IgnorePolicyRulesAnnotation is a comma delimited list of rule names to omit from consideration
// in a given namespace. Loaded from the namespace.
const IgnorePolicyRulesAnnotation = "alpha.image.policy.openshift.io/ignore-rules"

// ImagePolicyConfig is the configuration for controlling how images are used in the cluster.
type ImagePolicyConfig struct {
	unversioned.TypeMeta

	// ResolveImages indicates what kind of image resolution should be done.  If a rewriting policy is chosen,
	// then the image pull specs will be updated.
	ResolveImages ImageResolutionType

	// ExecutionRules determine whether the use of an image is allowed in an object with a pod spec.
	// By default, these rules only apply to pods, but may be extended to other resource types.
	ExecutionRules []ImageExecutionPolicyRule
}

// ImageResolutionType is an enumerated string that indicates how image pull spec resolution should be handled
type ImageResolutionType string

var (
	// require resolution to succeed and rewrite the resource to use it
	RequiredRewrite ImageResolutionType = "RequiredRewrite"
	// require resolution to succeed, but don't rewrite the image pull spec
	Required ImageResolutionType = "Required"
	// attempt resolution, rewrite if successful
	AttemptRewrite ImageResolutionType = "AttemptRewrite"
	// attempt resolution, don't rewrite
	Attempt ImageResolutionType = "Attempt"
	// don't attempt resolution
	DoNotAttempt ImageResolutionType = "DoNotAttempt"
)

// ImageExecutionPolicyRule determines whether a provided image may be used on the platform.
type ImageExecutionPolicyRule struct {
	ImageCondition

	// Reject means this rule, if it matches the condition, will cause an immediate failure. No
	// other rules will be considered.
	Reject bool
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

	// OnResources determines which resources this applies to. Defaults to 'pods' for ImageExecutionPolicyRules.
	OnResources []unversioned.GroupResource

	// InvertMatch means the value of the condition is logically inverted (true -> false, false -> true).
	InvertMatch bool

	// MatchIntegratedRegistry will only match image sources that originate from the configured integrated
	// registry.
	MatchIntegratedRegistry bool
	// MatchRegistries will match image references that point to the provided registries. If any of the listed
	// registries match, this condition is satisfied.
	MatchRegistries []string

	// SkipOnResolutionFailure allows the subsequent conditions to be bypassed if the integrated registry does
	// not have access to image metadata (no image exists matching the image digest).
	SkipOnResolutionFailure bool

	// MatchDockerImageLabels checks against the resolved image for the presence of a Docker label. All conditions
	// must match.
	MatchDockerImageLabels []ValueCondition
	// MatchImageLabels checks against the resolved image for a label. All conditions must match.
	MatchImageLabels []unversioned.LabelSelector
	// MatchImageLabelSelectors is the processed form of MatchImageLabels. All conditions must match.
	MatchImageLabelSelectors []labels.Selector
	// MatchImageAnnotations checks against the resolved image for an annotation. All conditions must match.
	MatchImageAnnotations []ValueCondition
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
