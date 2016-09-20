package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs, addDefaultingFuncs)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ImagePolicyConfig{},
	)
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddConversionFuncs(
		// TODO: remove when MatchSignatures is implemented
		func(in *ImageCondition, out *api.ImageCondition, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		// TODO: remove when ConsumptionRules and PlacementRules are implemented
		func(in *ImagePolicyConfig, out *api.ImagePolicyConfig, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
	)
}

func (obj *ImagePolicyConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
