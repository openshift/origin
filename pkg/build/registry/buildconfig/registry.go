package buildconfig

import (
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
)

// Registry is an interface for things that know how to store BuildConfigs.
type Registry interface {
	// ListBuildConfigs obtains list of buildConfigs that match a selector.
	ListBuildConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (*buildapi.BuildConfigList, error)
	// GetBuildConfig retrieves a specific buildConfig.
	GetBuildConfig(ctx apirequest.Context, id string, options *metav1.GetOptions) (*buildapi.BuildConfig, error)
	// CreateBuildConfig creates a new buildConfig.
	CreateBuildConfig(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error
	// UpdateBuildConfig updates a buildConfig.
	UpdateBuildConfig(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error
	// DeleteBuildConfig deletes a buildConfig.
	DeleteBuildConfig(ctx apirequest.Context, id string) error
	// WatchBuildConfigs watches buildConfigs.
	WatchBuildConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
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

func (s *storage) ListBuildConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (*buildapi.BuildConfigList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*buildapi.BuildConfigList), nil
}

func (s *storage) WatchBuildConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetBuildConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*buildapi.BuildConfig), nil
}

func (s *storage) CreateBuildConfig(ctx apirequest.Context, build *buildapi.BuildConfig) error {
	_, err := s.Create(ctx, build, false)
	return err
}

func (s *storage) UpdateBuildConfig(ctx apirequest.Context, build *buildapi.BuildConfig) error {
	_, _, err := s.Update(ctx, build.Name, rest.DefaultUpdatedObjectInfo(build, kapi.Scheme))
	return err
}

func (s *storage) DeleteBuildConfig(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}
