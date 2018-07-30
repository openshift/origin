package v1

import (
	"github.com/openshift/origin/pkg/quota/admission/apis/runonceduration"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		runonceduration.InstallLegacy,

		addConversionFuncs,
	)
	InstallLegacy = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&RunOnceDurationConfig{},
	)
	return nil
}

func (obj *RunOnceDurationConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
