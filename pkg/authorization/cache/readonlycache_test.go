package cache

import (
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	testregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

func beforeTestingSetup_readonlycache() (testClient client.ReadOnlyPolicyClient, policyStopChannel, bindingStopChannel, testChannel chan struct{}) {
	policyStopChannel = make(chan struct{})
	bindingStopChannel = make(chan struct{})
	testChannel = make(chan struct{})

	policyRegistry := testregistry.NewPolicyRegistry(testPolicies, nil)
	clusterPolicyRegistry := testregistry.NewClusterPolicyRegistry(testClusterPolicies, nil)
	policyBindingRegistry := testregistry.NewPolicyBindingRegistry(testPolicyBindings, nil)
	clusterPolicyBindingRegistry := testregistry.NewClusterPolicyBindingRegistry(testClusterPolicyBindings, nil)

	testCache, testClient := NewReadOnlyCacheAndClient(policyBindingRegistry, policyRegistry, clusterPolicyBindingRegistry, clusterPolicyRegistry)
	testCache.RunUntil(bindingStopChannel, policyStopChannel)
	return
}

// TestGetLocalPolicy tests that a ReadOnlyPolicyClient GetPolicy() call correctly retrieves a local policy
func TestGetLocalPolicy(t *testing.T) {
	testClient, policyStopChannel, bindingStopChannel, testChannel := beforeTestingSetup_readonlycache()
	defer close(policyStopChannel)
	defer close(bindingStopChannel)

	var policy *authorizationapi.Policy
	var err error

	namespace := "namespaceTwo"
	context := kapi.WithNamespace(kapi.NewContext(), namespace)
	name := "uniquePolicyName"

	util.Until(func() {
		policy, err = testClient.GetPolicy(context, name)

		if (err == nil) &&
			(policy != nil) &&
			(policy.Name == name) &&
			(policy.Namespace == namespace) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policy using GetPolicy(): %v", err)
	case policy == nil:
		t.Error("Policy is nil")
	case policy.Name != name:
		t.Errorf("Expected policy.Name to be '%s', but got '%s'", name, policy.Name)
	case policy.Namespace != namespace:
		t.Errorf("Expected policy.Namespace to be '%s', but got '%s'", namespace, policy.Namespace)
	}
}

// TestGetClusterPolicy tests that a ReadOnlyPolicyClient GetPolicy() call correctly retrieves a cluster policy
// when the namespace given is equal to the empty string
func TestGetClusterPolicy(t *testing.T) {
	testClient, policyStopChannel, bindingStopChannel, testChannel := beforeTestingSetup_readonlycache()
	defer close(policyStopChannel)
	defer close(bindingStopChannel)

	var clusterPolicy *authorizationapi.Policy
	var err error

	namespace := ""
	context := kapi.WithNamespace(kapi.NewContext(), namespace)
	name := "uniqueClusterPolicyName"

	util.Until(func() {
		clusterPolicy, err = testClient.GetPolicy(context, name)

		if (err == nil) &&
			(clusterPolicy != nil) &&
			(clusterPolicy.Name == name) &&
			(clusterPolicy.Namespace == namespace) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting cluster policy using GetPolicy(): %v", err)
	case clusterPolicy == nil:
		t.Error("Policy is nil")
	case clusterPolicy.Name != name:
		t.Errorf("Expected policy.Name to be '%s', but got '%s'", name, clusterPolicy.Name)
	case clusterPolicy.Namespace != "":
		t.Errorf("Expected policy.Namespace to be '%s', but got '%s'", namespace, clusterPolicy.Namespace)
	}
}

// TestListLocalPolicyBindings tests that a ReadOnlyPolicyClient ListPolicyBindings() call correctly lists local policy bindings
func TestListLocalPolicyBindings(t *testing.T) {
	testClient, policyStopChannel, bindingStopChannel, testChannel := beforeTestingSetup_readonlycache()
	defer close(policyStopChannel)
	defer close(bindingStopChannel)

	var policyBindings *authorizationapi.PolicyBindingList
	var err error

	namespace := "namespaceTwo"
	context := kapi.WithNamespace(kapi.NewContext(), namespace)
	label := labels.Everything()
	field := fields.Everything()

	util.Until(func() {
		policyBindings, err = testClient.ListPolicyBindings(context, label, field)

		if (err == nil) &&
			(policyBindings != nil) &&
			(len(policyBindings.Items) == 2) &&
			(!strings.Contains(policyBindings.Items[0].Name, "Cluster")) &&
			(!strings.Contains(policyBindings.Items[1].Name, "Cluster")) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting policy binding using ListPolicyBindings(): %v", err)
	case policyBindings == nil:
		t.Error("PolicyBindingsList is nil")
	case len(policyBindings.Items) != 2:
		t.Errorf("PolicyBindingsList contains %d items, should contain 2.", len(policyBindings.Items))
	case strings.Contains(policyBindings.Items[0].Name, "Cluster") || strings.Contains(policyBindings.Items[1].Name, "Cluster"):
		t.Error("PolicyBinding name should not contain \"Cluster\", but did.")
	}
}

// TestListClusterPolicyBindings tests that a ReadOnlyPolicyClient ListPolicyBindings() call correctly lists cluster policy bindings
// when the namespace given is the empty string
func TestListClusterPolicyBindings(t *testing.T) {
	testClient, policyStopChannel, bindingStopChannel, testChannel := beforeTestingSetup_readonlycache()
	defer close(policyStopChannel)
	defer close(bindingStopChannel)

	var clusterPolicyBindings *authorizationapi.PolicyBindingList
	var err error

	namespace := ""
	context := kapi.WithNamespace(kapi.NewContext(), namespace)
	label := labels.Everything()
	field := fields.Everything()

	util.Until(func() {
		clusterPolicyBindings, err = testClient.ListPolicyBindings(context, label, field)

		if (err == nil) &&
			(clusterPolicyBindings != nil) &&
			(len(clusterPolicyBindings.Items) == 2) &&
			(strings.Contains(clusterPolicyBindings.Items[0].Name, "Cluster")) &&
			(strings.Contains(clusterPolicyBindings.Items[1].Name, "Cluster")) {
			close(testChannel)
		}
	}, 1*time.Millisecond, testChannel)

	switch {
	case err != nil:
		t.Errorf("Error getting cluster policy binding using ListPolicyBindings(): %v", err)
	case clusterPolicyBindings == nil:
		t.Error("ClusterPolicyBindingsList is nil")
	case len(clusterPolicyBindings.Items) != 2:
		t.Errorf("ClusterPolicyBindingsList contains %d items, should contain 2.", len(clusterPolicyBindings.Items))
	case !strings.Contains(clusterPolicyBindings.Items[0].Name, "Cluster") || !strings.Contains(clusterPolicyBindings.Items[1].Name, "Cluster"):
		t.Error("ClusterPolicyBinding name should contain \"Cluster\", but did not.")
	}
}
