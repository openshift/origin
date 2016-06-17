package rulevalidation

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// CompactRules combines rules that contain a single APIGroup/Resource, differ only by verb, and contain no other attributes.
// this is a fast check, and works well with the decomposed "missing rules" list from a Covers check.
func CompactRules(rules []authorizationapi.PolicyRule) ([]authorizationapi.PolicyRule, error) {
	compacted := make([]authorizationapi.PolicyRule, 0, len(rules))

	simpleRules := map[unversioned.GroupResource]*authorizationapi.PolicyRule{}
	for _, rule := range rules {
		if resource, isSimple := isSimpleResourceRule(&rule); isSimple {
			if existingRule, ok := simpleRules[resource]; ok {
				// Add the new verbs to the existing simple resource rule
				if existingRule.Verbs == nil {
					existingRule.Verbs = sets.NewString()
				}
				existingRule.Verbs.Insert(rule.Verbs.List()...)
			} else {
				// Copy the rule to accumulate matching simple resource rules into
				objCopy, err := api.Scheme.DeepCopy(rule)
				if err != nil {
					// Unit tests ensure this should not ever happen
					return nil, err
				}
				ruleCopy, ok := objCopy.(authorizationapi.PolicyRule)
				if !ok {
					// Unit tests ensure this should not ever happen
					return nil, fmt.Errorf("expected authorizationapi.PolicyRule, got %#v", objCopy)
				}
				simpleRules[resource] = &ruleCopy
			}
		} else {
			compacted = append(compacted, rule)
		}
	}

	// Once we've consolidated the simple resource rules, add them to the compacted list
	for _, simpleRule := range simpleRules {
		compacted = append(compacted, *simpleRule)
	}

	return compacted, nil
}

// isSimpleResourceRule returns true if the given rule contains verbs, a single resource, a single API group, and no other values
func isSimpleResourceRule(rule *authorizationapi.PolicyRule) (unversioned.GroupResource, bool) {
	resource := unversioned.GroupResource{}

	// If we have "complex" rule attributes, return early without allocations or expensive comparisons
	if len(rule.ResourceNames) > 0 || len(rule.NonResourceURLs) > 0 || rule.AttributeRestrictions != nil {
		return resource, false
	}
	// If we have multiple api groups or resources, return early
	if len(rule.APIGroups) != 1 || len(rule.Resources) != 1 {
		return resource, false
	}

	// Test if this rule only contains APIGroups/Resources/Verbs
	simpleRule := &authorizationapi.PolicyRule{APIGroups: rule.APIGroups, Resources: rule.Resources, Verbs: rule.Verbs}
	if !reflect.DeepEqual(simpleRule, rule) {
		return resource, false
	}
	resource = unversioned.GroupResource{Group: rule.APIGroups[0], Resource: rule.Resources.List()[0]}
	return resource, true
}
