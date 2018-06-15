package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/api/legacy"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
)

func init() {
	legacy.InstallLegacyBuild(legacyscheme.Scheme, legacyscheme.Registry)
	Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  buildapi.GroupName,
			VersionPreferenceOrder:     []string{buildapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: buildapi.AddToScheme,
		},
		announced.VersionToSchemeFunc{
			buildapiv1.SchemeGroupVersion.Version: buildapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
