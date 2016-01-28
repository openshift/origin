package cache

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

func beforeTestingSetup_readonlypolicybindingcache() (testCache *readOnlyPolicyBindingCache, cacheChannel, testChannel chan struct{}) {
	cacheChannel = make(chan struct{})

	testRegistry := testregistry.NewPolicyBindingRegistry(testPolicyBindings, nil)
	testCache = NewReadOnlyPolicyBindingCache(testRegistry)

	testCache.RunUntil(cacheChannel)

	testChannel = make(chan struct{})
	return
}

// TestPolicyBindingGet tests that a Get() call to the ReadOnlyPolicyBindingCache will retrieve the correct policy binding
func TestPolicyBindingGet(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicybindingcache()
	defer close(cacheChannel)

	var policyBinding *authorizationapi.PolicyBinding
	var err error

	namespace := "namespaceTwo"
	name := "uniquePolicyBindingName"

	util.Until(func() {
		policyBinding, err = testCache.Get(name, namespace)

		if (err == nil) &&
			(policyBinding != nil) &&
			(policyBinding.Name == name) &&
			(policyBinding.Namespace == namespace) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policyBinding using ReadOnlyPolicyBindingCache: %v", err)
	case policyBinding == nil:
		t.Error("PolicyBinding is nil.")
	case policyBinding.Name != name:
		t.Errorf("Expected policyBinding name to be '%s', was '%s'", name, policyBinding.Name)
	case policyBinding.Namespace != namespace:
		t.Errorf("Expected policyBinding namespace to be '%s', was '%s'", namespace, policyBinding.Namespace)
	}
}

// TestPolicyBindingGetRespectingNamespaces tests that a Get() call to the ReadOnlyPolicyBindingCache will retrieve the correct policy binding
// when the name is an nonUnique identifier but the set {name, namespace} is not
func TestPolicyBindingGetRespectingNamespaces(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicybindingcache()
	defer close(cacheChannel)

	var policyBinding *authorizationapi.PolicyBinding
	var err error

	namespace := "namespaceOne"
	name := "nonUniquePolicyBindingName"

	util.Until(func() {
		policyBinding, err = testCache.Get(name, namespace)

		if (err == nil) &&
			(policyBinding != nil) &&
			(policyBinding.Name == name) &&
			(policyBinding.Namespace == namespace) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policyBinding using ReadOnlyPolicyBindingCache: %v", err)
	case policyBinding == nil:
		t.Error("PolicyBinding is nil.")
	case policyBinding.Name != name:
		t.Errorf("Expected policyBinding name to be '%s', was '%s'", name, policyBinding.Name)
	case policyBinding.Namespace != namespace:
		t.Errorf("Expected policyBinding namespace to be '%s', was '%s'", namespace, policyBinding.Namespace)
	}
}

// TestPolicyBindingList tests that a List() call for a namespace to the ReadOnlyPolicyBindingCache will return all policyBindings in that namespace
func TestPolicyBindingList(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicybindingcache()
	defer close(cacheChannel)

	var policyBindings *authorizationapi.PolicyBindingList
	var err error

	namespace := "namespaceTwo"

	util.Until(func() {
		policyBindings, err = testCache.List(nil, namespace)

		if (err == nil) &&
			(policyBindings != nil) &&
			(len(policyBindings.Items) == 2) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policyBindingList using ReadOnlyPolicyBindingCache: %v", err)
	case policyBindings == nil:
		t.Error("PolicyBindingList is nil.")
	case len(policyBindings.Items) != 2:
		t.Errorf("Expected policyBindingList to have 2 items, had %d", len(policyBindings.Items))
	}
}

// TestPolicyBindingListNamespaceAll tests that a List() call for kapi.NamespaceAll to the ReadOnlyPolicyBindingCache will return
// all policyBindings in all namespaces
func TestPolicyBindingListNamespaceAll(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicybindingcache()
	defer close(cacheChannel)

	var policyBindings *authorizationapi.PolicyBindingList
	var err error

	namespace := kapi.NamespaceAll

	util.Until(func() {
		policyBindings, err = testCache.List(nil, namespace)

		if (err == nil) &&
			(policyBindings != nil) &&
			(len(policyBindings.Items) == 3) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policyBindingList using ReadOnlyPolicyBindingCache: %v", err)
	case policyBindings == nil:
		t.Error("PolicyBindingList is nil.")
	case len(policyBindings.Items) != 3:
		t.Errorf("Expected policyBindingList to have 3 items, had %d", len(policyBindings.Items))
	}
}

// TestPolicyBindingListRespectingLabels tests that a List() call for some namespace, filtered with a label to the ReadOnlyPolicyBindingCache
// will return all policyBindings in that namespace matching that label
func TestPolicyBindingListRespectingLabels(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicybindingcache()
	defer close(cacheChannel)

	var policyBindings *authorizationapi.PolicyBindingList
	var err error

	desiredName := "nonUniquePolicyBindingName"
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
		policyBindings, err = testCache.List(&kapi.ListOptions{LabelSelector: unversioned.LabelSelector{Selector: label}}, namespace)

		if (err == nil) &&
			(policyBindings != nil) &&
			(len(policyBindings.Items) == 1) &&
			(policyBindings.Items[0].Name == desiredName) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policyBindingList using ReadOnlyPolicyBindingCache: %v", err)
	case policyBindings == nil:
		t.Error("PolicyBindingList is nil.")
	case len(policyBindings.Items) != 1:
		t.Errorf("Expected policyBindingList to have 1 item, had %d", len(policyBindings.Items))
	case policyBindings.Items[0].Name != desiredName:
		t.Errorf("Expected policyBinding name to be '%s', was '%s'", desiredName, policyBindings.Items[0].Name)
	}
}

// TestPolicyBindingListRespectingFields tests that a List() call for some namespace, filtered with a field to the ReadOnlyPolicyBindingCache
// will return all policyBindings in that namespace matching that field
func TestPolicyBindingListRespectingFields(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlypolicybindingcache()
	defer close(cacheChannel)

	var policyBindings *authorizationapi.PolicyBindingList
	var err error

	name := "uniquePolicyBindingName"
	namespace := "namespaceTwo"
	field := fields.OneTermEqualSelector("metadata.name", name)

	util.Until(func() {
		policyBindings, err = testCache.List(&kapi.ListOptions{FieldSelector: unversioned.FieldSelector{Selector: field}}, namespace)

		if (err == nil) &&
			(policyBindings != nil) &&
			(len(policyBindings.Items) == 1) &&
			(policyBindings.Items[0].Name == name) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policyBindingList using ReadOnlyPolicyBindingCache: %v", err)
	case policyBindings == nil:
		t.Error("PolicyBindingList is nil.")
	case len(policyBindings.Items) != 1:
		t.Errorf("Expected policyBindingList to have 1 item, had %d", len(policyBindings.Items))
	case policyBindings.Items[0].Name != name:
		t.Errorf("Expected policyBinding name to be '%s', was '%s'", name, policyBindings.Items[0].Name)
	}
}

var (
	testPolicyBindings = []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "nonUniquePolicyBindingName",
				Namespace: "namespaceOne",
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "nonUniquePolicyBindingName",
				Namespace: "namespaceTwo",
				Labels: map[string]string{
					"labelToMatchOn": "someValue",
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "uniquePolicyBindingName",
				Namespace: "namespaceTwo",
				Labels: map[string]string{
					"labelToMatchOn": "someOtherValue",
				},
			},
		},
	}
)
