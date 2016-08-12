package rules

import (
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

type Accepter interface {
	Covers(unversioned.GroupResource) bool
	RequiresImage(unversioned.GroupResource) bool
	ResolvesImage(unversioned.GroupResource) bool

	Accepts(*ImagePolicyAttributes) bool
}

// mappedAccepter implements the Accepter interface for a map of group resources and accepters
type mappedAccepter map[unversioned.GroupResource]Accepter

func (a mappedAccepter) Covers(gr unversioned.GroupResource) bool {
	_, ok := a[gr]
	return ok
}

func (a mappedAccepter) RequiresImage(gr unversioned.GroupResource) bool {
	accepter, ok := a[gr]
	return ok && accepter.RequiresImage(gr)
}

func (a mappedAccepter) ResolvesImage(gr unversioned.GroupResource) bool {
	accepter, ok := a[gr]
	return ok && accepter.ResolvesImage(gr)
}

// Accepts returns true if no Accepter is registered for the group resource in attributes,
// or if the registered Accepter also returns true.
func (a mappedAccepter) Accepts(attr *ImagePolicyAttributes) bool {
	accepter, ok := a[attr.Resource]
	if !ok {
		return true
	}
	return accepter.Accepts(attr)
}

type executionAccepter struct {
	rules         []api.ImageExecutionPolicyRule
	covers        unversioned.GroupResource
	defaultReject bool
	requiresImage bool
	resolvesImage bool

	integratedRegistryMatcher RegistryMatcher
}

// NewExecutionRuleseAccepter creates an Accepter from the provided rules.
func NewExecutionRulesAccepter(rules []api.ImageExecutionPolicyRule, integratedRegistryMatcher RegistryMatcher) (Accepter, error) {
	mapped := make(mappedAccepter)

	for _, rule := range rules {
		requiresImage, over, selectors, err := imageConditionInfo(&rule.ImageCondition)
		if err != nil {
			return nil, err
		}
		rule.ImageCondition.MatchImageLabelSelectors = selectors
		for gr := range over {
			a, ok := mapped[gr]
			if !ok {
				a = &executionAccepter{
					covers: gr,
					integratedRegistryMatcher: integratedRegistryMatcher,
				}
				mapped[gr] = a
			}
			byResource := a.(*executionAccepter)
			byResource.rules = append(byResource.rules, rule)
			if rule.Resolve {
				byResource.resolvesImage = true
			}
			if requiresImage || rule.Resolve {
				byResource.requiresImage = true
			}
		}
	}

	for _, a := range mapped {
		byResource := a.(*executionAccepter)
		if len(byResource.rules) > 0 {
			// if all rules are reject, the default behavior is allow
			allReject := true
			for _, rule := range byResource.rules {
				if !rule.Reject {
					allReject = false
					break
				}
			}
			byResource.defaultReject = !allReject
		}
	}

	return mapped, nil
}

func (r *executionAccepter) RequiresImage(gr unversioned.GroupResource) bool {
	return r.requiresImage && r.Covers(gr)
}

func (r *executionAccepter) ResolvesImage(gr unversioned.GroupResource) bool {
	return r.resolvesImage && r.Covers(gr)
}

func (r *executionAccepter) Covers(gr unversioned.GroupResource) bool {
	return r.covers == gr
}

func (r *executionAccepter) Accepts(attrs *ImagePolicyAttributes) bool {
	if attrs.Resource != r.covers {
		return true
	}

	anyMatched := false
	for _, rule := range r.rules {
		if attrs.ExcludedRules.Has(rule.Name) && !rule.IgnoreNamespaceOverride {
			continue
		}

		matches := matchImageCondition(&rule.ImageCondition, r.integratedRegistryMatcher, attrs)
		glog.V(5).Infof("Validate image %v against rule %q: %t", attrs.Name, rule.Name, matches)
		if matches {
			if rule.Reject {
				return false
			}
			anyMatched = true
		}
	}
	return anyMatched || !r.defaultReject
}
