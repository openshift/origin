package legacy

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
)

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
