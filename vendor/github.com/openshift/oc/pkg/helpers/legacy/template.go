package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	templatev1 "github.com/openshift/api/template/v1"
)

func InstallExternalLegacyTemplate(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedTemplateTypes,
		corev1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedTemplateTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&templatev1.Template{},
		&templatev1.TemplateList{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	scheme.AddKnownTypeWithName(GroupVersion.WithKind("TemplateConfig"), &templatev1.Template{})
	scheme.AddKnownTypeWithName(GroupVersion.WithKind("ProcessedTemplate"), &templatev1.Template{})
	return nil
}
