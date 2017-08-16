package apiserver

import (
	"sync"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	userapiv1 "github.com/openshift/origin/pkg/user/apis/user/v1"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
	groupetcd "github.com/openshift/origin/pkg/user/registry/group/etcd"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

type UserConfig struct {
	GenericConfig *genericapiserver.Config

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type UserServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*UserConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *UserConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *UserConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// New returns a new instance of UserServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*UserServer, error) {
	genericServer, err := c.UserConfig.GenericConfig.SkipComplete().New("user.openshift.io-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &UserServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(userapiv1.GroupName, c.Registry, c.Scheme, metav1.ParameterCodec, c.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = userapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[userapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *UserConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.makeV1Storage.Do(func() {
		c.v1Storage, c.v1StorageErr = c.newV1RESTStorage()
	})

	return c.v1Storage, c.v1StorageErr
}

func (c *UserConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	userClient, err := userclient.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	userStorage, err := useretcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}
	identityStorage, err := identityetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}
	userIdentityMappingStorage := useridentitymapping.NewREST(userClient.Users(), userClient.Identities())
	groupStorage, err := groupetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}

	v1Storage := map[string]rest.Storage{}
	v1Storage["users"] = userStorage
	v1Storage["groups"] = groupStorage
	v1Storage["identities"] = identityStorage
	v1Storage["userIdentityMappings"] = userIdentityMappingStorage
	return v1Storage, nil
}
