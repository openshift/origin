package clusterpolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/policy"
)

// Registry is an interface for things that know how to store ClusterPolicies.
type Registry interface {
	// ListClusterPolicies obtains list of policies that match a selector.
	ListClusterPolicies(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.ClusterPolicyList, error)
	// GetClusterPolicy retrieves a specific policy.
	GetClusterPolicy(ctx kapi.Context, id string) (*authorizationapi.ClusterPolicy, error)
	// CreateClusterPolicy creates a new policy.
	CreateClusterPolicy(ctx kapi.Context, policy *authorizationapi.ClusterPolicy) error
	// UpdateClusterPolicy updates a policy.
	UpdateClusterPolicy(ctx kapi.Context, policy *authorizationapi.ClusterPolicy) error
	// DeleteClusterPolicy deletes a policy.
	DeleteClusterPolicy(ctx kapi.Context, id string) error
}

type WatchingRegistry interface {
	Registry
	// WatchClusterPolicies watches policies.
	WatchClusterPolicies(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
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

func (s *storage) ListClusterPolicies(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyList), nil
}

func (s *storage) CreateClusterPolicy(ctx kapi.Context, node *authorizationapi.ClusterPolicy) error {
	_, err := s.Create(ctx, node)
	return err
}

func (s *storage) UpdateClusterPolicy(ctx kapi.Context, node *authorizationapi.ClusterPolicy) error {
	_, _, err := s.Update(ctx, node)
	return err
}

func (s *storage) WatchClusterPolicies(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetClusterPolicy(ctx kapi.Context, name string) (*authorizationapi.ClusterPolicy, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterPolicy), nil
}

func (s *storage) DeleteClusterPolicy(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}

type simulatedStorage struct {
	clusterRegistry Registry
}

func NewSimulatedRegistry(clusterRegistry Registry) policy.Registry {
	return &simulatedStorage{clusterRegistry}
}

func (s *simulatedStorage) ListPolicies(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.PolicyList, error) {
	ret, err := s.clusterRegistry.ListClusterPolicies(ctx, options)
	return authorizationapi.ToPolicyList(ret), err
}

func (s *simulatedStorage) CreatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error {
	return s.clusterRegistry.CreateClusterPolicy(ctx, authorizationapi.ToClusterPolicy(policy))
}

func (s *simulatedStorage) UpdatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error {
	return s.clusterRegistry.UpdateClusterPolicy(ctx, authorizationapi.ToClusterPolicy(policy))
}

func (s *simulatedStorage) GetPolicy(ctx kapi.Context, name string) (*authorizationapi.Policy, error) {
	ret, err := s.clusterRegistry.GetClusterPolicy(ctx, name)
	return authorizationapi.ToPolicy(ret), err
}

func (s *simulatedStorage) DeletePolicy(ctx kapi.Context, name string) error {
	return s.clusterRegistry.DeleteClusterPolicy(ctx, name)
}
