package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Adds the list of known types to api.Scheme.
func AddToScheme(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ImagePolicyConfig{},
	)
	scheme.AddDefaultingFuncs(
		func(c *ImagePolicyConfig) {
			for i := range c.ExecutionRules {
				if len(c.ExecutionRules[i].OnResources) == 0 {
					c.ExecutionRules[i].OnResources = []GroupResource{{Resource: "pods", Group: kapi.GroupName}}
				}
			}
		},
	)
	scheme.AddConversionFuncs(
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
