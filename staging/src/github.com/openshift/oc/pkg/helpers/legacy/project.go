package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	projectv1 "github.com/openshift/api/project/v1"
)

func InstallExternalLegacyProject(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedProjectTypes,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedProjectTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&projectv1.Project{},
		&projectv1.ProjectList{},
		&projectv1.ProjectRequest{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
