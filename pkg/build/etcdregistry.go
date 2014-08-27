package build

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/openshift/origin/pkg/build/api"
)

// EtcdRegistry implements build.Registry and buildconfig.Registry backed by etcd.
type EtcdRegistry struct {
	tools.EtcdHelper
}

// NewEtcdRegistry creates an etcd registry.
// 'client' is the connection to etcd
func NewEtcdRegistry(client tools.EtcdClient) *EtcdRegistry {
	registry := &EtcdRegistry{
		EtcdHelper: tools.EtcdHelper{
			client,
			runtime.Codec,
			runtime.ResourceVersioner,
		},
	}
	return registry
}

func makeBuildKey(id string) string {
	return "/registry/builds/" + id
}

// ListBuilds obtains a list of Builds.
func (r *EtcdRegistry) ListBuilds(selector labels.Selector) (*api.BuildList, error) {
	allBuilds := api.BuildList{}
	err := r.ExtractList("/registry/builds", &allBuilds.Items, &allBuilds.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.Build{}
	for _, build := range allBuilds.Items {
		if selector.Matches(labels.Set(build.Labels)) {
			filtered = append(filtered, build)
		}
	}
	allBuilds.Items = filtered
	return &allBuilds, nil
}

// GetBuild gets a specific Build specified by its ID.
func (r *EtcdRegistry) GetBuild(id string) (*api.Build, error) {
	var build api.Build
	err := r.ExtractObj(makeBuildKey(id), &build, false)
	if tools.IsEtcdNotFound(err) {
		return nil, errors.NewNotFound("build", id)
	}
	if err != nil {
		return nil, err
	}
	return &build, nil
}

// CreateBuild creates a new Build.
func (r *EtcdRegistry) CreateBuild(build *api.Build) error {
	err := r.CreateObj(makeBuildKey(build.ID), build)
	if tools.IsEtcdNodeExist(err) {
		return errors.NewAlreadyExists("build", build.ID)
	}
	return err
}

// UpdateBuild replaces an existing Build.
func (r *EtcdRegistry) UpdateBuild(build *api.Build) error {
	return r.SetObj(makeBuildKey(build.ID), build)
}

// DeleteBuild deletes a Build specified by its ID.
func (r *EtcdRegistry) DeleteBuild(id string) error {
	key := makeBuildKey(id)
	err := r.Delete(key, true)
	if tools.IsEtcdNotFound(err) {
		return errors.NewNotFound("build", id)
	}
	return err
}

func makeBuildConfigKey(id string) string {
	return "/registry/build-configs/" + id
}

// ListBuildConfigs obtains a list of BuildConfigs.
func (r *EtcdRegistry) ListBuildConfigs(selector labels.Selector) (*api.BuildConfigList, error) {
	allConfigs := api.BuildConfigList{}
	err := r.ExtractList("/registry/build-configs", &allConfigs.Items, &allConfigs.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.BuildConfig{}
	for _, config := range allConfigs.Items {
		if selector.Matches(labels.Set(config.Labels)) {
			filtered = append(filtered, config)
		}
	}
	allConfigs.Items = filtered
	return &allConfigs, nil
}

// GetBuildConfig gets a specific BuildConfig specified by its ID.
func (r *EtcdRegistry) GetBuildConfig(id string) (*api.BuildConfig, error) {
	var config api.BuildConfig
	err := r.ExtractObj(makeBuildConfigKey(id), &config, false)
	if tools.IsEtcdNotFound(err) {
		return nil, errors.NewNotFound("buildConfig", id)
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateBuildConfig creates a new BuildConfig.
func (r *EtcdRegistry) CreateBuildConfig(config *api.BuildConfig) error {
	err := r.CreateObj(makeBuildConfigKey(config.ID), config)
	if tools.IsEtcdNodeExist(err) {
		return errors.NewAlreadyExists("buildConfig", config.ID)
	}
	return err
}

// UpdateBuildConfig replaces an existing BuildConfig.
func (r *EtcdRegistry) UpdateBuildConfig(config *api.BuildConfig) error {
	return r.SetObj(makeBuildConfigKey(config.ID), config)
}

// DeleteBuildConfig deletes a BuildConfig specified by its ID.
func (r *EtcdRegistry) DeleteBuildConfig(id string) error {
	key := makeBuildConfigKey(id)
	err := r.Delete(key, true)
	if tools.IsEtcdNotFound(err) {
		return errors.NewNotFound("buildConfig", id)
	}
	return err
}
