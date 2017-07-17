package policy

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
)

// Registry is an interface for things that know how to store Policies.
type Registry interface {
	// ListPolicies obtains list of policies that match a selector.
	ListPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyList, error)
	// GetPolicy retrieves a specific policy.
	GetPolicy(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.Policy, error)
	// CreatePolicy creates a new policy.
	CreatePolicy(ctx apirequest.Context, policy *authorizationapi.Policy) error
	// UpdatePolicy updates a policy.
	UpdatePolicy(ctx apirequest.Context, policy *authorizationapi.Policy) error
	// DeletePolicy deletes a policy.
	DeletePolicy(ctx apirequest.Context, id string) error
}

type WatchingRegistry interface {
	Registry
	// WatchPolicies watches policies.
	WatchPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.StandardStorage
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) WatchingRegistry {
	return &storage{s}
}

func (s *storage) ListPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyList), nil
}

func (s *storage) CreatePolicy(ctx apirequest.Context, node *authorizationapi.Policy) error {
	_, err := s.Create(ctx, node, false)
	return err
}

func (s *storage) UpdatePolicy(ctx apirequest.Context, node *authorizationapi.Policy) error {
	_, _, err := s.Update(ctx, node.Name, rest.DefaultUpdatedObjectInfo(node, kapi.Scheme))
	return err
}

func (s *storage) WatchPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetPolicy(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.Policy, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.Policy), nil
}

func (s *storage) DeletePolicy(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

type ReadOnlyPolicyListerNamespacer struct {
	Registry Registry
}

func (s ReadOnlyPolicyListerNamespacer) Policies(namespace string) authorizationlister.PolicyNamespaceLister {
	return readOnlyPolicyLister{registry: s.Registry, namespace: namespace}
}

func (s ReadOnlyPolicyListerNamespacer) List(label labels.Selector) ([]*authorizationapi.Policy, error) {
	return s.Policies("").List(label)
}

type readOnlyPolicyLister struct {
	registry  Registry
	namespace string
}

func (s readOnlyPolicyLister) List(label labels.Selector) ([]*authorizationapi.Policy, error) {
	list, err := s.registry.ListPolicies(apirequest.WithNamespace(apirequest.NewContext(), s.namespace), &metainternal.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	var items []*authorizationapi.Policy
	for i := range list.Items {
		items = append(items, &list.Items[i])
	}
	return items, nil
}

func (s readOnlyPolicyLister) Get(name string) (*authorizationapi.Policy, error) {
	return s.registry.GetPolicy(apirequest.WithNamespace(apirequest.NewContext(), s.namespace), name, &metav1.GetOptions{})
}
