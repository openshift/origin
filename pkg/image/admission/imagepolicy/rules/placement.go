package rules

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

// mappedAdjuster implements the Adjuster interface for a map of group resources and accepters
type Adjusters []Adjuster

func (a Adjusters) Covers(gr unversioned.GroupResource) bool {
	for _, adjuster := range a {
		if adjuster.Covers(gr) {
			return true
		}
	}
	return false
}

func (a Adjusters) RequiresImage(gr unversioned.GroupResource) bool {
	for _, adjuster := range a {
		if adjuster.RequiresImage(gr) {
			return true
		}
	}
	return false
}

// Adjust invokes adjust if the provided group resource matches a registered adjuster.
func (a Adjusters) Adjust(attr *ImagePolicyAttributes, podSpec *kapi.PodSpec) bool {
	adjusted := false
	for _, adjuster := range a {
		if adjuster.Covers(attr.Resource) {
			if adjuster.Adjust(attr, podSpec) {
				adjusted = true
			}
		}
	}
	return adjusted
}

type placementAdjuster struct {
	rules         []api.ImagePlacementPolicyRule
	requiresImage bool
	covers        unversioned.GroupResource

	integratedRegistryMatcher RegistryMatcher
}

func NewPlacementRulesAdjuster(rules []api.ImagePlacementPolicyRule, integratedRegistryMatcher RegistryMatcher) Adjuster {
	mapped := make(mappedAdjuster)

	for _, rule := range rules {
		requiresImage, over := prepareSourceRule(&rule.ImageCondition)
		for gr := range over {
			a, ok := mapped[gr]
			if !ok {
				a = &placementAdjuster{
					covers: gr,
					integratedRegistryMatcher: integratedRegistryMatcher,
				}
				mapped[gr] = a
			}
			byResource := a.(*placementAdjuster)
			byResource.rules = append(byResource.rules, rule)
			if requiresImage {
				byResource.requiresImage = true
			}
		}
	}

	return mapped
}

func (r *placementAdjuster) RequiresImage(gr unversioned.GroupResource) bool {
	return r.requiresImage && r.Covers(gr)
}

func (r *placementAdjuster) Covers(gr unversioned.GroupResource) bool {
	return gr == r.covers
}

func (r *placementAdjuster) Adjust(attrs *ImagePolicyAttributes, spec *kapi.PodSpec) bool {
	adjusted := false
	for _, rule := range r.rules {
		if attrs.ExcludedRules.Has(rule.Name) && !rule.IgnoreNamespaceOverride {
			continue
		}

		switch matches := matchSourceRule(&rule.ImageCondition, r.integratedRegistryMatcher, attrs); {
		case matches && rule.Reject, !matches && !rule.Reject:
			continue
		}

		selector := spec.NodeSelector
		for _, effect := range rule.Constrain {
			for k, v := range effect.Add {
				if selector == nil {
					selector = make(map[string]string)
				}
				selector[k] = v
			}
			adjusted = true
		}
		spec.NodeSelector = selector
	}
	return adjusted
}
