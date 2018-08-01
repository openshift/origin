package v1

import (
	"github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		podnodeconstraints.InstallLegacy,

		addDefaultingFuncs,
	)
	InstallLegacy = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&PodNodeConstraintsConfig{},
	)
	return nil
}

func (obj *PodNodeConstraintsConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
