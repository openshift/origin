package overrides

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	overridesapi "github.com/openshift/origin/pkg/build/controller/build/apis/overrides"
	"github.com/openshift/origin/pkg/build/controller/build/apis/overrides/validation"
	"github.com/openshift/origin/pkg/build/controller/build/pluginconfig"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

type BuildOverrides struct {
	config *overridesapi.BuildOverridesConfig
}

// NewBuildOverrides creates a new BuildOverrides that will apply the overrides specified in the plugin config
func NewBuildOverrides(pluginConfig map[string]*configapi.AdmissionPluginConfig) (BuildOverrides, error) {
	config := &overridesapi.BuildOverridesConfig{}
	err := pluginconfig.ReadPluginConfig(pluginConfig, overridesapi.BuildOverridesPlugin, config)
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
func (b BuildOverrides) ApplyOverrides(pod *v1.Pod) error {
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

	// Override Tolerations
	if len(b.config.Tolerations) != 0 {
		glog.V(5).Infof("Overriding tolerations for pod %s/%s", pod.Namespace, pod.Name)
		pod.Spec.Tolerations = []v1.Toleration{}
		for _, toleration := range b.config.Tolerations {
			t := v1.Toleration{}

			if err := kapiv1.Convert_core_Toleration_To_v1_Toleration(&toleration, &t, nil); err != nil {
				err := fmt.Errorf("Unable to convert core.Toleration to v1.Toleration: %v", err)
				utilruntime.HandleError(err)
				return err
			}
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, t)
		}
	}

	return buildadmission.SetBuildInPod(pod, build, version)
}

func applyForcePullToPod(pod *v1.Pod) error {
	for i := range pod.Spec.InitContainers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on init container %s of pod %s/%s", pod.Spec.InitContainers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.InitContainers[i].ImagePullPolicy = v1.PullAlways
	}
	for i := range pod.Spec.Containers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on container %s of pod %s/%s", pod.Spec.Containers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.Containers[i].ImagePullPolicy = v1.PullAlways
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
