package overrides

import (
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	overridesapi "github.com/openshift/origin/pkg/build/admission/overrides/api"
	"github.com/openshift/origin/pkg/build/admission/overrides/api/validation"
	buildapi "github.com/openshift/origin/pkg/build/api"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

type BuildOverrides struct {
	config *overridesapi.BuildOverridesConfig
}

// NewBuildOverrides creates a new BuildOverrides that will apply the overrides specified in the plugin config
func NewBuildOverrides(pluginConfig map[string]configapi.AdmissionPluginConfig) (BuildOverrides, error) {
	config := &overridesapi.BuildOverridesConfig{}
	err := buildadmission.ReadPluginConfig(pluginConfig, overridesapi.BuildOverridesPlugin, config)
	if err != nil {
		return BuildOverrides{}, err
	}
	errs := validation.ValidateBuildOverridesConfig(config)
	if len(errs) > 0 {
		return BuildOverrides{}, errs.ToAggregate()
	}
	glog.V(4).Infof("Initialized build overrides plugin with config: %#v", *config)
	return BuildOverrides{config: config}, nil
}

// ApplyOverrides applies configured overrides to a build in a build pod
func (b BuildOverrides) ApplyOverrides(pod *kapi.Pod) error {
	if b.config == nil {
		return nil
	}

	build, version, err := buildadmission.GetBuildFromPod(pod)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Applying overrides to build %s/%s", build.Namespace, build.Name)

	if b.config.ForcePull {
		if build.Spec.Strategy.DockerStrategy != nil {
			glog.V(5).Infof("Setting docker strategy ForcePull to true in build %s/%s", build.Namespace, build.Name)
			build.Spec.Strategy.DockerStrategy.ForcePull = true
		}
		if build.Spec.Strategy.SourceStrategy != nil {
			glog.V(5).Infof("Setting source strategy ForcePull to true in build %s/%s", build.Namespace, build.Name)
			build.Spec.Strategy.SourceStrategy.ForcePull = true
		}
		if build.Spec.Strategy.CustomStrategy != nil {
			err := applyForcePullToPod(pod)
			if err != nil {
				return err
			}
			glog.V(5).Infof("Setting custom strategy ForcePull to true in build %s/%s", build.Namespace, build.Name)
			build.Spec.Strategy.CustomStrategy.ForcePull = true
		}
	}

	// Apply label overrides
	for _, lbl := range b.config.ImageLabels {
		glog.V(5).Infof("Overriding image label %s=%s in build %s/%s", lbl.Name, lbl.Value, build.Namespace, build.Name)
		overrideLabel(lbl, &build.Spec.Output.ImageLabels)
	}

	if len(b.config.NodeSelector) != 0 && pod.Spec.NodeSelector == nil {
		pod.Spec.NodeSelector = map[string]string{}
	}
	for k, v := range b.config.NodeSelector {
		glog.V(5).Infof("Adding override nodeselector %s=%s to build pod %s/%s", k, v, pod.Namespace, pod.Name)
		pod.Spec.NodeSelector[k] = v
	}

	if len(b.config.Annotations) != 0 && pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	for k, v := range b.config.Annotations {
		glog.V(5).Infof("Adding override annotation %s=%s to build pod %s/%s", k, v, pod.Namespace, pod.Name)
		pod.Annotations[k] = v
	}

	return buildadmission.SetBuildInPod(pod, build, version)
}

func applyForcePullToPod(pod *kapi.Pod) error {
	for i := range pod.Spec.InitContainers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on init container %s of pod %s/%s", pod.Spec.InitContainers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.InitContainers[i].ImagePullPolicy = kapi.PullAlways
	}
	for i := range pod.Spec.Containers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on container %s of pod %s/%s", pod.Spec.Containers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.Containers[i].ImagePullPolicy = kapi.PullAlways
	}
	return nil
}

func overrideLabel(overridingLabel buildapi.ImageLabel, buildLabels *[]buildapi.ImageLabel) {
	found := false
	for i, lbl := range *buildLabels {
		if lbl.Name == overridingLabel.Name {
			glog.V(5).Infof("Replacing label %s (original value %q) with new value %q", lbl.Name, lbl.Value, overridingLabel.Value)
			(*buildLabels)[i] = overridingLabel
			found = true
		}
	}
	if !found {
		*buildLabels = append(*buildLabels, overridingLabel)
	}
}
