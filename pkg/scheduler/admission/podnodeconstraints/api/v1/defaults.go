package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

func SetDefaults_PodNodeConstraintsConfig(obj *PodNodeConstraintsConfig) {
	if obj.NodeSelectorLabelBlacklist == nil {
		obj.NodeSelectorLabelBlacklist = []string{
			unversioned.LabelHostname,
		}
	}
}
func addDefaultingFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		SetDefaults_PodNodeConstraintsConfig,
	)
	if err != nil {
		panic(err)
	}
}
