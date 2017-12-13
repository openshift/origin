package origin

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionscmd "k8s.io/apiextensions-apiserver/pkg/cmd/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

func createAPIExtensionsConfig(kubeAPIServerConfig genericapiserver.Config, kubeEtcdOptions *genericoptions.EtcdOptions) (*apiextensionsapiserver.Config, error) {
	// make a shallow copy to let us twiddle a few things
	// most of the config actually remains the same.  We only need to mess with a couple items related to the particulars of the apiextensions
	recommendedConfig := genericapiserver.RecommendedConfig{
		Config: kubeAPIServerConfig,
	}

	// copy the etcd options so we don't mutate originals.
	etcdOptions := *kubeEtcdOptions
	etcdOptions.StorageConfig.Codec = apiextensionsapiserver.Codecs.LegacyCodec(v1beta1.SchemeGroupVersion)
	recommendedConfig.RESTOptionsGetter = &genericoptions.SimpleRestOptionsFactory{Options: etcdOptions}

	apiextensionsConfig := &apiextensionsapiserver.Config{
		GenericConfig: &recommendedConfig,
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionscmd.NewCRDRESTOptionsGetter(etcdOptions),
		},
	}

	return apiextensionsConfig, nil

}

func createAPIExtensionsServer(apiextensionsConfig *apiextensionsapiserver.Config, delegateAPIServer genericapiserver.DelegationTarget) (*apiextensionsapiserver.CustomResourceDefinitions, error) {
	apiextensionsServer, err := apiextensionsConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}

	return apiextensionsServer, nil
}
