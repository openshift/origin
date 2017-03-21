package install

import (
	"k8s.io/kubernetes/pkg/apimachinery/announced"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/api/v1"
)

func init() {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  api.GroupName,
			VersionPreferenceOrder:     []string{v1.SchemeGroupVersion.Version},
			ImportPrefix:               importPrefix,
			AddInternalObjectsToScheme: api.AddToScheme,
			RootScopedKinds:            sets.NewString("OAuthAccessToken", "OAuthAuthorizeToken", "OAuthClient", "OAuthClientAuthorization"),
		},
		announced.VersionToSchemeFunc{
			v1.SchemeGroupVersion.Version: v1.AddToScheme,
		},
	).Announce().RegisterAndEnable(); err != nil {
		panic(err)
	}
}
