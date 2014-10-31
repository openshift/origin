package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

type BuildConfigRegistry struct {
	Err             error
	BuildConfigs    *api.BuildConfigList
	BuildConfig     *api.BuildConfig
	DeletedConfigId string
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
	r.DeletedConfigId = id
	return r.Err
}
