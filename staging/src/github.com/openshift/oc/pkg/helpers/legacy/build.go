package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
)

func InstallExternalLegacyBuild(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedBuildTypes,
		corev1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedBuildTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&buildv1.Build{},
		&buildv1.BuildList{},
		&buildv1.BuildConfig{},
		&buildv1.BuildConfigList{},
		&buildv1.BuildLog{},
		&buildv1.BuildRequest{},
		&buildv1.BuildLogOptions{},
		&buildv1.BinaryBuildRequestOptions{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
