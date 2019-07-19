package v1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	internal "k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddConversionFuncs(
		func(in *RunOnceDurationConfig, out *internal.RunOnceDurationConfig, s conversion.Scope) error {
			out.ActiveDeadlineSecondsLimit = in.ActiveDeadlineSecondsOverride
			return nil
		},
		func(in *internal.RunOnceDurationConfig, out *RunOnceDurationConfig, s conversion.Scope) error {
			out.ActiveDeadlineSecondsOverride = in.ActiveDeadlineSecondsLimit
			return nil
		},
	)
}
