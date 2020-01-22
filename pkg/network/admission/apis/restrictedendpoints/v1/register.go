package v1

import (
	"github.com/openshift/origin/pkg/network/admission/apis/restrictedendpoints"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (obj *RestrictedEndpointsAdmissionConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }

var GroupVersion = schema.GroupVersion{Group: "network.openshift.io", Version: "v1"}

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		restrictedendpoints.Install,
	)
	Install = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&RestrictedEndpointsAdmissionConfig{},
	)
	return nil
}
