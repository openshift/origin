package defaults

import (
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
	"github.com/openshift/origin/pkg/build/admission/defaults/api/validation"
	buildapi "github.com/openshift/origin/pkg/build/api"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

type BuildDefaults struct {
	config *defaultsapi.BuildDefaultsConfig
}

// NewBuildDefaults creates a new BuildDefaults that will apply the defaults specified in the plugin config
func NewBuildDefaults(pluginConfig map[string]configapi.AdmissionPluginConfig) (BuildDefaults, error) {
	config := &defaultsapi.BuildDefaultsConfig{}
	err := buildadmission.ReadPluginConfig(pluginConfig, defaultsapi.BuildDefaultsPlugin, config)
	if err != nil {
		return BuildDefaults{}, err
	}
	errs := validation.ValidateBuildDefaultsConfig(config)
	if len(errs) > 0 {
		return BuildDefaults{}, errs.ToAggregate()
	}
	glog.V(4).Infof("Initialized build defaults plugin with config: %#v", *config)
	return BuildDefaults{config: config}, nil
}

// ApplyDefaults applies configured build defaults to a build pod
func (b BuildDefaults) ApplyDefaults(pod *kapi.Pod) error {
	if b.config == nil {
		return nil
	}

	build, version, err := buildadmission.GetBuildFromPod(pod)
	if err != nil {
		return nil
	}

	glog.V(4).Infof("Applying defaults to build %s/%s", build.Namespace, build.Name)

	b.applyBuildDefaults(build)

	b.applyPodDefaults(pod)

	err = buildadmission.SetPodLogLevelFromBuild(pod, build)
	if err != nil {
		return err
	}

	return buildadmission.SetBuildInPod(pod, build, version)
}

func (b BuildDefaults) applyPodDefaults(pod *kapi.Pod) {
	if len(b.config.NodeSelector) != 0 && pod.Spec.NodeSelector == nil {
		// only apply nodeselector defaults if the pod has no nodeselector labels
		// already.
		pod.Spec.NodeSelector = map[string]string{}
		for k, v := range b.config.NodeSelector {
			addDefaultNodeSelector(k, v, pod.Spec.NodeSelector)
		}
	}

	if len(b.config.Annotations) != 0 && pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	for k, v := range b.config.Annotations {
		addDefaultAnnotations(k, v, pod.Annotations)
	}
}

func (b BuildDefaults) applyBuildDefaults(build *buildapi.Build) {
	// Apply default env
	buildEnv := getBuildEnv(build)
	for _, envVar := range b.config.Env {
		glog.V(5).Infof("Adding default environment variable %s=%s to build %s/%s", envVar.Name, envVar.Value, build.Namespace, build.Name)
		addDefaultEnvVar(envVar, buildEnv)
	}

	// Apply default labels
	for _, lbl := range b.config.ImageLabels {
		glog.V(5).Infof("Adding default image label %s=%s to build %s/%s", lbl.Name, lbl.Value, build.Namespace, build.Name)
		addDefaultLabel(lbl, &build.Spec.Output.ImageLabels)
	}

	sourceDefaults := b.config.SourceStrategyDefaults
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
	if len(b.config.GitHTTPProxy) != 0 {
		if build.Spec.Source.Git.HTTPProxy == nil {
			t := b.config.GitHTTPProxy
			glog.V(5).Infof("Setting default Git HTTP proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPProxy = &t
		}
	}

	if len(b.config.GitHTTPSProxy) != 0 {
		if build.Spec.Source.Git.HTTPSProxy == nil {
			t := b.config.GitHTTPSProxy
			glog.V(5).Infof("Setting default Git HTTPS proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPSProxy = &t
		}
	}

	if len(b.config.GitNoProxy) != 0 {
		if build.Spec.Source.Git.NoProxy == nil {
			t := b.config.GitNoProxy
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

	for i := range *envVars {
		if (*envVars)[i].Name == v.Name {
			return
		}
	}
	*envVars = append(*envVars, v)
}

func addDefaultLabel(defaultLabel buildapi.ImageLabel, buildLabels *[]buildapi.ImageLabel) {
	found := false
	for _, lbl := range *buildLabels {
		if lbl.Name == defaultLabel.Name {
			found = true
		}
	}
	if !found {
		*buildLabels = append(*buildLabels, defaultLabel)
	}
}

func addDefaultNodeSelector(k, v string, selectors map[string]string) bool {
	if _, ok := selectors[k]; !ok {
		selectors[k] = v
		return true
	}
	return false
}

func addDefaultAnnotations(k, v string, annotations map[string]string) bool {
	if _, ok := annotations[k]; !ok {
		annotations[k] = v
		return true
	}
	return false
}
