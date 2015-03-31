package test

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/build/api"
)

type BuildConfigRegistry struct {
	Err             error
	BuildConfigs    *api.BuildConfigList
	BuildConfig     *api.BuildConfig
	DeletedConfigID string
	sync.Mutex
}

func (r *BuildConfigRegistry) ListBuildConfigs(ctx kapi.Context, labels labels.Selector) (*api.BuildConfigList, error) {
	r.Lock()
	defer r.Unlock()
	return r.BuildConfigs, r.Err
}

func (r *BuildConfigRegistry) GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error) {
	r.Lock()
	defer r.Unlock()
	return r.BuildConfig, r.Err
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

func (r *BuildConfigRegistry) WatchBuildConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
