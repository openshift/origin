package apiserver

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"

	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/route/etcd"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
)

type RouteAPIServerConfig struct {
	GenericConfig *genericapiserver.Config

	CoreAPIServerClientConfig *restclient.Config
	RouteAllocator            *routeallocationcontroller.RouteAllocationController

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type RouteAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*RouteAPIServerConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *RouteAPIServerConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *RouteAPIServerConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// New returns a new instance of RouteAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*RouteAPIServer, error) {
	genericServer, err := c.RouteAPIServerConfig.GenericConfig.SkipComplete().New("route.openshift.io-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &RouteAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(routeapiv1.GroupName, c.Registry, c.Scheme, metav1.ParameterCodec, c.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = routeapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[routeapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *RouteAPIServerConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.makeV1Storage.Do(func() {
		c.v1Storage, c.v1StorageErr = c.newV1RESTStorage()
	})

	return c.v1Storage, c.v1StorageErr
}

func (c *RouteAPIServerConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	authorizationClient, err := authorizationclient.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	routeStorage, routeStatusStorage, err := routeetcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.RouteAllocator, authorizationClient.SubjectAccessReviews())
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	v1Storage := map[string]rest.Storage{}
	v1Storage["routes"] = routeStorage
	v1Storage["routes/status"] = routeStatusStorage
	return v1Storage, nil
}
