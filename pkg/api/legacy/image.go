package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/apis/core"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	"github.com/openshift/origin/pkg/image/apis/image"
	imagev1helpers "github.com/openshift/origin/pkg/image/apis/image/v1"
)

// InstallLegacyImage this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacyImage(scheme *runtime.Scheme) {
	InstallExternalLegacyImage(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalImageTypes,
		core.AddToScheme,
		corev1conversions.AddToScheme,

		addLegacyImageFieldSelectorKeyConversions,
		imagev1helpers.RegisterDefaults,
		imagev1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

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

func addUngroupifiedInternalImageTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalGroupVersion,
		&image.Image{},
		&image.ImageList{},
		&image.DockerImage{},
		&image.ImageSignature{},
		&image.ImageStream{},
		&image.ImageStreamList{},
		&image.ImageStreamMapping{},
		&image.ImageStreamTag{},
		&image.ImageStreamTagList{},
		&image.ImageStreamImage{},
		&image.ImageStreamImport{},
	)
	return nil
}

func addLegacyImageFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(GroupVersion.String(), "ImageStream", legacyImageStreamFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

func legacyImageStreamFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "spec.dockerImageRepository",
		"status.dockerImageRepository":
		return label, value, nil
	default:
		return apihelpers.LegacyMetaV1FieldSelectorConversionWithName(label, value)
	}
}
