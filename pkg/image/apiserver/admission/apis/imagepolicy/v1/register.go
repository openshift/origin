package v1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
)

// SchemeGroupVersion is group version used to register these objects
var DeprecatedSchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	DeprecatedSchemeBuilder = runtime.NewSchemeBuilder(
		deprecatedAddKnownTypes,
		imagepolicy.InstallLegacy,

		addConversionFuncs,
		addDefaultingFuncs,
	)
	DeprecatedInstall = DeprecatedSchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func deprecatedAddKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(DeprecatedSchemeGroupVersion,
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
