package auth

import (
	"fmt"
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	authorizationlister "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	"k8s.io/kubernetes/pkg/controller"
)

type MockRoleGetter struct{}

func (m MockRoleGetter) Get(name string) (*rbac.Role, error) {
	return &rbac.Role{}, nil
}
func (m MockRoleGetter) List(labels.Selector) ([]*rbac.Role, error) {
	return nil, nil
}
func (m MockRoleGetter) Roles(namespace string) authorizationlister.RoleNamespaceLister {
	return m
}
func (m MockRoleGetter) LastSyncResourceVersion() string { return "" }

type MockClusterRoleGetter struct{}

func (m MockClusterRoleGetter) List(labels.Selector) ([]*rbac.ClusterRole, error) {
	return nil, nil
}
func (m MockClusterRoleGetter) Get(name string) (*rbac.ClusterRole, error) {
	return &rbac.ClusterRole{}, nil
}
func (m MockClusterRoleGetter) LastSyncResourceVersion() string { return "" }

type MockRoleBindingGetter struct{}

func (m MockRoleBindingGetter) Get(name string) (*rbac.RoleBinding, error) {
	return &rbac.RoleBinding{}, nil
}
func (m MockRoleBindingGetter) List(labels.Selector) ([]*rbac.RoleBinding, error) {
	return nil, nil
}
func (m MockRoleBindingGetter) RoleBindings(namespace string) authorizationlister.RoleBindingNamespaceLister {
	return m
}
func (m MockRoleBindingGetter) LastSyncResourceVersion() string { return "" }

type MockClusterRoleBindingGetter struct{}

func (m MockClusterRoleBindingGetter) List(labels.Selector) ([]*rbac.ClusterRoleBinding, error) {
	return nil, nil
}
func (m MockClusterRoleBindingGetter) Get(name string) (*rbac.ClusterRoleBinding, error) {
	return &rbac.ClusterRoleBinding{}, nil
}
func (m MockClusterRoleBindingGetter) LastSyncResourceVersion() string { return "" }

// mockReview implements the Review interface for test cases
type mockReview struct {
	users  []string
	groups []string
	err    string
}

// Users returns the users that can access a resource
func (r *mockReview) Users() []string {
	return r.users
}

// Groups returns the groups that can access a resource
func (r *mockReview) Groups() []string {
	return r.groups
}

func (r *mockReview) EvaluationError() string {
	return r.err
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
		return nil, fmt.Errorf("Item %s does not exist", name)
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
				ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bar", ResourceVersion: "2"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "car", ResourceVersion: "3"},
			},
		},
	}
	mockKubeClient := fake.NewSimpleClientset(&namespaceList)

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

	informers := informers.NewSharedInformerFactory(mockKubeClient, controller.NoResyncPeriodFunc())

	authorizationCache := NewAuthorizationCache(
		informers.Core().InternalVersion().Namespaces().Informer(),
		reviewer,
		informers.Rbac().InternalVersion(),
	)
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
