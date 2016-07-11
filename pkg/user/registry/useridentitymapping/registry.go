package useridentitymapping

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
)

// Registry is an interface implemented by things that know how to store UserIdentityMapping objects.
type Registry interface {
	// GetUserIdentityMapping returns a UserIdentityMapping for the named identity
	GetUserIdentityMapping(ctx kapi.Context, name string) (*api.UserIdentityMapping, error)
	// CreateUserIdentityMapping associates a user and an identity
	CreateUserIdentityMapping(ctx kapi.Context, mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error)
	// UpdateUserIdentityMapping updates an associated user and identity
	UpdateUserIdentityMapping(ctx kapi.Context, mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error)
	// DeleteUserIdentityMapping removes the user association for the named identity
	DeleteUserIdentityMapping(ctx kapi.Context, name string) error
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type Storage interface {
	rest.Getter
	rest.Deleter

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
	Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error)
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

func (s *storage) GetUserIdentityMapping(ctx kapi.Context, name string) (*api.UserIdentityMapping, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.UserIdentityMapping), nil
}

func (s *storage) CreateUserIdentityMapping(ctx kapi.Context, mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error) {
	obj, err := s.Create(ctx, mapping)
	if err != nil {
		return nil, err
	}
	return obj.(*api.UserIdentityMapping), nil
}

func (s *storage) UpdateUserIdentityMapping(ctx kapi.Context, mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error) {
	obj, _, err := s.Update(ctx, mapping.Name, rest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*api.UserIdentityMapping), nil
}

//
func (s *storage) DeleteUserIdentityMapping(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name)
	return err
}
