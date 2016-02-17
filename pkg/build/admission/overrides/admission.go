package overrides

import (
	"io"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	overridesapi "github.com/openshift/origin/pkg/build/admission/overrides/api"
)

func init() {
	admission.RegisterPlugin("BuildOverrides", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {
		overridesConfig, err := getConfig(config)
		if err != nil {
			return nil, err
		}

		glog.V(4).Infof("Initializing BuildOverrides plugin with config: %#v", overridesConfig)
		return NewBuildOverrides(overridesConfig), nil
	})
}

func getConfig(in io.Reader) (*overridesapi.BuildOverridesConfig, error) {
	overridesConfig := &overridesapi.BuildOverridesConfig{}
	err := buildadmission.ReadPluginConfig(in, overridesConfig)
	if err != nil {
		return nil, err
	}
	return overridesConfig, nil
}

type buildOverrides struct {
	*admission.Handler
	overridesConfig *overridesapi.BuildOverridesConfig
}

// NewBuildOverrides returns an admission control for builds that overrides
// settings on builds
func NewBuildOverrides(overridesConfig *overridesapi.BuildOverridesConfig) admission.Interface {
	return &buildOverrides{
		Handler:         admission.NewHandler(admission.Create, admission.Update),
		overridesConfig: overridesConfig,
	}
}

// Admit appplies configured overrides to a build in a build pod
func (a *buildOverrides) Admit(attributes admission.Attributes) error {
	if a.overridesConfig == nil {
		return nil
	}
	if !buildadmission.IsBuildPod(attributes) {
		return nil
	}
	return a.applyOverrides(attributes)
}

func (a *buildOverrides) applyOverrides(attributes admission.Attributes) error {
	if !a.overridesConfig.ForcePull {
		return nil
	}
	build, version, err := buildadmission.GetBuild(attributes)
	if err != nil {
		return err
	}
	glog.V(4).Infof("Handling build %s/%s", build.Namespace, build.Name)
	if build.Spec.Strategy.DockerStrategy != nil {
		glog.V(5).Infof("Setting docker strategy ForcePull to true in build %s/%s", build.Namespace, build.Name)
		build.Spec.Strategy.DockerStrategy.ForcePull = true
	}
	if build.Spec.Strategy.SourceStrategy != nil {
		glog.V(5).Infof("Setting source strategy ForcePull to true in build %s/%s", build.Namespace, build.Name)
		build.Spec.Strategy.SourceStrategy.ForcePull = true
	}
	if build.Spec.Strategy.CustomStrategy != nil {
		err := applyForcePullToPod(attributes)
		if err != nil {
			return err
		}
		glog.V(5).Infof("Setting custom strategy ForcePull to true in build %s/%s", build.Namespace, build.Name)
		build.Spec.Strategy.CustomStrategy.ForcePull = true
	}
	return buildadmission.SetBuild(attributes, build, version)
}

func applyForcePullToPod(attributes admission.Attributes) error {
	pod, err := buildadmission.GetPod(attributes)
	if err != nil {
		return err
	}
	for i := range pod.Spec.Containers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on container %s of pod %s/%s", pod.Spec.Containers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.Containers[i].ImagePullPolicy = kapi.PullAlways
	}
	return nil
}
