package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/api/legacy"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
)

func init() {
	legacy.InstallLegacyOAuth(legacyscheme.Scheme, legacyscheme.Registry)
	Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  oauthapi.GroupName,
			VersionPreferenceOrder:     []string{oauthapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: oauthapi.AddToScheme,
			RootScopedKinds:            sets.NewString("OAuthAccessToken", "OAuthAuthorizeToken", "OAuthClient", "OAuthClientAuthorization"),
		},
		announced.VersionToSchemeFunc{
			oauthapiv1.SchemeGroupVersion.Version: oauthapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
