package rules

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

type Adjuster interface {
	Covers(unversioned.GroupResource) bool
	RequiresImage(unversioned.GroupResource) bool

	Adjust(*ImagePolicyAttributes, *kapi.PodSpec) bool
}

// mappedAdjuster implements the Adjuster interface for a map of group resources and accepters
type mappedAdjuster map[unversioned.GroupResource]Adjuster

func (a mappedAdjuster) Covers(gr unversioned.GroupResource) bool {
	_, ok := a[gr]
	return ok
}

func (a mappedAdjuster) RequiresImage(gr unversioned.GroupResource) bool {
	adjuster, ok := a[gr]
	return ok && adjuster.RequiresImage(gr)
}

// Adjust invokes adjust if the provided group resource matches a registered adjuster.
func (a mappedAdjuster) Adjust(attr *ImagePolicyAttributes, podSpec *kapi.PodSpec) bool {
	adjuster, ok := a[attr.Resource]
	if !ok {
		return false
	}
	return adjuster.Adjust(attr, podSpec)
}

type consumptionAdjuster struct {
	rules         []api.ImageConsumptionPolicyRule
	defaultReject bool
	requiresImage bool
	covers        unversioned.GroupResource

	integratedRegistryMatcher RegistryMatcher
}

func NewConsumptionRulesAdjuster(rules []api.ImageConsumptionPolicyRule, integratedRegistryMatcher RegistryMatcher) Adjuster {
	mapped := make(mappedAdjuster)

	for _, rule := range rules {
		requiresImage, over := prepareSourceRule(&rule.ImageCondition)
		for gr := range over {
			a, ok := mapped[gr]
			if !ok {
				a = &consumptionAdjuster{
					covers: gr,
					integratedRegistryMatcher: integratedRegistryMatcher,
				}
				mapped[gr] = a
			}
			byResource := a.(*consumptionAdjuster)
			byResource.rules = append(byResource.rules, rule)
			if requiresImage {
				byResource.requiresImage = true
			}
		}
	}

	return mapped
}

func (r *consumptionAdjuster) RequiresImage(gr unversioned.GroupResource) bool {
	return r.requiresImage && r.Covers(gr)
}

func (r *consumptionAdjuster) Covers(gr unversioned.GroupResource) bool {
	return gr == r.covers
}

func (r *consumptionAdjuster) Adjust(attrs *ImagePolicyAttributes, spec *kapi.PodSpec) bool {
	adjusted := false
	for _, rule := range r.rules {
		if attrs.ExcludedRules.Has(rule.Name) && !rule.IgnoreNamespaceOverride {
			continue
		}

		switch matches := matchSourceRule(&rule.ImageCondition, r.integratedRegistryMatcher, attrs); {
		case matches && rule.Reject, !matches && !rule.Reject:
			continue
		}

		for _, effect := range rule.Add {
			name, ok := resourceNameFromImage(effect, attrs)
			if !ok {
				continue
			}
			quantity, ok := resourceQuantityFromImage(effect, attrs)
			if !ok {
				continue
			}

			for i := range spec.Containers {
				if applyContainerResourceEffect(attrs.OriginalName, name, quantity, &rule, &spec.Containers[i]) {
					adjusted = true
				}
			}
			for i := range spec.InitContainers {
				if applyContainerResourceEffect(attrs.OriginalName, name, quantity, &rule, &spec.InitContainers[i]) {
					adjusted = true
				}
			}
		}
	}
	return adjusted
}

var one = resource.MustParse("1")

func applyContainerResourceEffect(imageName string, name kapi.ResourceName, amount resource.Quantity, rule *api.ImageConsumptionPolicyRule, container *kapi.Container) bool {
	if imageName != container.Image {
		return false
	}
	if container.Resources.Requests == nil {
		container.Resources.Requests = make(kapi.ResourceList)
	}
	container.Resources.Requests[kapi.ResourceName(name)] = amount
	return true
}

func resourceNameFromImage(effect api.ConsumeResourceEffect, attrs *ImagePolicyAttributes) (kapi.ResourceName, bool) {
	switch {
	case len(effect.Name) > 0:
		return kapi.ResourceName(effect.Name), true
	case len(effect.NameFromImageAnnotation) > 0:
		if attrs.Image == nil {
			return "", false
		}
		value, ok := attrs.Image.Annotations[effect.NameFromImageAnnotation]
		return kapi.ResourceName(value), ok
	case len(effect.NameFromDockerImageLabel) > 0:
		if attrs.Image == nil {
			return "", false
		}
		config := attrs.Image.DockerImageMetadata.Config
		if config == nil {
			return "", false
		}
		value, ok := config.Labels[effect.NameFromDockerImageLabel]
		return kapi.ResourceName(value), ok
	}
	return "", false
}

func resourceQuantityFromImage(effect api.ConsumeResourceEffect, attrs *ImagePolicyAttributes) (resource.Quantity, bool) {
	if attrs.Image != nil {
		if len(effect.QuantityFromImageAnnotation) > 0 {
			if value, ok := attrs.Image.Annotations[effect.QuantityFromImageAnnotation]; ok && len(value) > 0 {
				if q, err := resource.ParseQuantity(value); err == nil {
					return q, true
				}
			}
		}
		if len(effect.QuantityFromDockerImageLabel) > 0 {
			if config := attrs.Image.DockerImageMetadata.Config; config != nil {
				if value, ok := config.Labels[effect.QuantityFromDockerImageLabel]; ok && len(value) > 0 {
					if q, err := resource.ParseQuantity(value); err == nil {
						return q, true
					}
				}
			}
		}
	}

	if len(effect.Quantity) > 0 {
		q, err := resource.ParseQuantity(effect.Quantity)
		return q, err == nil
	}
	return resource.Quantity{}, false
}
