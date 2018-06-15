package apiserver

import (
	"sync"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	templateapiv1 "github.com/openshift/api/template/v1"
	brokertemplateinstanceetcd "github.com/openshift/origin/pkg/template/registry/brokertemplateinstance/etcd"
	templateregistry "github.com/openshift/origin/pkg/template/registry/template"
	templateetcd "github.com/openshift/origin/pkg/template/registry/template/etcd"
	templateinstanceetcd "github.com/openshift/origin/pkg/template/registry/templateinstance/etcd"
)

type ExtraConfig struct {
	KubeAPIServerClientConfig *restclient.Config

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type TemplateConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// TemplateServer contains state for a Kubernetes cluster master/api server.
type TemplateServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *TemplateConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

// New returns a new instance of TemplateServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*TemplateServer, error) {
	genericServer, err := c.GenericConfig.New("template.openshift.io-apiserver", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &TemplateServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(templateapiv1.GroupName, c.ExtraConfig.Registry, c.ExtraConfig.Scheme, metav1.ParameterCodec, c.ExtraConfig.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = templateapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[templateapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *completedConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.ExtraConfig.makeV1Storage.Do(func() {
		c.ExtraConfig.v1Storage, c.ExtraConfig.v1StorageErr = c.newV1RESTStorage()
	})

	return c.ExtraConfig.v1Storage, c.ExtraConfig.v1StorageErr
}

func (c *completedConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	authorizationClient, err := authorizationclient.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}

	templateStorage, err := templateetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}
	templateInstanceStorage, templateInstanceStatusStorage, err := templateinstanceetcd.NewREST(c.GenericConfig.RESTOptionsGetter, authorizationClient)
	if err != nil {
		return nil, err
	}
	brokerTemplateInstanceStorage, err := brokertemplateinstanceetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}

	v1Storage := map[string]rest.Storage{}
	v1Storage["processedTemplates"] = templateregistry.NewREST()
	v1Storage["templates"] = templateStorage
	v1Storage["templateinstances"] = templateInstanceStorage
	v1Storage["templateinstances/status"] = templateInstanceStatusStorage
	v1Storage["brokertemplateinstances"] = brokerTemplateInstanceStorage
	return v1Storage, nil
}
