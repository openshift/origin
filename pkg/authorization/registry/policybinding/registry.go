package policybinding

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store PolicyBindings.
type Registry interface {
	// ListPolicyBindings obtains list of policyBindings that match a selector.
	ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error)
	// GetPolicyBinding retrieves a specific policyBinding.
	GetPolicyBinding(ctx kapi.Context, name string) (*authorizationapi.PolicyBinding, error)
	// CreatePolicyBinding creates a new policyBinding.
	CreatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error
	// UpdatePolicyBinding updates a policyBinding.
	UpdatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error
	// DeletePolicyBinding deletes a policyBinding.
	DeletePolicyBinding(ctx kapi.Context, name string) error
}

type WatchingRegistry interface {
	Registry
	// WatchPolicyBindings watches policyBindings.
	WatchPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
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

func (s *storage) ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyBindingList), nil
}

func (s *storage) CreatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error {
	_, err := s.Create(ctx, policyBinding)
	return err
}

func (s *storage) UpdatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error {
	_, _, err := s.Update(ctx, policyBinding)
	return err
}

func (s *storage) WatchPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}

func (s *storage) GetPolicyBinding(ctx kapi.Context, name string) (*authorizationapi.PolicyBinding, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.PolicyBinding), nil
}

func (s *storage) DeletePolicyBinding(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}
