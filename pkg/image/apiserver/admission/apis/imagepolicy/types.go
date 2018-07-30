package imagepolicy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// IgnorePolicyRulesAnnotation is a comma delimited list of rule names to omit from consideration
	// in a given namespace. Loaded from the namespace.
	IgnorePolicyRulesAnnotation = "alpha.image.policy.openshift.io/ignore-rules"
	// ResolveNamesAnnotation when placed on an object template or object requests that all relevant
	// image names be resolved by taking the name and tag and attempting to resolve a local image stream.
	// This overrides the imageLookupPolicy on the image stream. If the object is not namespaced the
	// annotation is ignored. The only valid value is '*'.
	ResolveNamesAnnotation = "alpha.image.policy.openshift.io/resolve-names"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePolicyConfig is the configuration for controlling how images are used in the cluster.
type ImagePolicyConfig struct {
	metav1.TypeMeta

	// ResolveImages indicates what kind of image resolution should be done.  If a rewriting policy is chosen,
	// then the image pull specs will be updated.
	ResolveImages ImageResolutionType

	// ResolutionRules allows more specific image resolution rules to be applied per resource. If
	// empty, it defaults to allowing local image stream lookups - "mysql" will map to the image stream
	// tag "mysql:latest" in the current namespace if the stream supports it. The default for this
	// field is all known types that support image resolution, and the policy for those rules will be
	// set to the overall resolution policy if an execution rule references the same resource.
	ResolutionRules []ImageResolutionPolicyRule

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

// ImageResolutionPolicyRule describes resolution rules based on resource.
type ImageResolutionPolicyRule struct {
	// Policy controls whether resolution will happen if the rule doesn't match local names. This value
	// overrides the global image policy for all target resources.
	Policy ImageResolutionType
	// TargetResource is the identified group and resource. If Resource is *, this rule will apply
	// to all resources in that group.
	TargetResource metav1.GroupResource
	// LocalNames will allow single segment names to be interpreted as namespace local image
	// stream tags, but only if the target image stream tag has the "resolveLocalNames" field
	// set.
	LocalNames bool
}

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
	// IgnoreNamespaceOverride prevents this condition from being overridden when the
	// `alpha.image.policy.openshift.io/ignore-rules` is set on a namespace and contains this rule name.
	IgnoreNamespaceOverride bool

	// OnResources determines which resources this applies to. Defaults to 'pods' for ImageExecutionPolicyRules.
	OnResources []schema.GroupResource

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
	MatchImageLabels []metav1.LabelSelector
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
