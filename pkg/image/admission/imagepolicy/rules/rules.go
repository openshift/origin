package rules

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ImagePolicyAttributes struct {
	Resource           unversioned.GroupResource
	Name               imageapi.DockerImageReference
	Image              *imageapi.Image
	ExcludedRules      sets.String
	IntegratedRegistry bool
}

type RegistryMatcher interface {
	Matches(name string) bool
}

type RegistryNameMatcher imageapi.DefaultRegistryFunc

func (m RegistryNameMatcher) Matches(name string) bool {
	current, ok := imageapi.DefaultRegistryFunc(m)()
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

type resourceSet map[unversioned.GroupResource]struct{}

func (s resourceSet) addAll(other resourceSet) {
	for k := range other {
		s[k] = struct{}{}
	}
}

func imageConditionInfo(rule *api.ImageCondition) (covers resourceSet, selectors []labels.Selector, err error) {
	covers = make(resourceSet)
	for _, gr := range rule.OnResources {
		covers[gr] = struct{}{}
	}

	for i := range rule.MatchImageLabels {
		s, err := unversioned.LabelSelectorAsSelector(&rule.MatchImageLabels[i])
		if err != nil {
			return nil, nil, err
		}
		selectors = append(selectors, s)
	}

	return covers, selectors, nil
}

func requiresImage(rule *api.ImageCondition) bool {
	switch {
	case len(rule.MatchImageLabels) > 0,
		len(rule.MatchImageAnnotations) > 0,
		len(rule.MatchDockerImageLabels) > 0:
		return true
	}

	return false
}

// matchImageCondition determines the result of an ImageCondition or the provided arguments.
func matchImageCondition(condition *api.ImageCondition, integrated RegistryMatcher, attrs *ImagePolicyAttributes) bool {
	result := matchImageConditionValues(condition, integrated, attrs)
	if condition.InvertMatch {
		result = !result
	}
	return result
}

// matchImageConditionValues handles only the match rules on the condition, returning true if the conditions match.
// Use matchImageCondition to apply invertMatch rules.
func matchImageConditionValues(rule *api.ImageCondition, integrated RegistryMatcher, attrs *ImagePolicyAttributes) bool {
	if rule.MatchIntegratedRegistry && !(attrs.IntegratedRegistry || integrated.Matches(attrs.Name.Registry)) {
		return false
	}
	if len(rule.MatchRegistries) > 0 && !hasAnyMatch(attrs.Name.Registry, rule.MatchRegistries) {
		return false
	}

	// all subsequent calls require the image
	image := attrs.Image
	if image == nil {
		if rule.SkipOnResolutionFailure {
			return true
		}

		// if we don't require an image to evaluate our rules, then there's no reason to continue from here
		// we already know that we passed our filter
		return !requiresImage(rule)
	}

	if len(rule.MatchDockerImageLabels) > 0 {
		if image.DockerImageMetadata.Config == nil {
			return false
		}
		if !matchKeyValue(image.DockerImageMetadata.Config.Labels, rule.MatchDockerImageLabels) {
			return false
		}
	}
	if !matchKeyValue(image.Annotations, rule.MatchImageAnnotations) {
		return false
	}
	for _, s := range rule.MatchImageLabelSelectors {
		if !s.Matches(labels.Set(image.Labels)) {
			return false
		}
	}

	return true
}

func matchKeyValue(all map[string]string, conditions []api.ValueCondition) bool {
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
