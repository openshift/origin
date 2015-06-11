package rulevalidation

import (
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type escalationTest struct {
	ownerRules   []authorizationapi.PolicyRule
	servantRules []authorizationapi.PolicyRule

	expectedCovered        bool
	expectedUncoveredRules []authorizationapi.PolicyRule
}

func TestExactMatch(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("builds")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("builds")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestMultipleRulesCoveringSingleRule(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("deployments")},
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("builds")},
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("builds", "deployments")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("builds", "deployments")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)

}

func TestMultipleRulesMissingSingleVerbResourceCombination(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("builds", "deployments")},
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("pods")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("builds", "deployments", "pods")},
		},

		expectedCovered: false,
		expectedUncoveredRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("pods")},
		},
	}.test(t)
}

func TestResourceGroupCoveringEnumerated(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("create", "delete", "update"), Resources: util.NewStringSet("resourcegroup:builds")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("builds", "buildconfigs")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestEnumeratedCoveringResourceGroup(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("builds", "buildconfigs", "buildlogs", "buildconfigs/instantiate", "builds/log", "builds/clone", "buildconfigs/webhooks")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("resourcegroup:builds")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestEnumeratedMissingPartOfResourceGroup(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("builds", "buildconfigs")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete", "update"), Resources: util.NewStringSet("resourcegroup:builds")},
		},

		expectedCovered: false,
		expectedUncoveredRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("buildlogs")},
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("buildlogs")},
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("buildconfigs/instantiate")},
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("buildconfigs/instantiate")},
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("builds/log")},
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("builds/log")},
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("builds/clone")},
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("builds/clone")},
			{Verbs: util.NewStringSet("delete"), Resources: util.NewStringSet("buildconfigs/webhooks")},
			{Verbs: util.NewStringSet("update"), Resources: util.NewStringSet("buildconfigs/webhooks")},
		},
	}.test(t)
}

func TestVerbStarCoveringMultiple(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("roles")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("watch", "list"), Resources: util.NewStringSet("roles")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestEnumerationNotCoveringVerbStar(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get", "list", "watch", "create", "update", "delete", "exec"), Resources: util.NewStringSet("roles")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("roles")},
		},

		expectedCovered: false,
		expectedUncoveredRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("roles")},
		},
	}.test(t)
}

func TestVerbStarCoveringStar(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("roles")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("roles")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestResourceStarCoveringMultiple(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("*")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("resourcegroup:deployments")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestEnumerationNotCoveringResourceStar(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("roles", "resourcegroup:deployments")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("*")},
		},

		expectedCovered: false,
		expectedUncoveredRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("*")},
		},
	}.test(t)
}

func TestResourceStarCoveringStar(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("*")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("*")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestResourceNameEmptyCoveringMultiple(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("pods"), ResourceNames: util.NewStringSet()},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("pods"), ResourceNames: util.NewStringSet("foo", "bar")},
		},

		expectedCovered:        true,
		expectedUncoveredRules: []authorizationapi.PolicyRule{},
	}.test(t)
}

func TestEnumerationNotCoveringResourceNameEmpty(t *testing.T) {
	escalationTest{
		ownerRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("pods"), ResourceNames: util.NewStringSet("foo", "bar")},
		},
		servantRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("pods"), ResourceNames: util.NewStringSet()},
		},

		expectedCovered: false,
		expectedUncoveredRules: []authorizationapi.PolicyRule{
			{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("pods")},
		},
	}.test(t)
}

func (test escalationTest) test(t *testing.T) {
	actualCovered, actualUncoveredRules := Covers(test.ownerRules, test.servantRules)

	if actualCovered != test.expectedCovered {
		t.Errorf("expected %v, but got %v", test.expectedCovered, actualCovered)
	}

	if !rulesMatch(test.expectedUncoveredRules, actualUncoveredRules) {
		t.Errorf("expected %v, but got %v", test.expectedUncoveredRules, actualUncoveredRules)
	}
}

func rulesMatch(expectedRules, actualRules []authorizationapi.PolicyRule) bool {
	if len(expectedRules) != len(actualRules) {
		return false
	}

	for _, expectedRule := range expectedRules {
		found := false
		for _, actualRule := range actualRules {
			if reflect.DeepEqual(expectedRule, actualRule) {
				found = true
			}
		}

		if !found {
			return false
		}
	}

	return true
}
