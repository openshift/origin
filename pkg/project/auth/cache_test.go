package auth

import (
	"fmt"
	"strconv"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
)

// MockReadOnlyPolicyClient implements the ReadOnlyPolicyCache interface for testing
type MockReadOnlyPolicyClient struct{}

// Following methods enable the MockReadOnlyPolicyClient to implement the ReadOnlyPolicyCache interface

// Policies gives access to a read-only policy interface
func (this *MockReadOnlyPolicyClient) ReadOnlyPolicies(namespace string) client.ReadOnlyPolicyInterface {
	return MockReadOnlyPolicyGetter{}
}

type MockReadOnlyPolicyGetter struct{}

func (this MockReadOnlyPolicyGetter) List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyList, error) {
	return &authorizationapi.PolicyList{}, nil
}

func (this MockReadOnlyPolicyGetter) Get(name string) (*authorizationapi.Policy, error) {
	return &authorizationapi.Policy{}, nil
}

// ClusterPolicies gives access to a read-only cluster policy interface
func (this *MockReadOnlyPolicyClient) ReadOnlyClusterPolicies() client.ReadOnlyClusterPolicyInterface {
	return MockReadOnlyClusterPolicyGetter{}
}

type MockReadOnlyClusterPolicyGetter struct{}

func (this MockReadOnlyClusterPolicyGetter) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyList, error) {
	return &authorizationapi.ClusterPolicyList{}, nil
}

func (this MockReadOnlyClusterPolicyGetter) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	return &authorizationapi.ClusterPolicy{}, nil
}

// PolicyBindings gives access to a read-only policy binding interface
func (this *MockReadOnlyPolicyClient) ReadOnlyPolicyBindings(namespace string) client.ReadOnlyPolicyBindingInterface {
	return MockReadOnlyPolicyBindingGetter{}
}

type MockReadOnlyPolicyBindingGetter struct{}

func (this MockReadOnlyPolicyBindingGetter) List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error) {
	return &authorizationapi.PolicyBindingList{}, nil
}

func (this MockReadOnlyPolicyBindingGetter) Get(name string) (*authorizationapi.PolicyBinding, error) {
	return &authorizationapi.PolicyBinding{}, nil
}

// ClusterPolicyBindings gives access to a read-only cluster policy binding interface
func (this *MockReadOnlyPolicyClient) ReadOnlyClusterPolicyBindings() client.ReadOnlyClusterPolicyBindingInterface {
	return MockReadOnlyClusterPolicyBindingGetter{}
}

type MockReadOnlyClusterPolicyBindingGetter struct{}

func (this MockReadOnlyClusterPolicyBindingGetter) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error) {
	return &authorizationapi.ClusterPolicyBindingList{}, nil
}

func (this MockReadOnlyClusterPolicyBindingGetter) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	return &authorizationapi.ClusterPolicyBinding{}, nil
}

// LastSyncResourceVersion returns the resource version for the last sync performed
func (this *MockReadOnlyPolicyClient) LastSyncResourceVersion() string {
	return ""
}

func (this *MockReadOnlyPolicyClient) GetPolicy(ctx kapi.Context, name string) (*authorizationapi.Policy, error) {
	return &authorizationapi.Policy{}, nil
}

func (this *MockReadOnlyPolicyClient) ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error) {
	return &authorizationapi.PolicyBindingList{}, nil
}

func (this *MockReadOnlyPolicyClient) GetClusterPolicy(ctx kapi.Context, name string) (*authorizationapi.ClusterPolicy, error) {
	return &authorizationapi.ClusterPolicy{}, nil
}

func (this *MockReadOnlyPolicyClient) ListClusterPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error) {
	return &authorizationapi.ClusterPolicyBindingList{}, nil
}

// mockReview implements the Review interface for test cases
type mockReview struct {
	users  []string
	groups []string
}

// Users returns the users that can access a resource
func (r *mockReview) Users() []string {
	return r.users
}

// Groups returns the groups that can access a resource
func (r *mockReview) Groups() []string {
	return r.groups
}

// common test users
var (
	alice = &user.DefaultInfo{
		Name:   "Alice",
		UID:    "alice-uid",
		Groups: []string{},
	}
	bob = &user.DefaultInfo{
		Name:   "Bob",
		UID:    "bob-uid",
		Groups: []string{"employee"},
	}
	eve = &user.DefaultInfo{
		Name:   "Eve",
		UID:    "eve-uid",
		Groups: []string{"employee"},
	}
	frank = &user.DefaultInfo{
		Name:   "Frank",
		UID:    "frank-uid",
		Groups: []string{},
	}
)

// mockReviewer returns the specified values for each supplied resource
type mockReviewer struct {
	expectedResults map[string]*mockReview
}

// Review returns the mapped review from the mock object, or an error if none exists
func (mr *mockReviewer) Review(name string) (Review, error) {
	review := mr.expectedResults[name]
	if review == nil {
		return nil, fmt.Errorf("item %s does not exist", name)
	}
	return review, nil
}

func validateList(t *testing.T, lister Lister, user user.Info, expectedSet sets.String) {
	namespaceList, err := lister.List(user)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	results := sets.String{}
	for _, namespace := range namespaceList.Items {
		results.Insert(namespace.Name)
	}
	if results.Len() != expectedSet.Len() || !results.HasAll(expectedSet.List()...) {
		t.Errorf("User %v, Expected: %v, Actual: %v", user.GetName(), expectedSet, results)
	}
}

func TestSyncNamespace(t *testing.T) {
	namespaceList := kapi.NamespaceList{
		Items: []kapi.Namespace{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", ResourceVersion: "1"},
			},
			{
				ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "2"},
			},
			{
				ObjectMeta: kapi.ObjectMeta{Name: "car", ResourceVersion: "3"},
			},
		},
	}
	mockKubeClient := testclient.NewSimpleFake(&namespaceList)

	reviewer := &mockReviewer{
		expectedResults: map[string]*mockReview{
			"foo": {
				users:  []string{alice.GetName(), bob.GetName()},
				groups: eve.GetGroups(),
			},
			"bar": {
				users:  []string{frank.GetName(), eve.GetName()},
				groups: []string{"random"},
			},
			"car": {
				users:  []string{},
				groups: []string{},
			},
		},
	}

	mockPolicyCache := &MockReadOnlyPolicyClient{}

	authorizationCache := NewAuthorizationCache(reviewer, mockKubeClient.Namespaces(), mockPolicyCache)
	// we prime the data we need here since we are not running reflectors
	for i := range namespaceList.Items {
		authorizationCache.namespaceStore.Add(&namespaceList.Items[i])
	}

	// synchronize the cache
	authorizationCache.synchronize()

	validateList(t, authorizationCache, alice, sets.NewString("foo"))
	validateList(t, authorizationCache, bob, sets.NewString("foo"))
	validateList(t, authorizationCache, eve, sets.NewString("foo", "bar"))
	validateList(t, authorizationCache, frank, sets.NewString("bar"))

	// modify access rules
	reviewer.expectedResults["foo"].users = []string{bob.GetName()}
	reviewer.expectedResults["foo"].groups = []string{"random"}
	reviewer.expectedResults["bar"].users = []string{alice.GetName(), eve.GetName()}
	reviewer.expectedResults["bar"].groups = []string{"employee"}
	reviewer.expectedResults["car"].users = []string{bob.GetName(), eve.GetName()}
	reviewer.expectedResults["car"].groups = []string{"employee"}

	// modify resource version on each namespace to simulate a change had occurred to force cache refresh
	for i := range namespaceList.Items {
		namespace := namespaceList.Items[i]
		oldVersion, err := strconv.Atoi(namespace.ResourceVersion)
		if err != nil {
			t.Errorf("Bad test setup, resource versions should be numbered, %v", err)
		}
		newVersion := strconv.Itoa(oldVersion + 1)
		namespace.ResourceVersion = newVersion
		authorizationCache.namespaceStore.Add(&namespace)
	}

	// now refresh the cache (which is resource version aware)
	authorizationCache.synchronize()

	// make sure new rights hold
	validateList(t, authorizationCache, alice, sets.NewString("bar"))
	validateList(t, authorizationCache, bob, sets.NewString("foo", "bar", "car"))
	validateList(t, authorizationCache, eve, sets.NewString("bar", "car"))
	validateList(t, authorizationCache, frank, sets.NewString())
}
