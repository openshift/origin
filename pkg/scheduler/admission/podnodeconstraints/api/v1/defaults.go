package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func SetDefaults_PodNodeConstraintsConfig(obj *PodNodeConstraintsConfig) {
	if obj.NodeSelectorLabelBlacklist == nil {
		obj.NodeSelectorLabelBlacklist = []string{
			metav1.LabelHostname,
		}
	}
}
func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return scheme.AddDefaultingFuncs(
		SetDefaults_PodNodeConstraintsConfig,
	)
}
