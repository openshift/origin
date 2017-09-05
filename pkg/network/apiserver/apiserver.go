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

	networkapiv1 "github.com/openshift/origin/pkg/network/apis/network/v1"
	clusternetworketcd "github.com/openshift/origin/pkg/network/registry/clusternetwork/etcd"
	egressnetworkpolicyetcd "github.com/openshift/origin/pkg/network/registry/egressnetworkpolicy/etcd"
	hostsubnetetcd "github.com/openshift/origin/pkg/network/registry/hostsubnet/etcd"
	netnamespaceetcd "github.com/openshift/origin/pkg/network/registry/netnamespace/etcd"
)

type NetworkAPIServerConfig struct {
	GenericConfig *genericapiserver.Config

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type NetworkAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*NetworkAPIServerConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *NetworkAPIServerConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *NetworkAPIServerConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// New returns a new instance of NetworkAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*NetworkAPIServer, error) {
	genericServer, err := c.NetworkAPIServerConfig.GenericConfig.SkipComplete().New("network.openshift.io-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &NetworkAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(networkapiv1.GroupName, c.Registry, c.Scheme, metav1.ParameterCodec, c.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = networkapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[networkapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *NetworkAPIServerConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.makeV1Storage.Do(func() {
		c.v1Storage, c.v1StorageErr = c.newV1RESTStorage()
	})

	return c.v1Storage, c.v1StorageErr
}

func (c *NetworkAPIServerConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	hostSubnetStorage, err := hostsubnetetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	netNamespaceStorage, err := netnamespaceetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	clusterNetworkStorage, err := clusternetworketcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	egressNetworkPolicyStorage, err := egressnetworkpolicyetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	v1Storage := map[string]rest.Storage{}
	v1Storage["hostSubnets"] = hostSubnetStorage
	v1Storage["netNamespaces"] = netNamespaceStorage
	v1Storage["clusterNetworks"] = clusterNetworkStorage
	v1Storage["egressNetworkPolicies"] = egressNetworkPolicyStorage
	return v1Storage, nil
}
