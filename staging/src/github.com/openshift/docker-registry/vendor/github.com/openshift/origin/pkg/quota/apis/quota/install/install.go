package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/legacy"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	quotaapiv1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
)

func init() {
	legacy.InstallLegacy(quotaapi.GroupName, quotaapi.AddToSchemeInCoreGroup, quotaapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterResourceQuota"),
		kapi.Registry, kapi.Scheme,
	)
	Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  quotaapi.GroupName,
			VersionPreferenceOrder:     []string{quotaapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: quotaapi.AddToScheme,
			RootScopedKinds:            sets.NewString("ClusterResourceQuota"),
		},
		announced.VersionToSchemeFunc{
			quotaapiv1.SchemeGroupVersion.Version: quotaapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
