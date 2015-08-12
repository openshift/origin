package group

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/user/api"
)

// Registry is an interface implemented by things that know how to store Group objects.
type Registry interface {
	// ListGroups obtains a list of groups having labels which match selector.
	ListGroups(ctx kapi.Context, selector labels.Selector, field fields.Selector) (*api.GroupList, error)
	// GetGroup returns a specific group
	GetGroup(ctx kapi.Context, name string) (*api.Group, error)
	// CreateGroup creates a group
	CreateGroup(ctx kapi.Context, group *api.Group) (*api.Group, error)
	// UpdateGroup updates an existing group
	UpdateGroup(ctx kapi.Context, group *api.Group) (*api.Group, error)
	// DeleteGroup deletes a name.
	DeleteGroup(ctx kapi.Context, name string) error
	// WatchGroups watches groups.
	WatchGroups(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
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

func (s *storage) ListGroups(ctx kapi.Context, label labels.Selector, field fields.Selector) (*api.GroupList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return obj.(*api.GroupList), nil
}

func (s *storage) GetGroup(ctx kapi.Context, name string) (*api.Group, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Group), nil
}

func (s *storage) CreateGroup(ctx kapi.Context, group *api.Group) (*api.Group, error) {
	obj, err := s.Create(ctx, group)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Group), nil
}

func (s *storage) UpdateGroup(ctx kapi.Context, group *api.Group) (*api.Group, error) {
	obj, _, err := s.Update(ctx, group)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Group), nil
}

func (s *storage) DeleteGroup(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}

func (s *storage) WatchGroups(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}
