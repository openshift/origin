package test

import (
	"sync"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
)

type BuildConfigRegistry struct {
	Err             error
	BuildConfigs    *api.BuildConfigList
	BuildConfig     *api.BuildConfig
	DeletedConfigID string
	sync.Mutex
}

func (r *BuildConfigRegistry) ListBuildConfigs(ctx kapi.Context, options *kapi.ListOptions) (*api.BuildConfigList, error) {
	r.Lock()
	defer r.Unlock()
	return r.BuildConfigs, r.Err
}

func (r *BuildConfigRegistry) GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error) {
	r.Lock()
	defer r.Unlock()
	if r.BuildConfig != nil && r.BuildConfig.Name == id {
		return r.BuildConfig, r.Err
	}
	return nil, kapierrors.NewNotFound(api.Resource("buildconfig"), id)
}

func (r *BuildConfigRegistry) CreateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	r.Lock()
	defer r.Unlock()
	r.BuildConfig = config
	return r.Err
}

func (r *BuildConfigRegistry) UpdateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	r.Lock()
	defer r.Unlock()
	r.BuildConfig = config
	return r.Err
}

func (r *BuildConfigRegistry) DeleteBuildConfig(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()
	r.DeletedConfigID = id
	r.BuildConfig = nil
	return r.Err
}

func (r *BuildConfigRegistry) WatchBuildConfigs(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return nil, r.Err
}
