package defaults

import (
	"io"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
	"github.com/openshift/origin/pkg/build/admission/defaults/api/validation"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

func init() {
	admission.RegisterPlugin("BuildDefaults", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {

		defaultsConfig, err := getConfig(config)
		if err != nil {
			return nil, err
		}

		glog.V(5).Infof("Initializing BuildDefaults plugin with config: %#v", defaultsConfig)
		return NewBuildDefaults(defaultsConfig), nil
	})
}

func getConfig(in io.Reader) (*defaultsapi.BuildDefaultsConfig, error) {
	defaultsConfig := &defaultsapi.BuildDefaultsConfig{}
	err := buildadmission.ReadPluginConfig(in, defaultsConfig)
	if err != nil {
		return nil, err
	}
	errs := validation.ValidateBuildDefaultsConfig(defaultsConfig)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return defaultsConfig, nil
}

type buildDefaults struct {
	*admission.Handler
	defaultsConfig *defaultsapi.BuildDefaultsConfig
}

// NewBuildDefaults returns an admission control for builds that sets build defaults
// based on the plugin configuration
func NewBuildDefaults(defaultsConfig *defaultsapi.BuildDefaultsConfig) admission.Interface {
	return &buildDefaults{
		Handler:        admission.NewHandler(admission.Create),
		defaultsConfig: defaultsConfig,
	}
}

// Admit applies configured build defaults to a pod that is identified
// as a build pod.
func (a *buildDefaults) Admit(attributes admission.Attributes) error {
	if a.defaultsConfig == nil {
		return nil
	}
	if !buildadmission.IsBuildPod(attributes) {
		return nil
	}
	build, version, err := buildadmission.GetBuild(attributes)
	if err != nil {
		return nil
	}

	glog.V(4).Infof("Handling build %s/%s", build.Namespace, build.Name)

	a.applyBuildDefaults(build)

	err = buildadmission.SetBuildLogLevel(attributes, build)
	if err != nil {
		return err
	}

	return buildadmission.SetBuild(attributes, build, version)
}

func (a *buildDefaults) applyBuildDefaults(build *buildapi.Build) {
	// Apply default env
	buildEnv := getBuildEnv(build)
	for _, envVar := range a.defaultsConfig.Env {
		glog.V(5).Infof("Adding default environment variable %s=%s to build %s/%s", envVar.Name, envVar.Value, build.Namespace, build.Name)
		addDefaultEnvVar(envVar, buildEnv)
	}

	sourceDefaults := a.defaultsConfig.SourceStrategyDefaults
	sourceStrategy := build.Spec.Strategy.SourceStrategy
	if sourceDefaults != nil && sourceDefaults.Incremental != nil && *sourceDefaults.Incremental &&
		sourceStrategy != nil && sourceStrategy.Incremental == nil {
		glog.V(5).Infof("Setting source strategy Incremental to true in build %s/%s", build.Namespace, build.Name)
		t := true
		build.Spec.Strategy.SourceStrategy.Incremental = &t
	}

	// Apply git proxy defaults
	if build.Spec.Source.Git == nil {
		return
	}
	if len(a.defaultsConfig.GitHTTPProxy) != 0 {
		if build.Spec.Source.Git.HTTPProxy == nil {
			t := a.defaultsConfig.GitHTTPProxy
			glog.V(5).Infof("Setting default Git HTTP proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPProxy = &t
		}
	}

	if len(a.defaultsConfig.GitHTTPSProxy) != 0 {
		if build.Spec.Source.Git.HTTPSProxy == nil {
			t := a.defaultsConfig.GitHTTPSProxy
			glog.V(5).Infof("Setting default Git HTTPS proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPSProxy = &t
		}
	}

	if len(a.defaultsConfig.GitNoProxy) != 0 {
		if build.Spec.Source.Git.NoProxy == nil {
			t := a.defaultsConfig.GitNoProxy
			glog.V(5).Infof("Setting default Git no proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.NoProxy = &t
		}
	}
}

func getBuildEnv(build *buildapi.Build) *[]kapi.EnvVar {
	switch {
	case build.Spec.Strategy.DockerStrategy != nil:
		return &build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.SourceStrategy != nil:
		return &build.Spec.Strategy.SourceStrategy.Env
	case build.Spec.Strategy.CustomStrategy != nil:
		return &build.Spec.Strategy.CustomStrategy.Env
	}
	return nil
}

func addDefaultEnvVar(v kapi.EnvVar, envVars *[]kapi.EnvVar) {
	if envVars == nil {
		return
	}

	found := false
	for i := range *envVars {
		if (*envVars)[i].Name == v.Name {
			found = true
		}
	}
	if !found {
		*envVars = append(*envVars, v)
	}
}
