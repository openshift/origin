package rolebinding

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store ClusterRoleBindings.
type Registry interface {
	// ListClusterRoleBindings obtains list of policyClusterRoleBindings that match a selector.
	ListClusterRoleBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterRoleBindingList, error)
	// GetClusterRoleBinding retrieves a specific policyClusterRoleBinding.
	GetClusterRoleBinding(ctx kapi.Context, id string) (*authorizationapi.ClusterRoleBinding, error)
	// CreateClusterRoleBinding creates a new policyClusterRoleBinding.
	CreateClusterRoleBinding(ctx kapi.Context, policyClusterRoleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error)
	// UpdateClusterRoleBinding updates a policyClusterRoleBinding.
	UpdateClusterRoleBinding(ctx kapi.Context, policyClusterRoleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, bool, error)
	// DeleteClusterRoleBinding deletes a policyClusterRoleBinding.
	DeleteClusterRoleBinding(ctx kapi.Context, id string) error
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Getter
	rest.Lister
	rest.CreaterUpdater
	rest.GracefulDeleter

	// CreateClusterRoleBindingWithEscalation creates a new policyClusterRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	CreateClusterRoleBindingWithEscalation(ctx kapi.Context, policyClusterRoleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error)
	// UpdateClusterRoleBindingWithEscalation updates a policyClusterRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	UpdateClusterRoleBindingWithEscalation(ctx kapi.Context, policyClusterRoleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, bool, error)
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) ListClusterRoleBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterRoleBindingList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBindingList), nil
}

func (s *storage) CreateClusterRoleBinding(ctx kapi.Context, binding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := s.Create(ctx, binding)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (s *storage) UpdateClusterRoleBinding(ctx kapi.Context, binding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, bool, error) {
	obj, created, err := s.Update(ctx, binding)
	if obj == nil {
		return nil, created, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), created, err
}

func (s *storage) GetClusterRoleBinding(ctx kapi.Context, name string) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterRoleBinding), nil
}

func (s *storage) DeleteClusterRoleBinding(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}
