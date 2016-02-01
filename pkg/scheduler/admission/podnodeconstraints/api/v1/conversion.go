package v1

import (
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	// oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api"
)

func convert_v1_PodNodeConstraintsConfig_to_api_PodNodeConstraintsConfig(in *PodNodeConstraintsConfig, out *api.PodNodeConstraintsConfig, s conversion.Scope) error {
	out.NodeSelectorLabelBlacklist = sets.NewString(in.NodeSelectorLabelBlacklist...)
	return nil
}

func convert_api_PodNodeConstraintsConfig_to_v1_PodNodeConstraintsConfig(in *api.PodNodeConstraintsConfig, out *PodNodeConstraintsConfig, s conversion.Scope) error {
	out.NodeSelectorLabelBlacklist = in.NodeSelectorLabelBlacklist.List()
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		convert_v1_PodNodeConstraintsConfig_to_api_PodNodeConstraintsConfig,
		convert_api_PodNodeConstraintsConfig_to_v1_PodNodeConstraintsConfig,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
