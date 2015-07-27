package role

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store ClusterRoles.
type Registry interface {
	// ListClusterRoles obtains list of policyClusterRoles that match a selector.
	ListClusterRoles(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterRoleList, error)
	// GetClusterRole retrieves a specific policyClusterRole.
	GetClusterRole(ctx kapi.Context, id string) (*authorizationapi.ClusterRole, error)
	// CreateClusterRole creates a new policyClusterRole.
	CreateClusterRole(ctx kapi.Context, policyClusterRole *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error)
	// UpdateClusterRole updates a policyClusterRole.
	UpdateClusterRole(ctx kapi.Context, policyClusterRole *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error)
	// DeleteClusterRole deletes a policyClusterRole.
	DeleteClusterRole(ctx kapi.Context, id string) error
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Getter
	rest.Lister
	rest.CreaterUpdater
	rest.GracefulDeleter
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

func (s *storage) ListClusterRoles(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterRoleList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleList), nil
}

func (s *storage) CreateClusterRole(ctx kapi.Context, node *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error) {
	obj, err := s.Create(ctx, node)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRole), err
}

func (s *storage) UpdateClusterRole(ctx kapi.Context, node *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error) {
	obj, created, err := s.Update(ctx, node)
	if err != nil {
		return nil, created, err
	}
	return obj.(*authorizationapi.ClusterRole), created, err
}

func (s *storage) GetClusterRole(ctx kapi.Context, name string) (*authorizationapi.ClusterRole, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterRole), nil
}

func (s *storage) DeleteClusterRole(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}
