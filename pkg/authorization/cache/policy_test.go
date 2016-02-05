package cache

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

func beforeTestingSetup_readonlypolicycache() (testCache *readOnlyPolicyCache, cacheChannel, testChannel chan struct{}) {
	cacheChannel = make(chan struct{})

	testRegistry := testregistry.NewPolicyRegistry(testPolicies, nil)
	testCache = NewReadOnlyPolicyCache(testRegistry)

	testCache.RunUntil(cacheChannel)

	testChannel = make(chan struct{})
	return
}

// TestPolicyGet tests that a Get() call to the ReadOnlyPolicyCache will retrieve the correct policy
func TestPolicyGet(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicycache()
	defer close(cacheChannel)

	var policy *authorizationapi.Policy
	var err error

	namespace := "namespaceTwo"
	name := "uniquePolicyName"

	util.Until(func() {
		policy, err = testCache.Get(name, namespace)

		if (err == nil) &&
			(policy != nil) &&
			(policy.Name == name) &&
			(policy.Namespace == namespace) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policy using ReadOnlyPolicyCache: %v", err)
	case policy == nil:
		t.Error("Policy is nil")
	case policy.Name != name:
		t.Errorf("Expected policy name to be '%s', was '%s'", name, policy.Name)
	case policy.Namespace != namespace:
		t.Errorf("Expected policy namespace to be '%s', was '%s'", namespace, policy.Namespace)
	}
}

// TestPolicyGetRespectingNamespaces tests that a Get() call to the ReadOnlyPolicyCache will retrieve the correct policy when the name is
// an nonUnique identifier but the set {name, namespace} is not
func TestPolicyGetRespectingNamespaces(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicycache()
	defer close(cacheChannel)

	var policy *authorizationapi.Policy
	var err error

	namespace := "namespaceOne"
	name := "nonUniquePolicyName"

	util.Until(func() {
		policy, err = testCache.Get(name, namespace)

		if (err == nil) &&
			(policy != nil) &&
			(policy.Name == name) &&
			(policy.Namespace == namespace) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policy using ReadOnlyPolicyCache: %v", err)
	case policy == nil:
		t.Error("Policy is nil")
	case policy.Name != name:
		t.Errorf("Expected policy name to be '%s', was '%s'", name, policy.Name)
	case policy.Namespace != namespace:
		t.Errorf("Expected policy namespace to be '%s', was '%s'", namespace, policy.Namespace)
	}
}

// TestPolicyList tests that a List() call for a namespace to the ReadOnlyPolicyCache will return all policies in that namespace
func TestPolicyList(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicycache()
	defer close(cacheChannel)

	var policies *authorizationapi.PolicyList
	var err error

	namespace := "namespaceTwo"

	util.Until(func() {
		policies, err = testCache.List(nil, namespace)

		if (err == nil) &&
			(policies != nil) &&
			(len(policies.Items) == 2) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policies using ReadOnlyPolicyCache: %v", err)
	case policies == nil:
		t.Error("PoliciesList is nil")
	case len(policies.Items) != 2:
		t.Errorf("Expected policyList to have 2 policies, had %d", len(policies.Items))
	}
}

// TestPolicyListNamespaceAll tests that a List() call for kapi.NamespaceAll to the ReadOnlyPolicyCache will return
// all policies in all namespaces
func TestPolicyListNamespaceAll(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicycache()
	defer close(cacheChannel)

	var policies *authorizationapi.PolicyList
	var err error

	namespace := kapi.NamespaceAll

	util.Until(func() {
		policies, err = testCache.List(nil, namespace)

		if (err == nil) &&
			(policies != nil) &&
			(len(policies.Items) == 3) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policies using ReadOnlyPolicyCache: %v", err)
	case policies == nil:
		t.Error("PoliciesList is nil")
	case len(policies.Items) != 3:
		t.Errorf("Expected policyList to have 3 policies, had %d", len(policies.Items))
	}
}

// TestPolicyListRespectingLabels tests that a List() call for some namespace, filtered with a label to the ReadOnlyPolicyCache
// will return all policies in that namespace matching that label
func TestPolicyListRespectingLabels(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicycache()
	defer close(cacheChannel)

	var policies *authorizationapi.PolicyList
	var err error

	desiredName := "nonUniquePolicyName"
	namespace := "namespaceTwo"
	key := "labelToMatchOn"
	operator := labels.EqualsOperator
	val := sets.NewString("someValue")
	requirement, err := labels.NewRequirement(key, operator, val)
	if err != nil {
		t.Errorf("labels.Selector misconstructed: %v", err)
	}

	label := labels.NewSelector().Add(*requirement)

	util.Until(func() {
		policies, err = testCache.List(&kapi.ListOptions{LabelSelector: label}, namespace)

		if (err == nil) &&
			(policies != nil) &&
			(len(policies.Items) == 1) &&
			(policies.Items[0].Name == desiredName) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policies using ReadOnlyPolicyCache: %v", err)
	case policies == nil:
		t.Error("PoliciesList is nil")
	case len(policies.Items) != 1:
		t.Errorf("Expected policyList to have 1 policy, had %d", len(policies.Items))
	case policies.Items[0].Name != desiredName:
		t.Errorf("Expected policy name to be '%s', was '%s'", desiredName, policies.Items[0].Name)
	}
}

// TestPolicyListRespectingFields tests that a List() call for some namespace, filtered with a field to the ReadOnlyPolicyCache
// will return all policies in that namespace matching that field
func TestPolicyListRespectingFields(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicycache()
	defer close(cacheChannel)

	var policies *authorizationapi.PolicyList
	var err error

	name := "uniquePolicyName"
	namespace := "namespaceTwo"
	field := fields.OneTermEqualSelector("metadata.name", name)

	util.Until(func() {
		policies, err = testCache.List(&kapi.ListOptions{FieldSelector: field}, namespace)

		if (err == nil) &&
			(policies != nil) &&
			(len(policies.Items) == 1) &&
			(policies.Items[0].Name == name) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policies using ReadOnlyPolicyCache: %v", err)
	case policies == nil:
		t.Error("PoliciesList is nil")
	case len(policies.Items) != 1:
		t.Errorf("Expected policyList to have 1 policy, had %d", len(policies.Items))
	case policies.Items[0].Name != name:
		t.Errorf("Expected policy name to be '%s', was '%s'", name, policies.Items[0].Name)
	}
}

var (
	testPolicies = []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "nonUniquePolicyName",
				Namespace: "namespaceOne",
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "nonUniquePolicyName",
				Namespace: "namespaceTwo",
				Labels: map[string]string{
					"labelToMatchOn": "someValue",
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "uniquePolicyName",
				Namespace: "namespaceTwo",
				Labels: map[string]string{
					"labelToMatchOn": "someOtherValue",
				},
			},
		},
	}
)
