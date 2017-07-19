package clusterpolicy

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/policy"
)

// Registry is an interface for things that know how to store ClusterPolicies.
type Registry interface {
	// ListClusterPolicies obtains list of policies that match a selector.
	ListClusterPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error)
	// GetClusterPolicy retrieves a specific policy.
	GetClusterPolicy(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.ClusterPolicy, error)
	// CreateClusterPolicy creates a new policy.
	CreateClusterPolicy(ctx apirequest.Context, policy *authorizationapi.ClusterPolicy) error
	// UpdateClusterPolicy updates a policy.
	UpdateClusterPolicy(ctx apirequest.Context, policy *authorizationapi.ClusterPolicy) error
	// DeleteClusterPolicy deletes a policy.
	DeleteClusterPolicy(ctx apirequest.Context, id string) error
}

type WatchingRegistry interface {
	Registry
	// WatchClusterPolicies watches policies.
	WatchClusterPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
}

type ReadOnlyClusterPolicyInterface interface {
	List(options metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
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

func (s *storage) ListClusterPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyList), nil
}

func (s *storage) CreateClusterPolicy(ctx apirequest.Context, policy *authorizationapi.ClusterPolicy) error {
	_, err := s.Create(ctx, policy, false)
	return err
}

func (s *storage) UpdateClusterPolicy(ctx apirequest.Context, policy *authorizationapi.ClusterPolicy) error {
	_, _, err := s.Update(ctx, policy.Name, rest.DefaultUpdatedObjectInfo(policy, kapi.Scheme))
	return err
}

func (s *storage) WatchClusterPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetClusterPolicy(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.ClusterPolicy, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterPolicy), nil
}

func (s *storage) DeleteClusterPolicy(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

type simulatedStorage struct {
	clusterRegistry Registry
}

func NewSimulatedRegistry(clusterRegistry Registry) policy.Registry {
	return &simulatedStorage{clusterRegistry}
}

func (s *simulatedStorage) ListPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyList, error) {
	ret, err := s.clusterRegistry.ListClusterPolicies(ctx, options)
	return authorizationapi.ToPolicyList(ret), err
}

func (s *simulatedStorage) CreatePolicy(ctx apirequest.Context, policy *authorizationapi.Policy) error {
	return s.clusterRegistry.CreateClusterPolicy(ctx, authorizationapi.ToClusterPolicy(policy))
}

func (s *simulatedStorage) UpdatePolicy(ctx apirequest.Context, policy *authorizationapi.Policy) error {
	return s.clusterRegistry.UpdateClusterPolicy(ctx, authorizationapi.ToClusterPolicy(policy))
}

func (s *simulatedStorage) GetPolicy(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.Policy, error) {
	ret, err := s.clusterRegistry.GetClusterPolicy(ctx, name, options)
	return authorizationapi.ToPolicy(ret), err
}

func (s *simulatedStorage) DeletePolicy(ctx apirequest.Context, name string) error {
	return s.clusterRegistry.DeleteClusterPolicy(ctx, name)
}

type ReadOnlyClusterPolicy struct {
	Registry Registry
}

func (s ReadOnlyClusterPolicy) List(options metav1.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	optint := metainternal.ListOptions{}
	if err := metainternal.Convert_v1_ListOptions_To_internalversion_ListOptions(&options, &optint, nil); err != nil {
		return nil, err
	}
	return s.Registry.ListClusterPolicies(apirequest.WithNamespace(apirequest.NewContext(), ""), &optint)
}

func (s ReadOnlyClusterPolicy) Get(name string, options *metav1.GetOptions) (*authorizationapi.ClusterPolicy, error) {
	return s.Registry.GetClusterPolicy(apirequest.WithNamespace(apirequest.NewContext(), ""), name, options)
}

type ReadOnlyClusterPolicyClientShim struct {
	ReadOnlyClusterPolicy ReadOnlyClusterPolicy
}

func (r *ReadOnlyClusterPolicyClientShim) List(label labels.Selector) ([]*authorizationapi.ClusterPolicy, error) {
	list, err := r.ReadOnlyClusterPolicy.List(metav1.ListOptions{LabelSelector: label.String()})
	if err != nil {
		return nil, err
	}
	var items []*authorizationapi.ClusterPolicy
	for i := range list.Items {
		items = append(items, &list.Items[i])
	}
	return items, nil
}

func (r *ReadOnlyClusterPolicyClientShim) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	return r.ReadOnlyClusterPolicy.Get(name, &metav1.GetOptions{})
}
