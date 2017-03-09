package role

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store Roles.
type Registry interface {
	// ListRoles obtains list of policyRoles that match a selector.
	ListRoles(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.RoleList, error)
	// GetRole retrieves a specific policyRole.
	GetRole(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.Role, error)
	// CreateRole creates a new policyRole.
	CreateRole(ctx apirequest.Context, policyRole *authorizationapi.Role) (*authorizationapi.Role, error)
	// UpdateRole updates a policyRole.
	UpdateRole(ctx apirequest.Context, policyRole *authorizationapi.Role) (*authorizationapi.Role, bool, error)
	// DeleteRole deletes a policyRole.
	DeleteRole(ctx apirequest.Context, id string) error
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

func (s *storage) ListRoles(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.RoleList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleList), nil
}

func (s *storage) CreateRole(ctx apirequest.Context, role *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := s.Create(ctx, role)
	return obj.(*authorizationapi.Role), err
}

func (s *storage) UpdateRole(ctx apirequest.Context, role *authorizationapi.Role) (*authorizationapi.Role, bool, error) {
	obj, created, err := s.Update(ctx, role.Name, rest.DefaultUpdatedObjectInfo(role, kapi.Scheme))
	return obj.(*authorizationapi.Role), created, err
}

func (s *storage) GetRole(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.Role, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.Role), nil
}

func (s *storage) DeleteRole(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}
