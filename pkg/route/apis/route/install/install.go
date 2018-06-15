package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/api/legacy"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
)

func init() {
	legacy.InstallLegacyRoute(legacyscheme.Scheme, legacyscheme.Registry)
	Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  routeapi.GroupName,
			VersionPreferenceOrder:     []string{routeapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: routeapi.AddToScheme,
		},
		announced.VersionToSchemeFunc{
			routeapiv1.SchemeGroupVersion.Version: routeapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
