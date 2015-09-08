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

func beforeTestingSetup_readonlyclusterpolicybindingcache() (testCache readOnlyClusterPolicyBindingCache, cacheChannel, testChannel chan struct{}) {
	cacheChannel = make(chan struct{})

	testRegistry := testregistry.NewClusterPolicyBindingRegistry(testClusterPolicyBindings, nil)
	testCache = NewReadOnlyClusterPolicyBindingCache(testRegistry)

	testCache.RunUntil(cacheChannel)

	testChannel = make(chan struct{})
	return
}

// TestClusterPolicyBindingGet tests that a Get() call to the ReadOnlyClusterPolicyBindingCache will retrieve the correct clusterPolicyBinding
func TestClusterPolicyBindingGet(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicybindingcache()
	defer close(cacheChannel)

	var clusterPolicyBinding *authorizationapi.ClusterPolicyBinding
	var err error

	name := "uniqueClusterPolicyBindingName"

	util.Until(func() {
		clusterPolicyBinding, err = testCache.Get(name)

		if (err == nil) &&
			(clusterPolicyBinding != nil) &&
			(clusterPolicyBinding.Name == name) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyBinding using ReadOnlyClusterBindingCache: %v", err)
	case clusterPolicyBinding == nil:
		t.Error("ClusterPolicyBinding is nil")
	case clusterPolicyBinding.Name != name:
		t.Errorf("Expected clusterPolicyBindingName to be '%s', was '%s'", name, clusterPolicyBinding.Name)
	}
}

// TestClusterPolicyBindingList tests that a List() call to the ReadOnlyClusterPolicyBindingCache will return all clusterPolicyBindings
func TestClusterPolicyBindingList(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicybindingcache()
	defer close(cacheChannel)

	var clusterPolicyBindings *authorizationapi.ClusterPolicyBindingList
	var err error

	label := labels.Everything()
	field := fields.Everything()

	util.Until(func() {
		clusterPolicyBindings, err = testCache.List(label, field)

		if (err == nil) &&
			(clusterPolicyBindings != nil) &&
			(len(clusterPolicyBindings.Items) == 2) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyBindingList using ReadOnlyClusterBindingCache: %v", err)
	case clusterPolicyBindings == nil:
		t.Error("ClusterPolicyBindingList is nil")
	case len(clusterPolicyBindings.Items) != 2:
		t.Errorf("Expected clusterPolicyBindingList to contain 2 items, had %d", len(clusterPolicyBindings.Items))
	}
}

// TestClusterPolicyBindingListRespectingLabels tests that a List(), filtered with a label to the ReadOnlyClusterPolicyBindingCache
// will return all clusterPolicyBindings matching that label
func TestClusterPolicyBindingListRespectingLabels(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicybindingcache()
	defer close(cacheChannel)

	var clusterPolicyBindings *authorizationapi.ClusterPolicyBindingList
	var err error

	desiredName := "uniqueClusterPolicyBindingName"
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
		clusterPolicyBindings, err = testCache.List(label, field)

		if (err == nil) &&
			(clusterPolicyBindings != nil) &&
			(len(clusterPolicyBindings.Items) == 1) &&
			(clusterPolicyBindings.Items[0].Name == desiredName) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyBinding with labelSelector using ReadOnlyClusterBindingCache: %v", err)
	case clusterPolicyBindings == nil:
		t.Error("ClusterPolicyBindingList using labelSelector is nil")
	case len(clusterPolicyBindings.Items) != 1:
		t.Errorf("Expected clusterPolicyBindingList using labelSelector to contain 1 item, had %d", len(clusterPolicyBindings.Items))
	case clusterPolicyBindings.Items[0].Name != desiredName:
		t.Errorf("Expected clusterPolicyBinding to have name '%s', had '%s'", desiredName, clusterPolicyBindings.Items[0].Name)
	}
}

// TestClusterPolicyBindingListRespectingFields tests that a List() call, filtered with a field to the ReadOnlyClusterPolicyBindingCache
// will return all clusterPolicyBindings matching that field
func TestClusterPolicyBindingListRespectingFields(t *testing.T) {
	testCache, cacheChannel, testChannel := beforeTestingSetup_readonlyclusterpolicybindingcache()
	defer close(cacheChannel)

	var clusterPolicyBindings *authorizationapi.ClusterPolicyBindingList
	var err error

	name := "uniqueClusterPolicyBindingName"
	label := labels.Everything()
	field := fields.OneTermEqualSelector("metadata.name", name)

	util.Until(func() {
		clusterPolicyBindings, err = testCache.List(label, field)

		if (err == nil) &&
			(clusterPolicyBindings != nil) &&
			(len(clusterPolicyBindings.Items) == 1) &&
			(clusterPolicyBindings.Items[0].Name == name) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting clusterPolicyBinding with fieldSelector using ReadOnlyClusterBindingCache: %v", err)
	case clusterPolicyBindings == nil:
		t.Error("ClusterPolicyBindingList using fieldSelector is nil")
	case len(clusterPolicyBindings.Items) != 1:
		t.Errorf("Expected clusterPolicyBindingList using fieldSelector to contain 1 items, had %d", len(clusterPolicyBindings.Items))
	case clusterPolicyBindings.Items[0].Name != name:
		t.Errorf("Expected clusterPolicyBinding to have name '%s', had '%s'", name, clusterPolicyBindings.Items[0].Name)
	}
}

var (
	testClusterPolicyBindings = []authorizationapi.ClusterPolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "uniqueClusterPolicyBindingName",
				Namespace: "",
				Labels: map[string]string{
					"labelToMatchOn": "someValue",
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "otherUniqueClusterPolicyBindingName",
				Namespace: "",
				Labels: map[string]string{
					"labelToMatchOn": "someOtherValue",
				},
			},
		},
	}
)
