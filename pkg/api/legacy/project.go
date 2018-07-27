package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	projectv1 "github.com/openshift/api/project/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	"github.com/openshift/origin/pkg/project/apis/project"
	projectv1helpers "github.com/openshift/origin/pkg/project/apis/project/v1"
)

// InstallLegacyProject this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacyProject(scheme *runtime.Scheme) {
	InstallExternalLegacyProject(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalProjectTypes,

		addLegacyProjectFieldSelectorKeyConversions,
		projectv1helpers.RegisterDefaults,
		projectv1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

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

func addUngroupifiedInternalProjectTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalGroupVersion,
		&project.Project{},
		&project.ProjectList{},
		&project.ProjectRequest{},
	)
	return nil
}

func addLegacyProjectFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(GroupVersion.String(), "Project", legacyProjectFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

// we don't actually do any evaluation, only passing through, so we don't have our own field selector to test.  The upstream
// cannot remove the field selectors or they break compatibility, so we're fine.

func legacyProjectFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "status.phase":
		return label, value, nil
	default:
		return apihelpers.LegacyMetaV1FieldSelectorConversionWithName(label, value)
	}
}
