package defaults

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	corev1 "k8s.io/api/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

type BuildDefaults struct {
	Config *configapi.BuildDefaultsConfig
}

// ApplyDefaults applies configured build defaults to a build pod
func (b BuildDefaults) ApplyDefaults(pod *corev1.Pod) error {
	build, err := common.GetBuildFromPod(pod)
	if err != nil {
		return nil
	}

	if b.Config == nil {
		// even if there's no config for the defaulter, we need to set up the loglevel.
		return setPodLogLevelFromBuild(pod, build)
	}

	glog.V(4).Infof("Applying defaults to build %s/%s", build.Namespace, build.Name)
	b.applyBuildDefaults(build)

	glog.V(4).Infof("Applying defaults to pod %s/%s", pod.Namespace, pod.Name)
	b.applyPodDefaults(pod, build.Spec.Strategy.CustomStrategy != nil)

	err = setPodLogLevelFromBuild(pod, build)
	if err != nil {
		return err
	}

	return common.SetBuildInPod(pod, build)
}

// setPodLogLevelFromBuild extracts BUILD_LOGLEVEL from the Build environment
// and feeds it as an argument to the Pod's entrypoint. The BUILD_LOGLEVEL
// environment variable may have been set in multiple ways: a default value,
// by a BuildConfig, or by the BuildDefaults admission plugin. In this method
// we finally act on the value by injecting it into the Pod.
func setPodLogLevelFromBuild(pod *corev1.Pod, build *buildv1.Build) error {
	var envs []corev1.EnvVar

	// Check whether the build strategy supports --loglevel parameter.
	switch {
	case build.Spec.Strategy.DockerStrategy != nil:
		envs = build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.SourceStrategy != nil:
		envs = build.Spec.Strategy.SourceStrategy.Env
	default:
		// The build strategy does not support --loglevel
		return nil
	}

	buildLogLevel := "0" // The ultimate default for the build pod's loglevel if no actor sets BUILD_LOGLEVEL in the Build
	for i := range envs {
		env := envs[i]
		if env.Name == "BUILD_LOGLEVEL" {
			buildLogLevel = env.Value
			break
		}
	}
	c := &pod.Spec.Containers[0]
	c.Args = append(c.Args, "--loglevel="+buildLogLevel)
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].Args = append(pod.Spec.InitContainers[i].Args, "--loglevel="+buildLogLevel)
	}
	return nil
}

func (b BuildDefaults) applyPodDefaults(pod *corev1.Pod, isCustomBuild bool) {
	if len(b.Config.NodeSelector) != 0 && pod.Spec.NodeSelector == nil {
		// only apply nodeselector defaults if the pod has no nodeselector labels
		// already.
		pod.Spec.NodeSelector = map[string]string{}
		for k, v := range b.Config.NodeSelector {
			addDefaultNodeSelector(k, v, pod.Spec.NodeSelector)
		}
	}

	if len(b.Config.Annotations) != 0 {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		for k, v := range b.Config.Annotations {
			addDefaultAnnotation(k, v, pod.Annotations)
		}
	}

	// Apply default resources
	defaultResources := b.Config.Resources

	allContainers := make([]*corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	for i := range pod.Spec.Containers {
		allContainers = append(allContainers, &pod.Spec.Containers[i])
	}
	for i := range pod.Spec.InitContainers {
		allContainers = append(allContainers, &pod.Spec.InitContainers[i])
	}

	for _, c := range allContainers {
		// All env vars are allowed to be set in a custom build pod, the user already has
		// total control over the env+logic in a custom build pod anyway.
		externalEnv := make([]corev1.EnvVar, len(b.Config.Env))
		for i, v := range b.Config.Env {
			externalEnv[i] = corev1.EnvVar{}
			if err := legacyscheme.Scheme.Convert(&v, &externalEnv[i], nil); err != nil {
				panic(err)
			}
		}
		if isCustomBuild {
			util.MergeEnvWithoutDuplicates(externalEnv, &c.Env, false, []string{})
		} else {
			util.MergeTrustedEnvWithoutDuplicates(externalEnv, &c.Env, false)
		}

		if c.Resources.Limits == nil {
			c.Resources.Limits = corev1.ResourceList{}
		}
		for name, value := range defaultResources.Limits {
			if _, ok := c.Resources.Limits[corev1.ResourceName(name)]; !ok {
				glog.V(5).Infof("Setting default resource limit %s for pod %s/%s to %v", name, pod.Namespace, pod.Name, value)
				c.Resources.Limits[corev1.ResourceName(name)] = value
			}
		}
		if c.Resources.Requests == nil {
			c.Resources.Requests = corev1.ResourceList{}
		}
		for name, value := range defaultResources.Requests {
			if _, ok := c.Resources.Requests[corev1.ResourceName(name)]; !ok {
				glog.V(5).Infof("Setting default resource request %s for pod %s/%s to %v", name, pod.Namespace, pod.Name, value)
				c.Resources.Requests[corev1.ResourceName(name)] = value
			}
		}
	}
}

func (b BuildDefaults) applyBuildDefaults(build *buildv1.Build) {
	// Apply default env
	for _, envVar := range b.Config.Env {
		glog.V(5).Infof("Adding default environment variable %s=%s to build %s/%s", envVar.Name, envVar.Value, build.Namespace, build.Name)
		externalEnv := corev1.EnvVar{}
		if err := legacyscheme.Scheme.Convert(&envVar, &externalEnv, nil); err != nil {
			panic(err)
		}
		addDefaultEnvVar(build, externalEnv)
	}

	// Apply default labels
	for _, lbl := range b.Config.ImageLabels {
		glog.V(5).Infof("Adding default image label %s=%s to build %s/%s", lbl.Name, lbl.Value, build.Namespace, build.Name)
		label := buildv1.ImageLabel{
			Name:  lbl.Name,
			Value: lbl.Value,
		}
		addDefaultLabel(label, &build.Spec.Output.ImageLabels)
	}

	sourceDefaults := b.Config.SourceStrategyDefaults
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
	if len(b.Config.GitHTTPProxy) != 0 {
		if build.Spec.Source.Git.HTTPProxy == nil {
			t := b.Config.GitHTTPProxy
			glog.V(5).Infof("Setting default Git HTTP proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPProxy = &t
		}
	}

	if len(b.Config.GitHTTPSProxy) != 0 {
		if build.Spec.Source.Git.HTTPSProxy == nil {
			t := b.Config.GitHTTPSProxy
			glog.V(5).Infof("Setting default Git HTTPS proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPSProxy = &t
		}
	}

	if len(b.Config.GitNoProxy) != 0 {
		if build.Spec.Source.Git.NoProxy == nil {
			t := b.Config.GitNoProxy
			glog.V(5).Infof("Setting default Git no proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.NoProxy = &t
		}
	}

	//Apply default resources
	defaultResources := b.Config.Resources
	if build.Spec.Resources.Limits == nil {
		build.Spec.Resources.Limits = corev1.ResourceList{}
	}
	for name, value := range defaultResources.Limits {
		if _, ok := build.Spec.Resources.Limits[corev1.ResourceName(name)]; !ok {
			glog.V(5).Infof("Setting default resource limit %s for build %s/%s to %v", name, build.Namespace, build.Name, value)
			build.Spec.Resources.Limits[corev1.ResourceName(name)] = value
		}
	}
	if build.Spec.Resources.Requests == nil {
		build.Spec.Resources.Requests = corev1.ResourceList{}
	}
	for name, value := range defaultResources.Requests {
		if _, ok := build.Spec.Resources.Requests[corev1.ResourceName(name)]; !ok {
			glog.V(5).Infof("Setting default resource request %s for build %s/%s to %v", name, build.Namespace, build.Name, value)
			build.Spec.Resources.Requests[corev1.ResourceName(name)] = value
		}
	}
}

func addDefaultEnvVar(build *buildv1.Build, v corev1.EnvVar) {
	envVars := buildutil.GetBuildEnv(build)

	for i := range envVars {
		if envVars[i].Name == v.Name {
			return
		}
	}
	envVars = append(envVars, v)
	buildutil.SetBuildEnv(build, envVars)
}

func addDefaultLabel(defaultLabel buildv1.ImageLabel, buildLabels *[]buildv1.ImageLabel) {
	for _, lbl := range *buildLabels {
		if lbl.Name == defaultLabel.Name {
			return
		}
	}
	*buildLabels = append(*buildLabels, defaultLabel)
}

func addDefaultNodeSelector(k, v string, selectors map[string]string) {
	if _, ok := selectors[k]; !ok {
		selectors[k] = v
	}
}

func addDefaultAnnotation(k, v string, annotations map[string]string) {
	if _, ok := annotations[k]; !ok {
		annotations[k] = v
	}
}
