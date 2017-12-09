package rulevalidation

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func TestCompactRules(t *testing.T) {
	testcases := map[string]struct {
		Rules    []authorizationapi.PolicyRule
		Expected []authorizationapi.PolicyRule
	}{
		"empty": {
			Rules:    []authorizationapi.PolicyRule{},
			Expected: []authorizationapi.PolicyRule{},
		},
		"simple": {
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("update", "patch"), APIGroups: []string{""}, Resources: sets.NewString("builds")},

				{Verbs: sets.NewString("create"), APIGroups: []string{"extensions"}, Resources: sets.NewString("daemonsets")},
				{Verbs: sets.NewString("delete"), APIGroups: []string{"extensions"}, Resources: sets.NewString("daemonsets")},

				{Verbs: sets.NewString("educate"), APIGroups: []string{""}, Resources: sets.NewString("dolphins")},

				// nil verbs are preserved in non-merge cases.
				// these are the pirates who don't do anything.
				{Verbs: nil, APIGroups: []string{""}, Resources: sets.NewString("pirates")},

				// Test merging into a nil Verbs string set
				{Verbs: nil, APIGroups: []string{""}, Resources: sets.NewString("pods")},
				{Verbs: sets.NewString("create"), APIGroups: []string{""}, Resources: sets.NewString("pods")},
			},
			Expected: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("create", "delete"), APIGroups: []string{"extensions"}, Resources: sets.NewString("daemonsets")},
				{Verbs: sets.NewString("get", "list", "update", "patch"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("educate"), APIGroups: []string{""}, Resources: sets.NewString("dolphins")},
				{Verbs: nil, APIGroups: []string{""}, Resources: sets.NewString("pirates")},
				{Verbs: sets.NewString("create"), APIGroups: []string{""}, Resources: sets.NewString("pods")},
			},
		},
		"complex multi-group": {
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("list"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
			},
			Expected: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("list"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
			},
		},

		"complex multi-resource": {
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
			},
			Expected: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
			},
		},

		"complex named-resource": {
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild2")},
			},
			Expected: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild2")},
			},
		},

		"complex non-resource": {
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/")},
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/foo")},
			},
			Expected: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/")},
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/foo")},
			},
		},

		"complex attributes": {
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
			},
			Expected: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
			},
		},
	}

	for k, tc := range testcases {
		rules := tc.Rules
		originalRules := make([]authorizationapi.PolicyRule, len(tc.Rules))
		for i, r := range tc.Rules {
			r.DeepCopyInto(&originalRules[i])
		}
		compacted, err := CompactRules(tc.Rules)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if !reflect.DeepEqual(rules, originalRules) {
			t.Errorf("%s: CompactRules mutated rules. Expected\n%#v\ngot\n%#v", k, originalRules, rules)
			continue
		}
		if covers, missing := Covers(compacted, rules); !covers {
			t.Errorf("%s: compacted rules did not cover original rules. missing: %#v", k, missing)
			continue
		}
		if covers, missing := Covers(rules, compacted); !covers {
			t.Errorf("%s: original rules did not cover compacted rules. missing: %#v", k, missing)
			continue
		}

		sort.Stable(authorizationapi.SortableRuleSlice(compacted))
		sort.Stable(authorizationapi.SortableRuleSlice(tc.Expected))
		if !reflect.DeepEqual(compacted, tc.Expected) {
			t.Errorf("%s: Expected\n%#v\ngot\n%#v", k, tc.Expected, compacted)
			continue
		}
	}
}

func TestIsSimpleResourceRule(t *testing.T) {
	testcases := map[string]struct {
		Rule     authorizationapi.PolicyRule
		Simple   bool
		Resource schema.GroupResource
	}{
		"simple, no verbs": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString(), APIGroups: []string{""}, Resources: sets.NewString("builds")},
			Simple:   true,
			Resource: schema.GroupResource{Group: "", Resource: "builds"},
		},
		"simple, one verb": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
			Simple:   true,
			Resource: schema.GroupResource{Group: "", Resource: "builds"},
		},
		"simple, multi verb": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get", "list"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
			Simple:   true,
			Resource: schema.GroupResource{Group: "", Resource: "builds"},
		},

		"complex, empty": {
			Rule:     authorizationapi.PolicyRule{},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, no group": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{}, Resources: sets.NewString("builds")},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, multi group": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{"a", "b"}, Resources: sets.NewString("builds")},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, no resource": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString()},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, multi resource": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, resource names": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("foo")},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, attribute restrictions": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
		"complex, non-resource urls": {
			Rule:     authorizationapi.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/")},
			Simple:   false,
			Resource: schema.GroupResource{},
		},
	}

	for k, tc := range testcases {
		resource, simple := isSimpleResourceRule(&tc.Rule)
		if simple != tc.Simple {
			t.Errorf("%s: expected simple=%v, got simple=%v", k, tc.Simple, simple)
			continue
		}
		if resource != tc.Resource {
			t.Errorf("%s: expected resource=%v, got resource=%v", k, tc.Resource, resource)
			continue
		}
	}
}
