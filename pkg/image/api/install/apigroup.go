package install

import (
	"k8s.io/kubernetes/pkg/apimachinery/announced"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/docker10"
	"github.com/openshift/origin/pkg/image/api/dockerpre012"
	"github.com/openshift/origin/pkg/image/api/v1"
)

func init() {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName: api.GroupName,
			VersionPreferenceOrder: []string{
				v1.SchemeGroupVersion.Version,
				dockerpre012.SchemeGroupVersion.Version,
				docker10.SchemeGroupVersion.Version,
			},
			ImportPrefix: importPrefix,
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
		announced.VersionToSchemeFunc{
			docker10.SchemeGroupVersion.Version:     docker10.AddToScheme,
			dockerpre012.SchemeGroupVersion.Version: dockerpre012.AddToScheme,
			v1.SchemeGroupVersion.Version:           v1.AddToScheme,
		},
	).Announce().RegisterAndEnable(); err != nil {
		panic(err)
	}
}
