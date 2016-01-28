package policy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store Policies.
type Registry interface {
	// ListPolicies obtains list of policies that match a selector.
	ListPolicies(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.PolicyList, error)
	// GetPolicy retrieves a specific policy.
	GetPolicy(ctx kapi.Context, id string) (*authorizationapi.Policy, error)
	// CreatePolicy creates a new policy.
	CreatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error
	// UpdatePolicy updates a policy.
	UpdatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error
	// DeletePolicy deletes a policy.
	DeletePolicy(ctx kapi.Context, id string) error
}

type WatchingRegistry interface {
	Registry
	// WatchPolicies watches policies.
	WatchPolicies(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
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

func (s *storage) ListPolicies(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.PolicyList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyList), nil
}

func (s *storage) CreatePolicy(ctx kapi.Context, node *authorizationapi.Policy) error {
	_, err := s.Create(ctx, node)
	return err
}

func (s *storage) UpdatePolicy(ctx kapi.Context, node *authorizationapi.Policy) error {
	_, _, err := s.Update(ctx, node)
	return err
}

func (s *storage) WatchPolicies(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetPolicy(ctx kapi.Context, name string) (*authorizationapi.Policy, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.Policy), nil
}

func (s *storage) DeletePolicy(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}
