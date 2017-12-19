package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	"github.com/openshift/origin/pkg/api/legacy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

func init() {
	legacy.InstallLegacyImage(legacyscheme.Scheme, legacyscheme.Registry)
	Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:              imageapi.GroupName,
			VersionPreferenceOrder: []string{imageapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: func(scheme *runtime.Scheme) error {
				if err := docker10.AddToScheme(scheme); err != nil {
					return err
				}
				if err := dockerpre012.AddToScheme(scheme); err != nil {
					return err
				}
				return imageapi.AddToScheme(scheme)
			},
			RootScopedKinds: sets.NewString("Image", "ImageSignature"),
		},
		announced.VersionToSchemeFunc{imageapiv1.SchemeGroupVersion.Version: imageapiv1.AddToScheme},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
