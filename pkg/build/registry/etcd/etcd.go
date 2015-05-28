package etcd

import (
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

const (
	// BuildPath is the path to build resources in etcd
	BuildPath string = "/builds"
	// BuildConfigPath is the path to buildConfig resources in etcd
	BuildConfigPath string = "/buildconfigs"
)

// Etcd implements build.Registry and buildconfig.Registry backed by etcd.
type Etcd struct {
	tools.EtcdHelper
}

// New creates an etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

func makeBuildListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, BuildPath)
}

func makeBuildKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, BuildPath, id)
}

// ListBuilds obtains a list of Builds.
func (r *Etcd) ListBuilds(ctx kapi.Context, selector labels.Selector) (*api.BuildList, error) {
	allBuilds := api.BuildList{}
	err := r.ExtractToList(makeBuildListKey(ctx), &allBuilds)
	if err != nil {
		return nil, err
	}
	filtered := []api.Build{}
	for _, build := range allBuilds.Items {
		if selector.Matches(labels.Set(build.Labels)) {
			setDuration(&build)
			filtered = append(filtered, build)
		}
	}
	allBuilds.Items = filtered
	return &allBuilds, nil
}

// WatchBuilds begins watching for new, changed, or deleted Builds.
func (r *Etcd) WatchBuilds(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := tools.ParseWatchResourceVersion(resourceVersion, "build")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeBuildListKey(ctx), version, func(obj runtime.Object) bool {
		build, ok := obj.(*api.Build)
		setDuration(build)
		if !ok {
			glog.Errorf("Unexpected object during build watch: %#v", obj)
			return false
		}
		fields := labels.Set{
			"metadata.name": build.Name,
			"status":        string(build.Status),
			"podName":       buildutil.GetBuildPodName(build),
		}
		return label.Matches(labels.Set(build.Labels)) && field.Matches(fields)
	})
}

// GetBuild gets a specific Build specified by its ID.
func (r *Etcd) GetBuild(ctx kapi.Context, id string) (*api.Build, error) {
	var build api.Build
	key, err := makeBuildKey(ctx, id)
	if err != nil {
		return nil, err
	}
	err = r.ExtractObj(key, &build, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "build", id)
	}
	setDuration(&build)
	return &build, nil
}

// CreateBuild creates a new Build.
func (r *Etcd) CreateBuild(ctx kapi.Context, build *api.Build) error {
	key, err := makeBuildKey(ctx, build.Name)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, build, nil, 0)
	return etcderr.InterpretCreateError(err, "build", build.Name)
}

// UpdateBuild replaces an existing Build.
func (r *Etcd) UpdateBuild(ctx kapi.Context, build *api.Build) error {
	key, err := makeBuildKey(ctx, build.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, build, nil, 0)
	return etcderr.InterpretUpdateError(err, "build", build.Name)
}

// DeleteBuild deletes a Build specified by its ID.
func (r *Etcd) DeleteBuild(ctx kapi.Context, id string) error {
	key, err := makeBuildKey(ctx, id)
	if err != nil {
		return err
	}
	err = r.Delete(key, true)
	return etcderr.InterpretDeleteError(err, "build", id)
}

func makeBuildConfigListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, BuildConfigPath)
}

func makeBuildConfigKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, BuildConfigPath, id)
}

// ListBuildConfigs obtains a list of BuildConfigs.
func (r *Etcd) ListBuildConfigs(ctx kapi.Context, selector labels.Selector) (*api.BuildConfigList, error) {
	allConfigs := api.BuildConfigList{}
	err := r.ExtractToList(makeBuildConfigListKey(ctx), &allConfigs)
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
func (r *Etcd) GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error) {
	var config api.BuildConfig
	key, err := makeBuildConfigKey(ctx, id)
	if err != nil {
		return nil, err
	}
	err = r.ExtractObj(key, &config, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "buildConfig", id)
	}
	return &config, nil
}

// CreateBuildConfig creates a new BuildConfig.
func (r *Etcd) CreateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	key, err := makeBuildConfigKey(ctx, config.Name)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, config, nil, 0)
	return etcderr.InterpretCreateError(err, "buildConfig", config.Name)
}

// UpdateBuildConfig replaces an existing BuildConfig.
func (r *Etcd) UpdateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	key, err := makeBuildConfigKey(ctx, config.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, config, nil, 0)
	return etcderr.InterpretUpdateError(err, "buildConfig", config.Name)
}

// DeleteBuildConfig deletes a BuildConfig specified by its ID.
func (r *Etcd) DeleteBuildConfig(ctx kapi.Context, id string) error {
	key, err := makeBuildConfigKey(ctx, id)
	if err != nil {
		return err
	}
	err = r.Delete(key, true)
	return etcderr.InterpretDeleteError(err, "buildConfig", id)
}

// WatchBuildConfigs begins watching for new, changed, or deleted BuildConfigs.
func (r *Etcd) WatchBuildConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := tools.ParseWatchResourceVersion(resourceVersion, "buildConfig")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeBuildConfigListKey(ctx), version, func(obj runtime.Object) bool {
		buildConfig, ok := obj.(*api.BuildConfig)
		if !ok {
			glog.Errorf("Unexpected object during %s/%s BuildConfig watch: %#v", buildConfig.Namespace, buildConfig.Name, obj)
			return false
		}
		fields := labels.Set{
			"metadata.name": buildConfig.Name,
		}
		return label.Matches(labels.Set(buildConfig.Labels)) && field.Matches(fields)
	})
}

func setDuration(build *api.Build) {
	if build.StartTimestamp == nil {
		build.Duration = time.Duration(0)
	} else {
		completionTimestamp := build.CompletionTimestamp
		if completionTimestamp == nil {
			dummy := util.Now()
			completionTimestamp = &dummy
		}
		build.Duration = completionTimestamp.Rfc3339Copy().Time.Sub(build.StartTimestamp.Rfc3339Copy().Time)
	}
}
