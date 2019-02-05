package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/route/apiserver/admission/apis/ingressadmission"
)

// SchemeGroupVersion is group version used to register these objects
var DeprecatedSchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	DeprecatedSchemeBuilder = runtime.NewSchemeBuilder(
		deprecatedAddKnownTypes,
		ingressadmission.InstallLegacy,
	)
	DeprecatedInstall = DeprecatedSchemeBuilder.AddToScheme
)

func deprecatedAddKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(DeprecatedSchemeGroupVersion,
		&IngressAdmissionConfig{},
	)
	return nil
}

func (obj *IngressAdmissionConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
