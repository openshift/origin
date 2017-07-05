package user

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// Registry is an interface implemented by things that know how to store User objects.
type Registry interface {
	// ListUsers obtains a list of users having labels which match selector.
	ListUsers(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.UserList, error)
	// GetUser returns a specific user
	GetUser(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.User, error)
	// CreateUser creates a user
	CreateUser(ctx apirequest.Context, user *userapi.User) (*userapi.User, error)
	// UpdateUser updates an existing user
	UpdateUser(ctx apirequest.Context, user *userapi.User) (*userapi.User, error)
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type Storage interface {
	rest.Lister
	rest.Getter

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

func (s *storage) ListUsers(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.UserList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.UserList), nil
}

func (s *storage) GetUser(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.User, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.User), nil
}

func (s *storage) CreateUser(ctx apirequest.Context, user *userapi.User) (*userapi.User, error) {
	obj, err := s.Create(ctx, user, false)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.User), nil
}

func (s *storage) UpdateUser(ctx apirequest.Context, user *userapi.User) (*userapi.User, error) {
	obj, _, err := s.Update(ctx, user.Name, rest.DefaultUpdatedObjectInfo(user, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.User), nil
}
