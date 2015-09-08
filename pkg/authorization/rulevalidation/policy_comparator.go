package rulevalidation

import (
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Covers determines whether or not the ownerRules cover the servantRules in terms of allowed actions.
// It returns whether or not the ownerRules cover and a list of the rules that the ownerRules do not cover.
func Covers(ownerRules, servantRules []authorizationapi.PolicyRule) (bool, []authorizationapi.PolicyRule) {
	// 1.  Break every servantRule into individual rule tuples: verb, resource, resourceName
	// 2.  Compare the mini-rules against each owner rule.  Because the breakdown is down to the most atomic level, we're guaranteed that each mini-servant rule will be either fully covered or not covered by a single owner rule
	// 3.  Any left over mini-rules means that we are not covered and we have a nice list of them.
	// TODO: it might be nice to collapse the list down into something more human readable

	subrules := []authorizationapi.PolicyRule{}
	for _, servantRule := range servantRules {
		subrules = append(subrules, breakdownRule(servantRule)...)
	}

	// fmt.Printf("subrules: %v\n", subrules)
	// fmt.Printf("ownerRules: %v\n", ownerRules)

	uncoveredRules := []authorizationapi.PolicyRule{}
	for _, subrule := range subrules {
		covered := false
		for _, ownerRule := range ownerRules {
			if ruleCovers(ownerRule, subrule) {
				covered = true
				break
			}
		}

		if !covered {
			uncoveredRules = append(uncoveredRules, subrule)
		}
	}

	return (len(uncoveredRules) == 0), uncoveredRules
}

// breadownRule takes a rule and builds an equivalent list of rules that each have at most one verb, one
// resource, and one resource name
func breakdownRule(rule authorizationapi.PolicyRule) []authorizationapi.PolicyRule {
	subrules := []authorizationapi.PolicyRule{}

	for resource := range authorizationapi.ExpandResources(rule.Resources) {
		for verb := range rule.Verbs {
			if len(rule.ResourceNames) > 0 {
				for _, resourceName := range rule.ResourceNames.List() {
					subrules = append(subrules, authorizationapi.PolicyRule{Resources: util.NewStringSet(resource), Verbs: util.NewStringSet(verb), ResourceNames: util.NewStringSet(resourceName)})
				}

			} else {
				subrules = append(subrules, authorizationapi.PolicyRule{Resources: util.NewStringSet(resource), Verbs: util.NewStringSet(verb)})
			}

		}
	}

	return subrules
}

// ruleCovers determines whether the ownerRule (which may have multiple verbs, resources, and resourceNames) covers
// the subrule (which may only contain at most one verb, resource, and resourceName)
func ruleCovers(ownerRule, subrule authorizationapi.PolicyRule) bool {
	allResources := authorizationapi.ExpandResources(ownerRule.Resources)

	verbMatches := ownerRule.Verbs.Has("*") || ownerRule.Verbs.HasAll(subrule.Verbs.List()...)
	resourceMatches := ownerRule.Resources.Has("*") || allResources.HasAll(subrule.Resources.List()...)
	resourceNameMatches := false

	if len(subrule.ResourceNames) == 0 {
		resourceNameMatches = (len(ownerRule.ResourceNames) == 0)
	} else {
		resourceNameMatches = (len(ownerRule.ResourceNames) == 0) || ownerRule.ResourceNames.HasAll(subrule.ResourceNames.List()...)
	}

	return verbMatches && resourceMatches && resourceNameMatches
}
