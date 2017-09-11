package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/legacy"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
)

func init() {
	Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)
	legacy.InstallLegacy(securityapi.GroupName, securityapi.AddToSchemeInCoreGroup, securityapiv1.AddToSchemeInCoreGroup,
		sets.NewString("SecurityContextConstraints"),
		kapi.Registry, kapi.Scheme,
	)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  securityapi.GroupName,
			VersionPreferenceOrder:     []string{securityapiv1.SchemeGroupVersion.Version},
			RootScopedKinds:            sets.NewString("SecurityContextConstraints"),
			AddInternalObjectsToScheme: securityapi.AddToScheme,
		},
		announced.VersionToSchemeFunc{
			securityapiv1.SchemeGroupVersion.Version: securityapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
