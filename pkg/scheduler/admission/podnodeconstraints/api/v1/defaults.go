package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

func SetDefaults_PodNodeConstraintsConfig(obj *PodNodeConstraintsConfig) {
	if obj.NodeSelectorLabelBlacklist == nil {
		obj.NodeSelectorLabelBlacklist = []string{
			kubeletapis.LabelHostname,
		}
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&PodNodeConstraintsConfig{}, func(obj interface{}) { SetDefaults_PodNodeConstraintsConfig(obj.(*PodNodeConstraintsConfig)) })
	return nil
}
