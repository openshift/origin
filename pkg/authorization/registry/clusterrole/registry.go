package clusterrole

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store ClusterRoles.
type Registry interface {
	// ListClusterRoles obtains list of policyClusterRoles that match a selector.
	ListClusterRoles(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterRoleList, error)
	// GetClusterRole retrieves a specific policyClusterRole.
	GetClusterRole(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.ClusterRole, error)
	// CreateClusterRole creates a new policyClusterRole.
	CreateClusterRole(ctx apirequest.Context, policyClusterRole *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error)
	// UpdateClusterRole updates a policyClusterRole.
	UpdateClusterRole(ctx apirequest.Context, policyClusterRole *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error)
	// DeleteClusterRole deletes a policyClusterRole.
	DeleteClusterRole(ctx apirequest.Context, id string) error
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Getter
	rest.Lister
	rest.CreaterUpdater
	rest.GracefulDeleter

	// CreateRoleWithEscalation creates a new policyRole.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	CreateRoleWithEscalation(ctx apirequest.Context, policyRole *authorizationapi.Role) (*authorizationapi.Role, error)
	// UpdateRoleWithEscalation updates a policyRole.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	UpdateRoleWithEscalation(ctx apirequest.Context, policyRole *authorizationapi.Role) (*authorizationapi.Role, bool, error)
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

func (s *storage) ListClusterRoles(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterRoleList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleList), nil
}

func (s *storage) CreateClusterRole(ctx apirequest.Context, node *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error) {
	obj, err := s.Create(ctx, node)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRole), err
}

func (s *storage) UpdateClusterRole(ctx apirequest.Context, node *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error) {
	obj, created, err := s.Update(ctx, node.Name, rest.DefaultUpdatedObjectInfo(node, kapi.Scheme))
	if err != nil {
		return nil, created, err
	}
	return obj.(*authorizationapi.ClusterRole), created, err
}

func (s *storage) GetClusterRole(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.ClusterRole, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterRole), nil
}

func (s *storage) DeleteClusterRole(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}
