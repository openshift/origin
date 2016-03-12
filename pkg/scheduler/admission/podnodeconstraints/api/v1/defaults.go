package v1

import (
	"k8s.io/kubernetes/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *PodNodeConstraintsConfig) {
			if obj.NodeSelectorLabelBlacklist == nil {
				obj.NodeSelectorLabelBlacklist = []string{
					"kubernetes.io/hostname",
				}
			}
		},
	)
	if err != nil {
		panic(err)
	}
}
