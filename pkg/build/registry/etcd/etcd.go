package etcd

import (
	"strconv"

	"github.com/golang/glog"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/openshift/origin/pkg/build/api"
)

const (
	// BuildPath is the path to build resources in etcd
	BuildPath string = "/builds"
	// BuildConfigPath is the path to buildConfig resources in etcd
	BuildConfigPath string = "/buildConfigs"
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
			filtered = append(filtered, build)
		}
	}
	allBuilds.Items = filtered
	return &allBuilds, nil
}

// WatchBuilds begins watching for new, changed, or deleted Builds.
func (r *Etcd) WatchBuilds(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := parseWatchResourceVersion(resourceVersion, "build")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeBuildListKey(ctx), version, func(obj runtime.Object) bool {
		build, ok := obj.(*api.Build)
		if !ok {
			glog.Errorf("Unexpected object during build watch: %#v", obj)
			return false
		}
		fields := labels.Set{
			"name":    build.Name,
			"status":  string(build.Status),
			"podName": build.PodName,
		}
		return label.Matches(labels.Set(build.Labels)) && field.Matches(fields)
	})
}

// TODO expose this from kubernetes.  I will do that, but I don't want this merge stuck on kubernetes refactoring
// parseWatchResourceVersion takes a resource version argument and converts it to
// the etcd version we should pass to helper.Watch(). Because resourceVersion is
// an opaque value, the default watch behavior for non-zero watch is to watch
// the next value (if you pass "1", you will see updates from "2" onwards).
func parseWatchResourceVersion(resourceVersion, kind string) (uint64, error) {
	if resourceVersion == "" || resourceVersion == "0" {
		return 0, nil
	}
	version, err := strconv.ParseUint(resourceVersion, 10, 64)
	if err != nil {
		return 0, etcderr.InterpretResourceVersionError(err, kind, resourceVersion)
	}
	return version + 1, nil
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
	return &build, nil
}

// CreateBuild creates a new Build.
func (r *Etcd) CreateBuild(ctx kapi.Context, build *api.Build) error {
	key, err := makeBuildKey(ctx, build.Name)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, build, 0)
	return etcderr.InterpretCreateError(err, "build", build.Name)
}

// UpdateBuild replaces an existing Build.
func (r *Etcd) UpdateBuild(ctx kapi.Context, build *api.Build) error {
	key, err := makeBuildKey(ctx, build.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, build)
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
	err = r.CreateObj(key, config, 0)
	return etcderr.InterpretCreateError(err, "buildConfig", config.Name)
}

// UpdateBuildConfig replaces an existing BuildConfig.
func (r *Etcd) UpdateBuildConfig(ctx kapi.Context, config *api.BuildConfig) error {
	key, err := makeBuildConfigKey(ctx, config.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, config)
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
