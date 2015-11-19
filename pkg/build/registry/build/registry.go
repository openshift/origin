package build

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	api "github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store Builds.
type Registry interface {
	// ListBuilds obtains list of builds that match a selector.
	ListBuilds(ctx kapi.Context, labels labels.Selector, fields fields.Selector) (*api.BuildList, error)
	// GetBuild retrieves a specific build.
	GetBuild(ctx kapi.Context, id string) (*api.Build, error)
	// CreateBuild creates a new build.
	CreateBuild(ctx kapi.Context, build *api.Build) error
	// UpdateBuild updates a build.
	UpdateBuild(ctx kapi.Context, build *api.Build) error
	// DeleteBuild deletes a build.
	DeleteBuild(ctx kapi.Context, id string) error
	// WatchBuilds watches builds.
	WatchBuilds(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListBuilds(ctx kapi.Context, label labels.Selector, field fields.Selector) (*api.BuildList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return obj.(*api.BuildList), nil
}

func (s *storage) WatchBuilds(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}

func (s *storage) GetBuild(ctx kapi.Context, name string) (*api.Build, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Build), nil
}

func (s *storage) CreateBuild(ctx kapi.Context, build *api.Build) error {
	_, err := s.Create(ctx, build)
	return err
}

func (s *storage) UpdateBuild(ctx kapi.Context, build *api.Build) error {
	_, _, err := s.Update(ctx, build)
	return err
}

func (s *storage) DeleteBuild(ctx kapi.Context, buildID string) error {
	_, err := s.Delete(ctx, buildID, nil)
	return err
}
