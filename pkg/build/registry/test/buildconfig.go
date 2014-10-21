package test

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

type BuildConfigRegistry struct {
	Err             error
	BuildConfigs    *api.BuildConfigList
	BuildConfig     *api.BuildConfig
	DeletedConfigId string
}

func (r *BuildConfigRegistry) ListBuildConfigs(ctx kubeapi.Context, labels labels.Selector) (*api.BuildConfigList, error) {
	return r.BuildConfigs, r.Err
}

func (r *BuildConfigRegistry) GetBuildConfig(ctx kubeapi.Context, id string) (*api.BuildConfig, error) {
	return r.BuildConfig, r.Err
}

func (r *BuildConfigRegistry) CreateBuildConfig(ctx kubeapi.Context, config *api.BuildConfig) error {
	return r.Err
}

func (r *BuildConfigRegistry) UpdateBuildConfig(ctx kubeapi.Context, config *api.BuildConfig) error {
	return r.Err
}

func (r *BuildConfigRegistry) DeleteBuildConfig(ctx kubeapi.Context, id string) error {
	r.DeletedConfigId = id
	return r.Err
}
