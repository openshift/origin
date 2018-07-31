package v1

import (
	"github.com/openshift/origin/pkg/ingress/admission/apis/ingressadmission"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		ingressadmission.InstallLegacy,
	)
	InstallLegacy = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&IngressAdmissionConfig{},
	)
	return nil
}

func (obj *IngressAdmissionConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
