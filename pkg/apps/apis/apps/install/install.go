package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	legacy "github.com/openshift/origin/pkg/api/legacy"
	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	deployapiv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func init() {
	legacy.InstallLegacy(deployapi.GroupName, deployapi.AddToSchemeInCoreGroup, deployapiv1.AddToSchemeInCoreGroup, sets.NewString(), kapi.Registry, kapi.Scheme)
	Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  deployapi.GroupName,
			VersionPreferenceOrder:     []string{deployapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: deployapi.AddToScheme,
		},
		announced.VersionToSchemeFunc{
			deployapiv1.SchemeGroupVersion.Version: deployapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
