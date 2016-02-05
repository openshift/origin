package clusterpolicybinding

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

// Registry is an interface for things that know how to store ClusterPolicyBindings.
type Registry interface {
	// ListClusterPolicyBindings obtains list of policyBindings that match a selector.
	ListClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
	// GetClusterPolicyBinding retrieves a specific policyBinding.
	GetClusterPolicyBinding(ctx kapi.Context, name string) (*authorizationapi.ClusterPolicyBinding, error)
	// CreateClusterPolicyBinding creates a new policyBinding.
	CreateClusterPolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error
	// UpdateClusterPolicyBinding updates a policyBinding.
	UpdateClusterPolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error
	// DeleteClusterPolicyBinding deletes a policyBinding.
	DeleteClusterPolicyBinding(ctx kapi.Context, name string) error
}

type WatchingRegistry interface {
	Registry
	// WatchClusterPolicyBindings watches policyBindings.
	WatchClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
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

func (s *storage) ListClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBindingList), nil
}

func (s *storage) CreateClusterPolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	_, err := s.Create(ctx, policyBinding)
	return err
}

func (s *storage) UpdateClusterPolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	_, _, err := s.Update(ctx, policyBinding)
	return err
}

func (s *storage) WatchClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetClusterPolicyBinding(ctx kapi.Context, name string) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterPolicyBinding), nil
}

func (s *storage) DeleteClusterPolicyBinding(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}

type simulatedStorage struct {
	clusterRegistry Registry
}

func NewSimulatedRegistry(clusterRegistry Registry) policybinding.Registry {
	return &simulatedStorage{clusterRegistry}
}

func (s *simulatedStorage) ListPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	ret, err := s.clusterRegistry.ListClusterPolicyBindings(ctx, options)
	return authorizationapi.ToPolicyBindingList(ret), err
}

func (s *simulatedStorage) CreatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error {
	return s.clusterRegistry.CreateClusterPolicyBinding(ctx, authorizationapi.ToClusterPolicyBinding(policyBinding))
}

func (s *simulatedStorage) UpdatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error {
	return s.clusterRegistry.UpdateClusterPolicyBinding(ctx, authorizationapi.ToClusterPolicyBinding(policyBinding))
}

func (s *simulatedStorage) GetPolicyBinding(ctx kapi.Context, name string) (*authorizationapi.PolicyBinding, error) {
	ret, err := s.clusterRegistry.GetClusterPolicyBinding(ctx, name)
	return authorizationapi.ToPolicyBinding(ret), err
}

func (s *simulatedStorage) DeletePolicyBinding(ctx kapi.Context, name string) error {
	return s.clusterRegistry.DeleteClusterPolicyBinding(ctx, name)
}
