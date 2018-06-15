package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/api/legacy"
	sdnapi "github.com/openshift/origin/pkg/network/apis/network"
	sdnapiv1 "github.com/openshift/origin/pkg/network/apis/network/v1"
)

func init() {
	legacy.InstallLegacyNetwork(legacyscheme.Scheme, legacyscheme.Registry)
	Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  sdnapi.GroupName,
			VersionPreferenceOrder:     []string{sdnapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: sdnapi.AddToScheme,
			RootScopedKinds:            sets.NewString("ClusterNetwork", "HostSubnet", "NetNamespace"),
		},
		announced.VersionToSchemeFunc{
			sdnapiv1.SchemeGroupVersion.Version: sdnapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
