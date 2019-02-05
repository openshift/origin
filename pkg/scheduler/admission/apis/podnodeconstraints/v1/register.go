package v1

import (
	"github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var DeprecatedSchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	DeprecatedSchemeBuilder = runtime.NewSchemeBuilder(
		deprecatedAddKnownTypes,
		podnodeconstraints.InstallLegacy,

		addDefaultingFuncs,
	)
	DeprecatedInstall = DeprecatedSchemeBuilder.AddToScheme
)

func deprecatedAddKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(DeprecatedSchemeGroupVersion,
		&PodNodeConstraintsConfig{},
	)
	return nil
}

func (obj *PodNodeConstraintsConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
