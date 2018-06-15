package v1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/image/admission/apis/imagepolicy"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

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
		func(in *ImageCondition, out *imagepolicy.ImageCondition, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		// TODO: remove when ConsumptionRules and PlacementRules are implemented
		func(in *ImagePolicyConfig, out *imagepolicy.ImagePolicyConfig, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
	)
}

func (obj *ImagePolicyConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
