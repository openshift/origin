package policy

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func role(rules []authorizationapi.PolicyRule, labels map[string]string, annotations map[string]string) *authorizationapi.ClusterRole {
	return &authorizationapi.ClusterRole{Rules: rules, ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: annotations}}
}

func rules(resources ...string) []authorizationapi.PolicyRule {
	r := []authorizationapi.PolicyRule{}
	for _, resource := range resources {
		r = append(r, authorizationapi.PolicyRule{Verbs: sets.NewString("get"), Resources: sets.NewString(resource)})
	}
	return r
}

type ss map[string]string

func TestComputeReconciledRole(t *testing.T) {
	tests := map[string]struct {
		expectedRole *authorizationapi.ClusterRole
		actualRole   *authorizationapi.ClusterRole
		union        bool

		expectedReconciledRole       *authorizationapi.ClusterRole
		expectedReconciliationNeeded bool
	}{
		"empty": {
			expectedRole: role(rules(), nil, nil),
			actualRole:   role(rules(), nil, nil),
			union:        false,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"match without union": {
			expectedRole: role(rules("a"), nil, nil),
			actualRole:   role(rules("a"), nil, nil),
			union:        false,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"match with union": {
			expectedRole: role(rules("a"), nil, nil),
			actualRole:   role(rules("a"), nil, nil),
			union:        true,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"different rules without union": {
			expectedRole: role(rules("a"), nil, nil),
			actualRole:   role(rules("b"), nil, nil),
			union:        false,

			expectedReconciledRole:       role(rules("a"), nil, nil),
			expectedReconciliationNeeded: true,
		},
		"different rules with union": {
			expectedRole: role(rules("a"), nil, nil),
			actualRole:   role(rules("b"), nil, nil),
			union:        true,

			expectedReconciledRole:       role(rules("a", "b"), nil, nil),
			expectedReconciliationNeeded: true,
		},
		"match labels without union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("a"), ss{"1": "a"}, nil),
			union:        false,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"match labels with union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("a"), ss{"1": "a"}, nil),
			union:        true,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"different labels without union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("a"), ss{"2": "b"}, nil),
			union:        false,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"different labels with union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("a"), ss{"2": "b"}, nil),
			union:        true,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"different labels and rules without union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("b"), ss{"2": "b"}, nil),
			union:        false,

			expectedReconciledRole:       role(rules("a"), ss{"2": "b"}, nil),
			expectedReconciliationNeeded: true,
		},
		"different labels and rules with union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("b"), ss{"2": "b"}, nil),
			union:        true,

			expectedReconciledRole:       role(rules("a", "b"), ss{"2": "b"}, nil),
			expectedReconciliationNeeded: true,
		},
		"conflicting labels and rules without union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("b"), ss{"1": "b"}, nil),
			union:        false,

			expectedReconciledRole:       role(rules("a"), ss{"1": "b"}, nil),
			expectedReconciliationNeeded: true,
		},
		"conflicting labels and rules with union": {
			expectedRole: role(rules("a"), ss{"1": "a"}, nil),
			actualRole:   role(rules("b"), ss{"1": "b"}, nil),
			union:        true,

			expectedReconciledRole:       role(rules("a", "b"), ss{"1": "b"}, nil),
			expectedReconciliationNeeded: true,
		},
		"match annotations without union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("a"), nil, ss{"1": "a"}),
			union:        false,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"match annotations with union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("a"), nil, ss{"1": "a"}),
			union:        true,

			expectedReconciledRole:       nil,
			expectedReconciliationNeeded: false,
		},
		"different annotations without union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("a"), nil, ss{"2": "b"}),
			union:        false,

			expectedReconciledRole:       role(rules("a"), nil, ss{"1": "a", "2": "b"}),
			expectedReconciliationNeeded: true,
		},
		"different annotations with union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("a"), nil, ss{"2": "b"}),
			union:        true,

			expectedReconciledRole:       role(rules("a"), nil, ss{"1": "a", "2": "b"}),
			expectedReconciliationNeeded: true,
		},
		"different annotations and rules without union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("b"), nil, ss{"2": "b"}),
			union:        false,

			expectedReconciledRole:       role(rules("a"), nil, ss{"1": "a", "2": "b"}),
			expectedReconciliationNeeded: true,
		},
		"different annotations and rules with union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("b"), nil, ss{"2": "b"}),
			union:        true,

			expectedReconciledRole:       role(rules("a", "b"), nil, ss{"1": "a", "2": "b"}),
			expectedReconciliationNeeded: true,
		},
		"conflicting annotations and rules without union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("b"), nil, ss{"1": "b"}),
			union:        false,

			expectedReconciledRole:       role(rules("a"), nil, ss{"1": "b"}),
			expectedReconciliationNeeded: true,
		},
		"conflicting annotations and rules with union": {
			expectedRole: role(rules("a"), nil, ss{"1": "a"}),
			actualRole:   role(rules("b"), nil, ss{"1": "b"}),
			union:        true,

			expectedReconciledRole:       role(rules("a", "b"), nil, ss{"1": "b"}),
			expectedReconciliationNeeded: true,
		},
		"conflicting labels/annotations and rules without union": {
			expectedRole: role(rules("a"), ss{"3": "d"}, ss{"1": "a"}),
			actualRole:   role(rules("b"), ss{"4": "e"}, ss{"1": "b"}),
			union:        false,

			expectedReconciledRole:       role(rules("a"), ss{"4": "e"}, ss{"1": "b"}),
			expectedReconciliationNeeded: true,
		},
		"conflicting labels/annotations and rules with union": {
			expectedRole: role(rules("a"), ss{"3": "d"}, ss{"1": "a"}),
			actualRole:   role(rules("b"), ss{"4": "e"}, ss{"1": "b"}),
			union:        true,

			expectedReconciledRole:       role(rules("a", "b"), ss{"4": "e"}, ss{"1": "b"}),
			expectedReconciliationNeeded: true,
		},
		"complex labels/annotations and rules without union": {
			expectedRole: role(rules("pods", "nodes", "secrets"), ss{"env": "prod", "color": "blue"}, ss{"description": "fancy", "system": "true"}),
			actualRole:   role(rules("nodes", "images", "projects"), ss{"color": "red", "team": "pm"}, ss{"system": "false", "owner": "admin", "vip": "yes"}),
			union:        false,

			expectedReconciledRole:       role(rules("pods", "nodes", "secrets"), ss{"color": "red", "team": "pm"}, ss{"description": "fancy", "system": "false", "owner": "admin", "vip": "yes"}),
			expectedReconciliationNeeded: true,
		},
		"complex labels/annotations and rules with union": {
			expectedRole: role(rules("pods", "nodes", "secrets"), ss{"env": "prod", "color": "blue", "manager": "randy"}, ss{"description": "fancy", "system": "true", "up": "true"}),
			actualRole:   role(rules("nodes", "images", "projects"), ss{"color": "red", "team": "pm"}, ss{"system": "false", "owner": "admin", "vip": "yes", "rate": "down"}),
			union:        true,

			expectedReconciledRole:       role(rules("pods", "nodes", "secrets", "images", "projects"), ss{"color": "red", "team": "pm"}, ss{"description": "fancy", "system": "false", "owner": "admin", "vip": "yes", "rate": "down", "up": "true"}),
			expectedReconciliationNeeded: true,
		},
	}

	for k, tc := range tests {
		reconciledRole, reconciliationNeeded := computeReconciledRole(*tc.expectedRole, *tc.actualRole, tc.union)
		if reconciliationNeeded != tc.expectedReconciliationNeeded {
			t.Errorf("%s: Expected\n\t%v\ngot\n\t%v", k, tc.expectedReconciliationNeeded, reconciliationNeeded)
		}
		if !kapihelper.Semantic.DeepEqual(reconciledRole, tc.expectedReconciledRole) {
			t.Errorf("%s: Expected\n\t%#v\ngot\n\t%#v", k, tc.expectedReconciledRole, reconciledRole)
		}
	}
}
