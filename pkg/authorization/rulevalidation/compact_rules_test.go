package rulevalidation

import (
	"reflect"
	"sort"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/authorization/api"
)

func TestCompactRules(t *testing.T) {
	testcases := map[string]struct {
		Rules    []api.PolicyRule
		Expected []api.PolicyRule
	}{
		"empty": {
			Rules:    []api.PolicyRule{},
			Expected: []api.PolicyRule{},
		},
		"simple": {
			Rules: []api.PolicyRule{
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
			Expected: []api.PolicyRule{
				{Verbs: sets.NewString("create", "delete"), APIGroups: []string{"extensions"}, Resources: sets.NewString("daemonsets")},
				{Verbs: sets.NewString("get", "list", "update", "patch"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("educate"), APIGroups: []string{""}, Resources: sets.NewString("dolphins")},
				{Verbs: nil, APIGroups: []string{""}, Resources: sets.NewString("pirates")},
				{Verbs: sets.NewString("create"), APIGroups: []string{""}, Resources: sets.NewString("pods")},
			},
		},
		"complex multi-group": {
			Rules: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("list"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
			},
			Expected: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
				{Verbs: sets.NewString("list"), APIGroups: []string{"", "builds.openshift.io"}, Resources: sets.NewString("builds")},
			},
		},

		"complex multi-resource": {
			Rules: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
			},
			Expected: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
			},
		},

		"complex named-resource": {
			Rules: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild2")},
			},
			Expected: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild")},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("mybuild2")},
			},
		},

		"complex non-resource": {
			Rules: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/")},
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/foo")},
			},
			Expected: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/")},
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/foo")},
			},
		},

		"complex attributes": {
			Rules: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &api.IsPersonalSubjectAccessReview{}},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &api.IsPersonalSubjectAccessReview{}},
			},
			Expected: []api.PolicyRule{
				{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &api.IsPersonalSubjectAccessReview{}},
				{Verbs: sets.NewString("list"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &api.IsPersonalSubjectAccessReview{}},
			},
		},
	}

	for k, tc := range testcases {
		rules := tc.Rules
		originalRules, err := kapi.Scheme.DeepCopy(tc.Rules)
		if err != nil {
			t.Errorf("%s: couldn't copy rules: %v", k, err)
			continue
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

		sort.Stable(api.SortableRuleSlice(compacted))
		sort.Stable(api.SortableRuleSlice(tc.Expected))
		if !reflect.DeepEqual(compacted, tc.Expected) {
			t.Errorf("%s: Expected\n%#v\ngot\n%#v", k, tc.Expected, compacted)
			continue
		}
	}
}

func TestIsSimpleResourceRule(t *testing.T) {
	testcases := map[string]struct {
		Rule     api.PolicyRule
		Simple   bool
		Resource unversioned.GroupResource
	}{
		"simple, no verbs": {
			Rule:     api.PolicyRule{Verbs: sets.NewString(), APIGroups: []string{""}, Resources: sets.NewString("builds")},
			Simple:   true,
			Resource: unversioned.GroupResource{Group: "", Resource: "builds"},
		},
		"simple, one verb": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
			Simple:   true,
			Resource: unversioned.GroupResource{Group: "", Resource: "builds"},
		},
		"simple, multi verb": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get", "list"), APIGroups: []string{""}, Resources: sets.NewString("builds")},
			Simple:   true,
			Resource: unversioned.GroupResource{Group: "", Resource: "builds"},
		},

		"complex, empty": {
			Rule:     api.PolicyRule{},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, no group": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{}, Resources: sets.NewString("builds")},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, multi group": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{"a", "b"}, Resources: sets.NewString("builds")},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, no resource": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString()},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, multi resource": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds", "images")},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, resource names": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), ResourceNames: sets.NewString("foo")},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, attribute restrictions": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), AttributeRestrictions: &api.IsPersonalSubjectAccessReview{}},
			Simple:   false,
			Resource: unversioned.GroupResource{},
		},
		"complex, non-resource urls": {
			Rule:     api.PolicyRule{Verbs: sets.NewString("get"), APIGroups: []string{""}, Resources: sets.NewString("builds"), NonResourceURLs: sets.NewString("/")},
			Simple:   false,
			Resource: unversioned.GroupResource{},
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
