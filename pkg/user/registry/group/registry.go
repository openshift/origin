package group

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// Registry is an interface implemented by things that know how to store Group objects.
type Registry interface {
	// ListGroups obtains a list of groups having labels which match selector.
	ListGroups(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.GroupList, error)
	// GetGroup returns a specific group
	GetGroup(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.Group, error)
	// CreateGroup creates a group
	CreateGroup(ctx apirequest.Context, group *userapi.Group) (*userapi.Group, error)
	// UpdateGroup updates an existing group
	UpdateGroup(ctx apirequest.Context, group *userapi.Group) (*userapi.Group, error)
	// DeleteGroup deletes a name.
	DeleteGroup(ctx apirequest.Context, name string) error
	// WatchGroups watches groups.
	WatchGroups(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
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
func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) ListGroups(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.GroupList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.GroupList), nil
}

func (s *storage) GetGroup(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.Group, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.Group), nil
}

func (s *storage) CreateGroup(ctx apirequest.Context, group *userapi.Group) (*userapi.Group, error) {
	obj, err := s.Create(ctx, group, false)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.Group), nil
}

func (s *storage) UpdateGroup(ctx apirequest.Context, group *userapi.Group) (*userapi.Group, error) {
	obj, _, err := s.Update(ctx, group.Name, rest.DefaultUpdatedObjectInfo(group, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.Group), nil
}

func (s *storage) DeleteGroup(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

func (s *storage) WatchGroups(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}
