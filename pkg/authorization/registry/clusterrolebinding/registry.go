package clusterrolebinding

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store RoleBindings.
type Registry interface {
	// ListRoleBindings obtains list of policyRoleBindings that match a selector.
	ListRoleBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.RoleBindingList, error)
	// GetRoleBinding retrieves a specific policyRoleBinding.
	GetRoleBinding(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.RoleBinding, error)
	// CreateRoleBinding creates a new policyRoleBinding.
	CreateRoleBinding(ctx apirequest.Context, policyRoleBinding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error)
	// UpdateRoleBinding updates a policyRoleBinding.
	UpdateRoleBinding(ctx apirequest.Context, policyRoleBinding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, bool, error)
	// DeleteRoleBinding deletes a policyRoleBinding.
	DeleteRoleBinding(ctx apirequest.Context, id string) error
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Getter
	rest.Lister
	rest.CreaterUpdater
	rest.GracefulDeleter

	// CreateRoleBinding creates a new policyRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	CreateRoleBindingWithEscalation(ctx apirequest.Context, policyRoleBinding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error)
	// UpdateRoleBinding updates a policyRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	UpdateRoleBindingWithEscalation(ctx apirequest.Context, policyRoleBinding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, bool, error)
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

func (s *storage) ListRoleBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.RoleBindingList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingList), nil
}

func (s *storage) CreateRoleBinding(ctx apirequest.Context, binding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	obj, err := s.Create(ctx, binding)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.RoleBinding), err
}

func (s *storage) UpdateRoleBinding(ctx apirequest.Context, binding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, bool, error) {
	obj, created, err := s.Update(ctx, binding.Name, rest.DefaultUpdatedObjectInfo(binding, kapi.Scheme))
	if err != nil {
		return nil, created, err
	}
	return obj.(*authorizationapi.RoleBinding), created, err
}

func (s *storage) GetRoleBinding(ctx apirequest.Context, name string, options *metav1.GetOptions) (*authorizationapi.RoleBinding, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.RoleBinding), nil
}

func (s *storage) DeleteRoleBinding(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}
