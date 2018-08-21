package overrides

import (
	"fmt"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/build/controller/common"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

type BuildOverrides struct {
	Config *configapi.BuildOverridesConfig
}

// ApplyOverrides applies configured overrides to a build in a build pod
func (b BuildOverrides) ApplyOverrides(pod *corev1.Pod) error {
	if b.Config == nil {
		return nil
	}

	build, err := common.GetBuildFromPod(pod)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Applying overrides to build %s/%s", build.Namespace, build.Name)

	if b.Config.ForcePull {
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
	for _, lbl := range b.Config.ImageLabels {
		externalLabel := buildv1.ImageLabel{
			Name:  lbl.Name,
			Value: lbl.Value,
		}
		glog.V(5).Infof("Overriding image label %s=%s in build %s/%s", lbl.Name, lbl.Value, build.Namespace, build.Name)
		overrideLabel(externalLabel, &build.Spec.Output.ImageLabels)
	}

	if len(b.Config.NodeSelector) != 0 && pod.Spec.NodeSelector == nil {
		pod.Spec.NodeSelector = map[string]string{}
	}
	for k, v := range b.Config.NodeSelector {
		glog.V(5).Infof("Adding override nodeselector %s=%s to build pod %s/%s", k, v, pod.Namespace, pod.Name)
		pod.Spec.NodeSelector[k] = v
	}

	if len(b.Config.Annotations) != 0 && pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	for k, v := range b.Config.Annotations {
		glog.V(5).Infof("Adding override annotation %s=%s to build pod %s/%s", k, v, pod.Namespace, pod.Name)
		pod.Annotations[k] = v
	}

	// Override Tolerations
	if len(b.Config.Tolerations) != 0 {
		glog.V(5).Infof("Overriding tolerations for pod %s/%s", pod.Namespace, pod.Name)
		pod.Spec.Tolerations = []corev1.Toleration{}
		for _, toleration := range b.Config.Tolerations {
			t := corev1.Toleration{}
			if err := legacyscheme.Scheme.Convert(&toleration, &t, nil); err != nil {
				err := fmt.Errorf("unable to convert core.Toleration to corev1.Toleration: %v", err)
				utilruntime.HandleError(err)
				return err
			}
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, t)
		}
	}

	return common.SetBuildInPod(pod, build)
}

func applyForcePullToPod(pod *corev1.Pod) error {
	for i := range pod.Spec.InitContainers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on init container %s of pod %s/%s", pod.Spec.InitContainers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.InitContainers[i].ImagePullPolicy = corev1.PullAlways
	}
	for i := range pod.Spec.Containers {
		glog.V(5).Infof("Setting ImagePullPolicy to PullAlways on container %s of pod %s/%s", pod.Spec.Containers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways
	}
	return nil
}

func overrideLabel(overridingLabel buildv1.ImageLabel, buildLabels *[]buildv1.ImageLabel) {
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
