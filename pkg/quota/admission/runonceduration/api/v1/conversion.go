package v1

import (
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	internal "github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		func(in *RunOnceDurationConfig, out *internal.RunOnceDurationConfig, s conversion.Scope) error {
			out.ActiveDeadlineSecondsLimit = in.ActiveDeadlineSecondsOverride
			return nil
		},
		func(in *internal.RunOnceDurationConfig, out *RunOnceDurationConfig, s conversion.Scope) error {
			out.ActiveDeadlineSecondsOverride = in.ActiveDeadlineSecondsLimit
			return nil
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
