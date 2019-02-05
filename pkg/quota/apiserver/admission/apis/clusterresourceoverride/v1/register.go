package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
)

// SchemeGroupVersion is group version used to register these objects
var DeprecatedSchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

var (
	DeprecatedSchemeBuilder = runtime.NewSchemeBuilder(
		deprecatedAddKnownTypes,
		clusterresourceoverride.InstallLegacy,
	)
	DeprecatedInstall = DeprecatedSchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func deprecatedAddKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(DeprecatedSchemeGroupVersion,
		&ClusterResourceOverrideConfig{},
	)
	return nil
}

func (obj *ClusterResourceOverrideConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
