package test

import (
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
}

func (r *BuildConfigRegistry) ListBuildConfigs(ctx kapi.Context, labels labels.Selector) (*api.BuildConfigList, error) {
	return r.BuildConfigs, r.Err
}

func (r *BuildConfigRegistry) GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error) {
	return r.BuildConfig, r.Err
}

func (r *BuildConfigRegistry) CreateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	return r.Err
}

func (r *BuildConfigRegistry) UpdateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	return r.Err
}

func (r *BuildConfigRegistry) DeleteBuildConfig(ctx kapi.Context, id string) error {
	r.DeletedConfigID = id
	return r.Err
}

func (r *BuildConfigRegistry) WatchBuildConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
