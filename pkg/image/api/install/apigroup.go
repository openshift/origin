package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/docker10"
	"github.com/openshift/origin/pkg/image/api/dockerpre012"
	"github.com/openshift/origin/pkg/image/api/v1"
)

func installApiGroup() {
	Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:              api.GroupName,
			VersionPreferenceOrder: []string{v1.SchemeGroupVersion.Version},
			ImportPrefix:           importPrefix,
			AddInternalObjectsToScheme: func(scheme *runtime.Scheme) error {
				if err := docker10.AddToScheme(scheme); err != nil {
					return err
				}
				if err := dockerpre012.AddToScheme(scheme); err != nil {
					return err
				}
				return api.AddToScheme(scheme)
			},
			RootScopedKinds: sets.NewString("Image", "ImageSignature"),
		},
		announced.VersionToSchemeFunc{v1.SchemeGroupVersion.Version: v1.AddToScheme},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
