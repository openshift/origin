package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	imagev1 "github.com/openshift/api/image/v1"
)

func InstallExternalLegacyImage(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedImageTypes,
		docker10.AddToSchemeInCoreGroup,
		dockerpre012.AddToSchemeInCoreGroup,
		corev1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedImageTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&imagev1.Image{},
		&imagev1.ImageList{},
		&imagev1.ImageSignature{},
		&imagev1.ImageStream{},
		&imagev1.ImageStreamList{},
		&imagev1.ImageStreamMapping{},
		&imagev1.ImageStreamTag{},
		&imagev1.ImageStreamTagList{},
		&imagev1.ImageStreamImage{},
		&imagev1.ImageStreamImport{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
