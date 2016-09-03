package rulevalidation

import (
	"strings"

	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Covers determines whether or not the ownerRules cover the servantRules in terms of allowed actions.
// It returns whether or not the ownerRules cover and a list of the rules that the ownerRules do not cover.
func Covers(ownerRules, servantRules []authorizationapi.PolicyRule) (bool, []authorizationapi.PolicyRule) {
	// 1.  Break every servantRule into individual rule tuples: group, verb, resource, resourceName
	// 2.  Compare the mini-rules against each owner rule.  Because the breakdown is down to the most atomic level, we're guaranteed that each mini-servant rule will be either fully covered or not covered by a single owner rule
	// 3.  Any left over mini-rules means that we are not covered and we have a nice list of them.
	// TODO: it might be nice to collapse the list down into something more human readable

	subrules := []authorizationapi.PolicyRule{}
	for _, servantRule := range servantRules {
		subrules = append(subrules, BreakdownRule(servantRule)...)
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

// BreakdownRule takes a rule and builds an equivalent list of rules that each have at most one verb, one
// resource, and one resource name
func BreakdownRule(rule authorizationapi.PolicyRule) []authorizationapi.PolicyRule {
	subrules := []authorizationapi.PolicyRule{}

	for _, group := range rule.APIGroups {
		subrules = append(subrules, breakdownRuleForGroup(group, rule)...)
	}

	// if no groups are present, then the default group is assumed.  Buidl the subrules, then strip the groups
	if len(rule.APIGroups) == 0 {
		for _, subrule := range breakdownRuleForGroup("", rule) {
			subrule.APIGroups = nil
			subrules = append(subrules, subrule)
		}
	}

	// nonResourceURLs depend only on verb/nonResourceURL pairs
	for nonResourceURL := range rule.NonResourceURLs {
		for verb := range rule.Verbs {
			subrules = append(subrules, authorizationapi.PolicyRule{Verbs: sets.NewString(verb), NonResourceURLs: sets.NewString(nonResourceURL)})
		}
	}

	return subrules
}

func breakdownRuleForGroup(group string, rule authorizationapi.PolicyRule) []authorizationapi.PolicyRule {
	subrules := []authorizationapi.PolicyRule{}

	for resource := range authorizationapi.NormalizeResources(rule.Resources) {
		for verb := range rule.Verbs {
			if len(rule.ResourceNames) > 0 {
				for _, resourceName := range rule.ResourceNames.List() {
					subrules = append(subrules, authorizationapi.PolicyRule{APIGroups: []string{group}, Resources: sets.NewString(resource), Verbs: sets.NewString(verb), ResourceNames: sets.NewString(resourceName)})
				}

			} else {
				subrules = append(subrules, authorizationapi.PolicyRule{APIGroups: []string{group}, Resources: sets.NewString(resource), Verbs: sets.NewString(verb)})
			}
		}
	}

	return subrules
}

// ruleCovers determines whether the ownerRule (which may have multiple verbs, resources, and resourceNames) covers
// the subrule (which may only contain at most one verb, resource, and resourceName)
func ruleCovers(ownerRule, subrule authorizationapi.PolicyRule) bool {
	allResources := authorizationapi.NormalizeResources(ownerRule.Resources)

	ownerGroups := sets.NewString(ownerRule.APIGroups...)
	groupMatches := ownerGroups.Has(authorizationapi.APIGroupAll) || ownerGroups.HasAll(subrule.APIGroups...) || (len(ownerRule.APIGroups) == 0 && len(subrule.APIGroups) == 0)

	verbMatches := ownerRule.Verbs.Has(authorizationapi.VerbAll) || ownerRule.Verbs.HasAll(subrule.Verbs.List()...)
	resourceMatches := ownerRule.Resources.Has(authorizationapi.ResourceAll) || allResources.HasAll(subrule.Resources.List()...)
	resourceNameMatches := false

	if len(subrule.ResourceNames) == 0 {
		resourceNameMatches = (len(ownerRule.ResourceNames) == 0)
	} else {
		resourceNameMatches = (len(ownerRule.ResourceNames) == 0) || ownerRule.ResourceNames.HasAll(subrule.ResourceNames.List()...)
	}

	nonResourceCovers := nonResourceRuleCovers(ownerRule.NonResourceURLs, subrule.NonResourceURLs)

	return verbMatches && resourceMatches && resourceNameMatches && groupMatches && nonResourceCovers
}

func nonResourceRuleCovers(allowedPaths sets.String, requestedPaths sets.String) bool {
	if allowedPaths.Has(authorizationapi.NonResourceAll) {
		return true
	}

	for requestedPath := range requestedPaths {
		// If we contain the exact path, we're good
		if allowedPaths.Has(requestedPath) {
			continue
		}

		// See if one of the rules has a wildcard that allows this path
		prefixMatch := false
		for allowedPath := range allowedPaths {
			if strings.HasSuffix(allowedPath, "*") {
				if strings.HasPrefix(requestedPath, allowedPath[0:len(allowedPath)-1]) {
					return true
				}
			}
		}
		if !prefixMatch {
			return false
		}
	}

	return true
}
