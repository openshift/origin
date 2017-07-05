package build

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/build/apis/build"
)

// Registry is an interface for things that know how to store Builds.
type Registry interface {
	// ListBuilds obtains list of builds that match a selector.
	ListBuilds(ctx apirequest.Context, options *metainternal.ListOptions) (*api.BuildList, error)
	// GetBuild retrieves a specific build.
	GetBuild(ctx apirequest.Context, id string, options *metav1.GetOptions) (*api.Build, error)
	// CreateBuild creates a new build.
	CreateBuild(ctx apirequest.Context, build *api.Build) error
	// UpdateBuild updates a build.
	UpdateBuild(ctx apirequest.Context, build *api.Build) error
	// DeleteBuild deletes a build.
	DeleteBuild(ctx apirequest.Context, id string) error
	// WatchBuilds watches builds.
	WatchBuilds(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
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

func (s *storage) ListBuilds(ctx apirequest.Context, options *metainternal.ListOptions) (*api.BuildList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.BuildList), nil
}

func (s *storage) WatchBuilds(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetBuild(ctx apirequest.Context, name string, options *metav1.GetOptions) (*api.Build, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Build), nil
}

func (s *storage) CreateBuild(ctx apirequest.Context, build *api.Build) error {
	_, err := s.Create(ctx, build, false)
	return err
}

func (s *storage) UpdateBuild(ctx apirequest.Context, build *api.Build) error {
	_, _, err := s.Update(ctx, build.Name, rest.DefaultUpdatedObjectInfo(build, kapi.Scheme))
	return err
}

func (s *storage) DeleteBuild(ctx apirequest.Context, buildID string) error {
	_, _, err := s.Delete(ctx, buildID, nil)
	return err
}
