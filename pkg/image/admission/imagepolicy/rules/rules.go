package rules

import (
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/admission/apis/imagepolicy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type ImagePolicyAttributes struct {
	Resource           schema.GroupResource
	Name               imageapi.DockerImageReference
	Image              *imageapi.Image
	ExcludedRules      sets.String
	IntegratedRegistry bool
	LocalRewrite       bool
}

type RegistryMatcher interface {
	Matches(name string) bool
}

type RegistryNameMatcher func() (string, bool)

func (m RegistryNameMatcher) Matches(name string) bool {
	current, ok := m()
	if !ok {
		return false
	}
	return current == name
}

type nameSet []string

func (m nameSet) Matches(name string) bool {
	for _, s := range m {
		if s == name {
			return true
		}
	}
	return false
}

func NewRegistryMatcher(names []string) RegistryMatcher {
	return nameSet(names)
}

type resourceSet map[schema.GroupResource]struct{}

func imageConditionInfo(rule *imagepolicy.ImageCondition) (covers resourceSet, selectors []labels.Selector, err error) {
	covers = make(resourceSet)
	for _, gr := range rule.OnResources {
		covers[gr] = struct{}{}
	}

	for i := range rule.MatchImageLabels {
		s, err := metav1.LabelSelectorAsSelector(&rule.MatchImageLabels[i])
		if err != nil {
			return nil, nil, err
		}
		selectors = append(selectors, s)
	}

	return covers, selectors, nil
}

func requiresImage(rule *imagepolicy.ImageCondition) bool {
	switch {
	case len(rule.MatchImageLabels) > 0,
		len(rule.MatchImageAnnotations) > 0,
		len(rule.MatchDockerImageLabels) > 0:
		return true
	}

	return false
}

// matchImageCondition determines the result of an ImageCondition or the provided arguments.
func matchImageCondition(condition *imagepolicy.ImageCondition, integrated RegistryMatcher, attrs *ImagePolicyAttributes) bool {
	result := matchImageConditionValues(condition, integrated, attrs)
	glog.V(5).Infof("image matches conditions for %q: %t(invert=%t)", condition.Name, result, condition.InvertMatch)
	if condition.InvertMatch {
		result = !result
	}
	return result
}

// matchImageConditionValues handles only the match rules on the condition, returning true if the conditions match.
// Use matchImageCondition to apply invertMatch rules.
func matchImageConditionValues(rule *imagepolicy.ImageCondition, integrated RegistryMatcher, attrs *ImagePolicyAttributes) bool {
	if rule.MatchIntegratedRegistry && !(attrs.IntegratedRegistry || integrated.Matches(attrs.Name.Registry)) {
		glog.V(5).Infof("image registry %v does not match integrated registry", attrs.Name.Registry)
		return false
	}
	if len(rule.MatchRegistries) > 0 && !hasAnyMatch(attrs.Name.Registry, rule.MatchRegistries) {
		glog.V(5).Infof("image registry %v does not match registries from rule: %#v", attrs.Name.Registry, rule.MatchRegistries)
		return false
	}

	// all subsequent calls require the image
	image := attrs.Image
	if image == nil {
		if rule.SkipOnResolutionFailure {
			glog.V(5).Infof("rule does not match because image did not resolve and SkipOnResolutionFailure is true")
			// Likely we will never get here (see: https://github.com/openshift/origin/blob/4f709b48f8e52e8c6012bd8b91945f022a437a6a/pkg/image/admission/imagepolicy/rules/accept.go#L99-L103)
			// but if we do, treat the condition as not matching since we are supposed to skip this rule on resolution failure.
			return false
		}

		// if we don't require an image to evaluate our rules, then there's no reason to continue from here
		// we already know that we passed our filter
		r := requiresImage(rule)
		glog.V(5).Infof("image did not resolve, rule requires image metadata for matching: %t", r)
		return !r
	}

	if len(rule.MatchDockerImageLabels) > 0 {
		if image.DockerImageMetadata.Config == nil {
			glog.V(5).Infof("image has no labels to match rule labels")
			return false
		}
		if !matchKeyValue(image.DockerImageMetadata.Config.Labels, rule.MatchDockerImageLabels) {
			glog.V(5).Infof("image labels %#v do not match rule labels %#v", image.DockerImageMetadata.Config.Labels, rule.MatchDockerImageLabels)
			return false
		}
	}
	if !matchKeyValue(image.Annotations, rule.MatchImageAnnotations) {
		glog.V(5).Infof("image annotations %#v do not match rule annotations %#v", image.Annotations, rule.MatchImageAnnotations)
		return false
	}
	for _, s := range rule.MatchImageLabelSelectors {
		if !s.Matches(labels.Set(image.Labels)) {
			glog.V(5).Infof("image label selectors %#v do not match rule label selectors %#v", image.Labels, s)
			return false
		}
	}

	return true
}

func matchKeyValue(all map[string]string, conditions []imagepolicy.ValueCondition) bool {
	for _, condition := range conditions {
		switch {
		case condition.Set:
			if _, ok := all[condition.Key]; !ok {
				return false
			}
		default:
			if all[condition.Key] != condition.Value {
				return false
			}
		}
	}
	return true
}

func hasAnyMatch(name string, all []string) bool {
	for _, s := range all {
		if name == s {
			return true
		}
	}
	return false
}
