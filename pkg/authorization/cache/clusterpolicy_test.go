package cache

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

func beforeTestingSetup_readonlyclusterpolicycache() (testCache readOnlyClusterPolicyCache, cacheChannel, testChannel chan struct{}) {
	cacheChannel = make(chan struct{})

	testRegistry := testregistry.NewClusterPolicyRegistry(testClusterPolicies, nil)
	testCache = NewReadOnlyClusterPolicyCache(testRegistry)

	testCache.RunUntil(cacheChannel)

	testChannel = make(chan struct{})
	return
}

// TestClusterPolicyGet tests that a Get() call to the ReadOnlyClusterPolicyCache will retrieve the correct clusterPolicy
func TestClusterPolicyGet(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicycache()
	defer close(cacheChannel)

	var clusterPolicy *authorizationapi.ClusterPolicy
	var err error

	name := "uniqueClusterPolicyName"

	util.Until(func() {
		clusterPolicy, err = testCache.Get(name)

		if (err == nil) &&
			(clusterPolicy != nil) &&
			(clusterPolicy.Name == name) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicy using ReadOnlyClusterPolicyCache: %v", err)
	case clusterPolicy == nil:
		t.Error("ClusterPolicy is nil.")
	case clusterPolicy.Name != name:
		t.Errorf("Expected clusterPolicy name to be '%s', was '%s'", name, clusterPolicy.Name)
	}
}

// TestClusterPolicyList tests that a List() call to the ReadOnlyClusterPolicyCache will return all clusterPolicies
func TestClusterPolicyList(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicycache()
	defer close(cacheChannel)

	var clusterPolicies *authorizationapi.ClusterPolicyList
	var err error

	label := labels.Everything()
	field := fields.Everything()

	util.Until(func() {
		clusterPolicies, err = testCache.List(label, field)

		if (err == nil) &&
			(clusterPolicies != nil) &&
			(len(clusterPolicies.Items) == 2) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyList using ReadOnlyClusterPolicyCache: %v", err)
	case clusterPolicies == nil:
		t.Error("ClusterPolicyList is nil.")
	case len(clusterPolicies.Items) != 2:
		t.Errorf("Expected clusterPolicyList to contain 2 clusterPolicies, contained %d", len(clusterPolicies.Items))
	}
}

// TestClusterPolicyListRespectingLabels tests that a List(), filtered with a label to the ReadOnlyClusterPolicyCache
// will return all clusterPolicies matching that label
func TestClusterPolicyListRespectingLabels(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicycache()
	defer close(cacheChannel)

	var clusterPolicies *authorizationapi.ClusterPolicyList
	var err error

	desiredName := "uniqueClusterPolicyName"
	key := "labelToMatchOn"
	operator := labels.EqualsOperator
	val := util.NewStringSet("someValue")
	requirement, err := labels.NewRequirement(key, operator, val)
	if err != nil {
		t.Errorf("labels.Selector misconstructed: %v", err)
	}

	label := labels.LabelSelector{*requirement}
	field := fields.Everything()

	util.Until(func() {
		clusterPolicies, err = testCache.List(label, field)

		if (err == nil) &&
			(clusterPolicies != nil) &&
			(len(clusterPolicies.Items) == 1) &&
			(clusterPolicies.Items[0].Name == desiredName) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyList with labelSelector using ReadOnlyClusterPolicyCache: %v", err)
	case clusterPolicies == nil:
		t.Error("ClusterPolicyList is nil.")
	case len(clusterPolicies.Items) != 1:
		t.Errorf("Expected clusterPolicyList to contain 2 clusterPolicies, contained %d", len(clusterPolicies.Items))
	case clusterPolicies.Items[0].Name != desiredName:
		t.Errorf("Expected label-selected clusterPolicy name to be '%s', was '%s'", desiredName, clusterPolicies.Items[0].Name)
	}
}

// TestClusterPolicyListRespectingFields tests that a List() call, filtered with a field to the ReadOnlyClusterPolicyCache
// will return all clusterPolicies matching that field
func TestClusterPolicyListRespectingFields(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicycache()
	defer close(cacheChannel)

	var clusterPolicies *authorizationapi.ClusterPolicyList
	var err error

	name := "uniqueClusterPolicyName"
	label := labels.Everything()
	field := fields.OneTermEqualSelector("metadata.name", name)

	util.Until(func() {
		clusterPolicies, err = testCache.List(label, field)

		if (err == nil) &&
			(clusterPolicies != nil) &&
			(len(clusterPolicies.Items) == 1) &&
			(clusterPolicies.Items[0].Name == name) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyList with fieldSelector using ReadOnlyClusterPolicyCache: %v", err)
	case clusterPolicies == nil:
		t.Error("ClusterPolicyList is nil.")
	case len(clusterPolicies.Items) != 1:
		t.Errorf("Expected clusterPolicyList to contain 2 clusterPolicies, contained %d", len(clusterPolicies.Items))
	case clusterPolicies.Items[0].Name != name:
		t.Errorf("Expected field-selected clusterPolicy name to be '%s', was '%s'", name, clusterPolicies.Items[0].Name)
	}
}

var (
	testClusterPolicies = []authorizationapi.ClusterPolicy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "uniqueClusterPolicyName",
				Namespace: "",
				Labels: map[string]string{
					"labelToMatchOn": "someValue",
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "otherUniqueClusterPolicyName",
				Namespace: "",
				Labels: map[string]string{
					"labelToMatchOn": "someOtherValue",
				},
			},
		},
	}
)
