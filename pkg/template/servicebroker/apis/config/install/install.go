package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"

	configapi "github.com/openshift/origin/pkg/template/servicebroker/apis/config"
	configapiv1 "github.com/openshift/origin/pkg/template/servicebroker/apis/config/v1"
)

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  configapi.GroupName,
			VersionPreferenceOrder:     []string{configapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: configapi.AddToScheme,
		},
		announced.VersionToSchemeFunc{
			configapiv1.SchemeGroupVersion.Version: configapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
