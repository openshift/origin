package defaults

import (
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	defaultsapi "github.com/openshift/origin/pkg/build/controller/build/defaults/api"
	"github.com/openshift/origin/pkg/build/controller/build/defaults/api/validation"
	"github.com/openshift/origin/pkg/build/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
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
func (b BuildDefaults) ApplyDefaults(pod *v1.Pod) error {
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

func (b BuildDefaults) applyPodDefaults(pod *v1.Pod) {
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

	// Apply default resources
	defaultResources := b.config.Resources
	for i := range pod.Spec.Containers {
		podEnv := &pod.Spec.Containers[i].Env
		util.MergeTrustedEnvWithoutDuplicates(util.CopyApiEnvVarToV1EnvVar(b.config.Env), podEnv, false)

		if pod.Spec.Containers[i].Resources.Limits == nil {
			pod.Spec.Containers[i].Resources.Limits = v1.ResourceList{}
		}
		for name, value := range defaultResources.Limits {
			if _, ok := pod.Spec.Containers[i].Resources.Limits[v1.ResourceName(name)]; !ok {
				glog.V(5).Infof("Setting default resource limit %s for pod %s/%s to %s", name, pod.Namespace, pod.Name, value)
				pod.Spec.Containers[i].Resources.Limits[v1.ResourceName(name)] = value
			}
		}
		if pod.Spec.Containers[i].Resources.Requests == nil {
			pod.Spec.Containers[i].Resources.Requests = v1.ResourceList{}
		}
		for name, value := range defaultResources.Requests {
			if _, ok := pod.Spec.Containers[i].Resources.Requests[v1.ResourceName(name)]; !ok {
				glog.V(5).Infof("Setting default resource request %s for pod %s/%s to %s", name, pod.Namespace, pod.Name, value)
				pod.Spec.Containers[i].Resources.Requests[v1.ResourceName(name)] = value
			}
		}
	}

}

func (b BuildDefaults) applyBuildDefaults(build *buildapi.Build) {
	// Apply default env
	for _, envVar := range b.config.Env {
		glog.V(5).Infof("Adding default environment variable %s=%s to build %s/%s", envVar.Name, envVar.Value, build.Namespace, build.Name)
		addDefaultEnvVar(build, envVar)
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

	//Apply default resources
	defaultResources := b.config.Resources
	if build.Spec.Resources.Limits == nil {
		build.Spec.Resources.Limits = kapi.ResourceList{}
	}
	for name, value := range defaultResources.Limits {
		if _, ok := build.Spec.Resources.Limits[name]; !ok {
			glog.V(5).Infof("Setting default resource limit %s for build %s/%s to %s", name, build.Namespace, build.Name, value)
			build.Spec.Resources.Limits[name] = value
		}
	}
	if build.Spec.Resources.Requests == nil {
		build.Spec.Resources.Requests = kapi.ResourceList{}
	}
	for name, value := range defaultResources.Requests {
		if _, ok := build.Spec.Resources.Requests[name]; !ok {
			glog.V(5).Infof("Setting default resource request %s for build %s/%s to %s", name, build.Namespace, build.Name, value)
			build.Spec.Resources.Requests[name] = value
		}
	}
}

func addDefaultEnvVar(build *buildapi.Build, v kapi.EnvVar) {
	envVars := buildutil.GetBuildEnv(build)

	for i := range envVars {
		if envVars[i].Name == v.Name {
			return
		}
	}
	envVars = append(envVars, v)
	buildutil.SetBuildEnv(build, envVars)
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
