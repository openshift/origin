package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePolicyConfig is the configuration for control of images running on the platform.
type ImagePolicyConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ResolveImages indicates the default image resolution behavior.  If a rewriting policy is chosen,
	// then the image pull specs will be updated.
	ResolveImages ImageResolutionType `json:"resolveImages"`

	// ResolutionRules allows more specific image resolution rules to be applied per resource. If
	// empty, it defaults to allowing local image stream lookups - "mysql" will map to the image stream
	// tag "mysql:latest" in the current namespace if the stream supports it. The default for this
	// field is all known types that support image resolution, and the policy for those rules will be
	// set to the overall resolution policy if an execution rule references the same resource.
	ResolutionRules []ImageResolutionPolicyRule `json:"resolutionRules"`

	// ExecutionRules determine whether the use of an image is allowed in an object with a pod spec.
	// By default, these rules only apply to pods, but may be extended to other resource types.
	// If all execution rules are negations, the default behavior is allow all. If any execution rule
	// is an allow, the default behavior is to reject all.
	ExecutionRules []ImageExecutionPolicyRule `json:"executionRules"`
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

// ImageResolutionPolicyRule describes resolution rules based on resource.
type ImageResolutionPolicyRule struct {
	// Policy controls whether resolution will happen if the rule doesn't match local names. This value
	// overrides the global image policy for all target resources.
	Policy ImageResolutionType `json:"policy"`
	// TargetResource is the identified group and resource. If Resource is *, this rule will apply
	// to all resources in that group.
	TargetResource GroupResource `json:"targetResource"`
	// LocalNames will allow single segment names to be interpreted as namespace local image
	// stream tags, but only if the target image stream tag has the "resolveLocalNames" field
	// set.
	LocalNames bool `json:"localNames"`
}

// ImageExecutionPolicyRule determines whether a provided image may be used on the platform.
type ImageExecutionPolicyRule struct {
	ImageCondition `json:",inline"`

	// Reject means this rule, if it matches the condition, will cause an immediate failure. No
	// other rules will be considered.
	Reject bool `json:"reject"`
}

// GroupResource represents a resource in a specific group.
type GroupResource struct {
	// Resource is the name of an admission resource to process, e.g. 'statefulsets'.
	Resource string `json:"resource"`
	// Group is the name of the group the resource is in, e.g. 'apps'.
	Group string `json:"group"`
}

// ImageCondition defines the conditions for matching a particular image source. The conditions below
// are all required (logical AND). If Reject is specified, the condition is false if all conditions match,
// and true otherwise.
type ImageCondition struct {
	// Name is the name of this policy rule for reference. It must be unique across all rules.
	Name string `json:"name"`
	// IgnoreNamespaceOverride prevents this condition from being overridden when the
	// `alpha.image.policy.openshift.io/ignore-rules` is set on a namespace and contains this rule name.
	IgnoreNamespaceOverride bool `json:"ignoreNamespaceOverride"`

	// OnResources determines which resources this applies to. Defaults to 'pods' for ImageExecutionPolicyRules.
	OnResources []GroupResource `json:"onResources"`

	// InvertMatch means the value of the condition is logically inverted (true -> false, false -> true).
	InvertMatch bool `json:"invertMatch"`

	// MatchIntegratedRegistry will only match image sources that originate from the configured integrated
	// registry.
	MatchIntegratedRegistry bool `json:"matchIntegratedRegistry"`
	// MatchRegistries will match image references that point to the provided registries. The image registry
	// must match at least one of these strings.
	MatchRegistries []string `json:"matchRegistries"`

	// SkipOnResolutionFailure allows the subsequent conditions to be bypassed if the integrated registry does
	// not have access to image metadata (no image exists matching the image digest).
	SkipOnResolutionFailure bool `json:"skipOnResolutionFailure"`

	// MatchDockerImageLabels checks against the resolved image for the presence of a Docker label. All
	// conditions must match.
	MatchDockerImageLabels []ValueCondition `json:"matchDockerImageLabels"`
	// MatchImageLabels checks against the resolved image for a label. All conditions must match.
	MatchImageLabels []metav1.LabelSelector `json:"matchImageLabels"`
	// MatchImageAnnotations checks against the resolved image for an annotation. All conditions must match.
	MatchImageAnnotations []ValueCondition `json:"matchImageAnnotations"`
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
