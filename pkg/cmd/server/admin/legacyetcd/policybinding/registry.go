package policybinding

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

// Registry is an interface for things that know how to store PolicyBindings.
type Registry interface {
	// ListPolicyBindings obtains list of policyBindings that match a selector.
	ListPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyBindingList, error)
	// GetPolicyBinding retrieves a specific policyBinding.
	GetPolicyBinding(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.PolicyBinding, error)
	// CreatePolicyBinding creates a new policyBinding.
	CreatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error
	// UpdatePolicyBinding updates a policyBinding.
	UpdatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error
	// DeletePolicyBinding deletes a policyBinding.
	DeletePolicyBinding(ctx apirequest.Context, name string) error
}

type WatchingRegistry interface {
	Registry
	// WatchPolicyBindings watches policyBindings.
	WatchPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
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

func (s *storage) ListPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyBindingList), nil
}

func (s *storage) CreatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error {
	_, err := s.Create(ctx, policyBinding, false)
	return err
}

func (s *storage) UpdatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error {
	_, _, err := s.Update(ctx, policyBinding.Name, rest.DefaultUpdatedObjectInfo(policyBinding, kapi.Scheme))
	return err
}

func (s *storage) WatchPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetPolicyBinding(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.PolicyBinding, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.PolicyBinding), nil
}

func (s *storage) DeletePolicyBinding(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

type ReadOnlyPolicyBindingListerNamespacer struct {
	Registry Registry
}

func (s ReadOnlyPolicyBindingListerNamespacer) PolicyBindings(namespace string) authorizationlister.PolicyBindingNamespaceLister {
	return policyBindingLister{registry: s.Registry, namespace: namespace}
}

func (s ReadOnlyPolicyBindingListerNamespacer) List(label labels.Selector) ([]*authorizationapi.PolicyBinding, error) {
	return s.PolicyBindings("").List(label)
}

type policyBindingLister struct {
	registry  Registry
	namespace string
}

func (s policyBindingLister) List(label labels.Selector) ([]*authorizationapi.PolicyBinding, error) {
	list, err := s.registry.ListPolicyBindings(apirequest.WithNamespace(apirequest.NewContext(), s.namespace), &metainternal.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	var items []*authorizationapi.PolicyBinding
	for i := range list.Items {
		items = append(items, &list.Items[i])
	}
	return items, nil
}

func (s policyBindingLister) Get(name string) (*authorizationapi.PolicyBinding, error) {
	return s.registry.GetPolicyBinding(apirequest.WithNamespace(apirequest.NewContext(), s.namespace), name, &metav1.GetOptions{})
}
