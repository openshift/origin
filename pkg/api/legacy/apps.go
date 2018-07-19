package legacy

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/origin/pkg/apps/apis/apps"
	appsv1helpers "github.com/openshift/origin/pkg/apps/apis/apps/v1"
)

// InstallLegacyApps this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallLegacyApps(scheme *runtime.Scheme) {
	InstallExternalLegacyApps(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalAppsTypes,
		core.AddToScheme,
		extensions.AddToScheme,

		appsv1helpers.AddConversionFuncs,
		appsv1helpers.RegisterDefaults,
		appsv1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func InstallExternalLegacyApps(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedAppsTypes,
		corev1.AddToScheme,
		rbacv1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedAppsTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&appsv1.DeploymentConfig{},
		&appsv1.DeploymentConfigList{},
		&appsv1.DeploymentConfigRollback{},
		&appsv1.DeploymentRequest{},
		&appsv1.DeploymentLog{},
		&appsv1.DeploymentLogOptions{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}

func addUngroupifiedInternalAppsTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(internalGroupVersion,
		&apps.DeploymentConfig{},
		&apps.DeploymentConfigList{},
		&apps.DeploymentConfigRollback{},
		&apps.DeploymentRequest{},
		&apps.DeploymentLog{},
		&apps.DeploymentLogOptions{},
	)
	return nil
}
