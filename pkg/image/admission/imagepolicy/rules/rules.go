package rules

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ImagePolicyAttributes struct {
	Resource      unversioned.GroupResource
	Name          imageapi.DockerImageReference
	OriginalName  string
	Image         *imageapi.Image
	ExcludedRules sets.String
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

func prepareSourceRule(rule *api.ImageCondition) (requiresImage bool, covers resourceSet) {
	switch {
	case len(rule.MatchImageLabels) > 0,
		len(rule.MatchImageAnnotations) > 0,
		len(rule.MatchSignatures) > 0,
		len(rule.MatchDockerImageLabels) > 0:
		requiresImage = true
	}

	covers = make(resourceSet)
	for _, gr := range rule.OnResources {
		covers[gr] = struct{}{}
	}

	return requiresImage, covers
}

// emptyImage is used when resolution failures occur but resolution failure is allowed
var emptyImage = &imageapi.Image{
	DockerImageMetadata: imageapi.DockerImage{
		Config: &imageapi.DockerConfig{},
	},
}

func matchSourceRule(rule *api.ImageCondition, integrated RegistryMatcher, attrs *ImagePolicyAttributes) bool {
	if rule.MatchIntegratedRegistry && !integrated.Matches(attrs.Name.Registry) {
		return false
	}
	if len(rule.MatchRegistries) > 0 && !hasAnyMatch(attrs.Name.Registry, rule.MatchRegistries) {
		return false
	}

	// all subsequent calls require the image
	image := attrs.Image
	if image == nil {
		if !rule.AllowResolutionFailure {
			return false
		}
		// matches will be against an empty image
		image = emptyImage
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
	if !matchKeyValue(image.Labels, rule.MatchImageLabels) {
		return false
	}
	// TODO: implement signature match
	if len(rule.MatchSignatures) > 0 {
		return false
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
