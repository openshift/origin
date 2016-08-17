package scope

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestUserEvaluator(t *testing.T) {
	testCases := []struct {
		name     string
		scopes   []string
		err      string
		numRules int
	}{
		{
			name:     "missing-part",
			scopes:   []string{UserIndicator},
			err:      "unrecognized scope",
			numRules: 1, // we always add the discovery rules
		},
		{
			name:     "bad-part",
			scopes:   []string{UserIndicator + "foo"},
			err:      "unrecognized scope",
			numRules: 1, // we always add the discovery rules
		},
		{
			name:     "info",
			scopes:   []string{UserInfo},
			numRules: 2,
		},
		{
			name:     "one-error",
			scopes:   []string{UserIndicator, UserInfo},
			err:      "unrecognized scope",
			numRules: 2,
		},
		{
			name:     "access",
			scopes:   []string{UserAccessCheck},
			numRules: 3,
		},
		{
			name:     "both",
			scopes:   []string{UserInfo, UserAccessCheck},
			numRules: 4,
		},
		{
			name:     "list--scoped-projects",
			scopes:   []string{UserListScopedProjects},
			numRules: 2,
		},
	}

	for _, tc := range testCases {
		actualRules, actualErr := ScopesToRules(tc.scopes, "namespace", nil)
		switch {
		case len(tc.err) == 0 && actualErr == nil:
		case len(tc.err) == 0 && actualErr != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, actualErr)
		case len(tc.err) != 0 && actualErr == nil:
			t.Errorf("%s: missing error: %v", tc.name, tc.err)
		case len(tc.err) != 0 && actualErr != nil:
			if !strings.Contains(actualErr.Error(), tc.err) {
				t.Errorf("%s: expected %v, got %v", tc.name, tc.err, actualErr)
			}
		}

		if len(actualRules) != tc.numRules {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.numRules, len(actualRules))
		}
	}
}

func TestClusterRoleEvaluator(t *testing.T) {
	testCases := []struct {
		name            string
		scopes          []string
		namespace       string
		clusterRoles    []authorizationapi.ClusterRole
		policyGetterErr error
		numRules        int
		err             string
	}{
		{
			name:     "bad-format-1",
			scopes:   []string{ClusterRoleIndicator},
			err:      "bad format for",
			numRules: 1, // we always add the discovery rules
		},
		{
			name:     "bad-format-2",
			scopes:   []string{ClusterRoleIndicator + "foo"},
			err:      "bad format for",
			numRules: 1, // we always add the discovery rules
		},
		{
			name:     "bad-format-3",
			scopes:   []string{ClusterRoleIndicator + ":ns"},
			err:      "bad format for",
			numRules: 1, // we always add the discovery rules
		},
		{
			name:     "bad-format-4",
			scopes:   []string{ClusterRoleIndicator + "foo:"},
			err:      "bad format for",
			numRules: 1, // we always add the discovery rules
		},
		{
			name: "missing-role",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{}},
				},
			},
			scopes:   []string{ClusterRoleIndicator + "missing:*"},
			err:      `clusterrole "missing" not found`,
			numRules: 1,
		},
		{
			name: "mismatched-namespace",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{}},
				},
			},
			namespace: "current-ns",
			scopes:    []string{ClusterRoleIndicator + "admin:mismatch"},
			numRules:  1,
		},
		{
			name: "all-namespaces",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{}},
				},
			},
			namespace: "current-ns",
			scopes:    []string{ClusterRoleIndicator + "admin:*"},
			numRules:  2,
		},
		{
			name: "matching-namespaces",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{}},
				},
			},
			namespace: "current-ns",
			scopes:    []string{ClusterRoleIndicator + "admin:current-ns"},
			numRules:  2,
		},
		{
			name: "colon-role",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin:two"},
					Rules:      []authorizationapi.PolicyRule{{}},
				},
			},
			namespace: "current-ns",
			scopes:    []string{ClusterRoleIndicator + "admin:two:current-ns"},
			numRules:  2,
		},
		{
			name:            "getter-error",
			policyGetterErr: fmt.Errorf("some bad thing happened"),
			namespace:       "current-ns",
			scopes:          []string{ClusterRoleIndicator + "admin:two:current-ns"},
			err:             `some bad thing happened`,
			numRules:        1,
		},
	}

	for _, tc := range testCases {
		actualRules, actualErr := ScopesToRules(tc.scopes, tc.namespace, &fakePolicyGetter{clusterRoles: tc.clusterRoles, err: tc.policyGetterErr})
		switch {
		case len(tc.err) == 0 && actualErr == nil:
		case len(tc.err) == 0 && actualErr != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, actualErr)
		case len(tc.err) != 0 && actualErr == nil:
			t.Errorf("%s: missing error: %v", tc.name, tc.err)
		case len(tc.err) != 0 && actualErr != nil:
			if !strings.Contains(actualErr.Error(), tc.err) {
				t.Errorf("%s: expected %v, got %v", tc.name, tc.err, actualErr)
			}
		}

		if len(actualRules) != tc.numRules {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.numRules, len(actualRules))
		}
	}
}

func TestEscalationProtection(t *testing.T) {
	testCases := []struct {
		name      string
		scopes    []string
		namespace string

		clusterRoles  []authorizationapi.ClusterRole
		expectedRules []authorizationapi.PolicyRule
	}{
		{
			name: "simple match secrets",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{APIGroups: []string{""}, Resources: sets.NewString("pods", "secrets")}},
				},
			},
			expectedRules: []authorizationapi.PolicyRule{authorizationapi.DiscoveryRule, {APIGroups: []string{""}, Resources: sets.NewString("pods")}},
			scopes:        []string{ClusterRoleIndicator + "admin:*"},
		},
		{
			name: "match old group secrets",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{APIGroups: []string{}, Resources: sets.NewString("pods", "secrets")}},
				},
			},
			expectedRules: []authorizationapi.PolicyRule{authorizationapi.DiscoveryRule, {APIGroups: []string{}, Resources: sets.NewString("pods")}},
			scopes:        []string{ClusterRoleIndicator + "admin:*"},
		},
		{
			name: "skip non-matching group secrets",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{APIGroups: []string{"foo"}, Resources: sets.NewString("pods", "secrets")}},
				},
			},
			expectedRules: []authorizationapi.PolicyRule{authorizationapi.DiscoveryRule, {APIGroups: []string{"foo"}, Resources: sets.NewString("pods", "secrets")}},
			scopes:        []string{ClusterRoleIndicator + "admin:*"},
		},
		{
			name: "access tokens",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{APIGroups: []string{"", "and-foo"}, Resources: sets.NewString("pods", "oauthaccesstokens")}},
				},
			},
			expectedRules: []authorizationapi.PolicyRule{authorizationapi.DiscoveryRule, {APIGroups: []string{"", "and-foo"}, Resources: sets.NewString("pods")}},
			scopes:        []string{ClusterRoleIndicator + "admin:*"},
		},
		{
			name: "allow the escalation",
			clusterRoles: []authorizationapi.ClusterRole{
				{
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{APIGroups: []string{""}, Resources: sets.NewString("pods", "secrets")}},
				},
			},
			expectedRules: []authorizationapi.PolicyRule{authorizationapi.DiscoveryRule, {APIGroups: []string{""}, Resources: sets.NewString("pods", "secrets")}},
			scopes:        []string{ClusterRoleIndicator + "admin:*:!"},
		},
	}

	for _, tc := range testCases {
		actualRules, actualErr := ScopesToRules(tc.scopes, "ns-01", &fakePolicyGetter{clusterRoles: tc.clusterRoles})
		if actualErr != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, actualErr)
		}

		if !reflect.DeepEqual(actualRules, tc.expectedRules) {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedRules, actualRules)
		}
	}
}

type fakePolicyGetter struct {
	clusterRoles []authorizationapi.ClusterRole
	err          error
}

func (f *fakePolicyGetter) List(kapi.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	policy, err := f.Get("")
	if err != nil {
		return nil, err
	}

	ret := &authorizationapi.ClusterPolicyList{}
	ret.Items = append(ret.Items, *policy)
	return ret, f.err
}

func (f *fakePolicyGetter) Get(id string) (*authorizationapi.ClusterPolicy, error) {
	ret := &authorizationapi.ClusterPolicy{}
	ret.Roles = map[string]*authorizationapi.ClusterRole{}
	for i := range f.clusterRoles {
		ret.Roles[f.clusterRoles[i].Name] = &f.clusterRoles[i]
	}
	return ret, f.err
}
