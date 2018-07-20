package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/apis/core"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	"github.com/openshift/origin/pkg/build/apis/build"
	buildv1helpers "github.com/openshift/origin/pkg/build/apis/build/v1"
)

// InstallLegacyBuild this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacyBuild(scheme *runtime.Scheme) {
	InstallExternalLegacyBuild(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalBuildTypes,
		core.AddToScheme,

		addLegacyBuildFieldSelectorKeyConversions,
		buildv1helpers.AddConversionFuncs,
		buildv1helpers.RegisterDefaults,
		buildv1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

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

func addUngroupifiedInternalBuildTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalGroupVersion,
		&build.Build{},
		&build.BuildList{},
		&build.BuildConfig{},
		&build.BuildConfigList{},
		&build.BuildLog{},
		&build.BuildRequest{},
		&build.BuildLogOptions{},
		&build.BinaryBuildRequestOptions{},
	)
	return nil
}

func addLegacyBuildFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(GroupVersion.String(), "Build", legacyBuildFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(GroupVersion.String(), "BuildConfig", apihelpers.LegacyMetaV1FieldSelectorConversionWithName); err != nil {
		return err
	}
	return nil
}

// because field selectors can vary in support by version they are exposed under, we have one function for each
// groupVersion we're registering for

func legacyBuildFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "status",
		"podName":
		return label, value, nil
	default:
		return apihelpers.LegacyMetaV1FieldSelectorConversionWithName(label, value)
	}
}
