package useridentitymapping

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// Registry is an interface implemented by things that know how to store UserIdentityMapping objects.
type Registry interface {
	// GetUserIdentityMapping returns a UserIdentityMapping for the named identity
	GetUserIdentityMapping(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.UserIdentityMapping, error)
	// CreateUserIdentityMapping associates a user and an identity
	CreateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error)
	// UpdateUserIdentityMapping updates an associated user and identity
	UpdateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error)
	// DeleteUserIdentityMapping removes the user association for the named identity
	DeleteUserIdentityMapping(ctx apirequest.Context, name string) error
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type Storage interface {
	rest.Getter
	rest.Deleter

	Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error)
	Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error)
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

func (s *storage) GetUserIdentityMapping(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.UserIdentityMapping, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.UserIdentityMapping), nil
}

func (s *storage) CreateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := s.Create(ctx, mapping, false)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.UserIdentityMapping), nil
}

func (s *storage) UpdateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, _, err := s.Update(ctx, mapping.Name, rest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.UserIdentityMapping), nil
}

//
func (s *storage) DeleteUserIdentityMapping(ctx apirequest.Context, name string) error {
	_, err := s.Delete(ctx, name)
	return err
}
