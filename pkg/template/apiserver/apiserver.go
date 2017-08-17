package apiserver

import (
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	authorizationinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	"sync"

	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	brokertemplateinstanceetcd "github.com/openshift/origin/pkg/template/registry/brokertemplateinstance/etcd"
	templateregistry "github.com/openshift/origin/pkg/template/registry/template"
	templateetcd "github.com/openshift/origin/pkg/template/registry/template/etcd"
	templateinstanceetcd "github.com/openshift/origin/pkg/template/registry/templateinstance/etcd"
)

type TemplateConfig struct {
	GenericConfig *genericapiserver.Config

	// AuthorizationClient is used to run SAR checks.  It should point to the master.
	AuthorizationClient authorizationinternalversion.AuthorizationInterface

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

// TemplateServer contains state for a Kubernetes cluster master/api server.
type TemplateServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*TemplateConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *TemplateConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *TemplateConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// New returns a new instance of TemplateServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*TemplateServer, error) {
	genericServer, err := c.TemplateConfig.GenericConfig.SkipComplete().New("template.openshift.io-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
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

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(templateapiv1.GroupName, c.Registry, c.Scheme, metav1.ParameterCodec, c.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = templateapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[templateapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *TemplateConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.makeV1Storage.Do(func() {
		c.v1Storage, c.v1StorageErr = c.newV1RESTStorage()
	})

	return c.v1Storage, c.v1StorageErr
}

func (c *TemplateConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	templateStorage, err := templateetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}
	templateInstanceStorage, templateInstanceStatusStorage, err := templateinstanceetcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.AuthorizationClient)
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
