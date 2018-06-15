package test

import (
	"context"
	"sync"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type BuildConfigRegistry struct {
	Err             error
	BuildConfigs    *buildapi.BuildConfigList
	BuildConfig     *buildapi.BuildConfig
	DeletedConfigID string
	sync.Mutex
}

func (r *BuildConfigRegistry) ListBuildConfigs(ctx context.Context, options *metainternal.ListOptions) (*buildapi.BuildConfigList, error) {
	r.Lock()
	defer r.Unlock()
	return r.BuildConfigs, r.Err
}

func (r *BuildConfigRegistry) GetBuildConfig(ctx context.Context, id string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
	r.Lock()
	defer r.Unlock()
	if r.BuildConfig != nil && r.BuildConfig.Name == id {
		return r.BuildConfig, r.Err
	}
	return nil, kapierrors.NewNotFound(buildapi.Resource("buildconfig"), id)
}

func (r *BuildConfigRegistry) CreateBuildConfig(ctx context.Context, config *buildapi.BuildConfig) error {
	r.Lock()
	defer r.Unlock()
	r.BuildConfig = config
	return r.Err
}

func (r *BuildConfigRegistry) UpdateBuildConfig(ctx context.Context, config *buildapi.BuildConfig) error {
	r.Lock()
	defer r.Unlock()
	r.BuildConfig = config
	return r.Err
}

func (r *BuildConfigRegistry) DeleteBuildConfig(ctx context.Context, id string) error {
	r.Lock()
	defer r.Unlock()
	r.DeletedConfigID = id
	r.BuildConfig = nil
	return r.Err
}

func (r *BuildConfigRegistry) WatchBuildConfigs(ctx context.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return nil, r.Err
}
