package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

type BuildConfigRegistry struct {
	Err             error
	BuildConfigs    *api.BuildConfigList
	BuildConfig     *api.BuildConfig
	DeletedConfigId string
}

func (r *BuildConfigRegistry) ListBuildConfigs(labels labels.Selector) (*api.BuildConfigList, error) {
	return r.BuildConfigs, r.Err
}

func (r *BuildConfigRegistry) GetBuildConfig(id string) (*api.BuildConfig, error) {
	return r.BuildConfig, r.Err
}

func (r *BuildConfigRegistry) CreateBuildConfig(config *api.BuildConfig) error {
	return r.Err
}

func (r *BuildConfigRegistry) UpdateBuildConfig(config *api.BuildConfig) error {
	return r.Err
}

func (r *BuildConfigRegistry) DeleteBuildConfig(id string) error {
	r.DeletedConfigId = id
	return r.Err
}
