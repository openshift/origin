package v1

import (
	"github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		restrictedendpoints.InstallLegacy,
	)
	InstallLegacy = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ExternalIPRangerAdmissionConfig{},
	)
	return nil
}
