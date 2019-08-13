package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	userv1 "github.com/openshift/api/user/v1"
)

func InstallExternalLegacyUser(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedUserTypes,
		corev1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedUserTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&userv1.User{},
		&userv1.UserList{},
		&userv1.Identity{},
		&userv1.IdentityList{},
		&userv1.UserIdentityMapping{},
		&userv1.Group{},
		&userv1.GroupList{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
