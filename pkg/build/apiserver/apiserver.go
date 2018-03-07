package apiserver

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildclientset "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildetcd "github.com/openshift/origin/pkg/build/registry/build/etcd"
	"github.com/openshift/origin/pkg/build/registry/buildclone"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	"github.com/openshift/origin/pkg/build/registry/buildconfiginstantiate"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/bitbucket"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/build/webhook/gitlab"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
)

type ExtraConfig struct {
	KubeAPIServerClientConfig *restclient.Config
	KubeletClientConfig       *kubeletclient.KubeletClientConfig

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type BuildServerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// BuildServer contains state for a Kubernetes cluster master/api server.
type BuildServer struct {
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
func (c *BuildServerConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

// New returns a new instance of BuildServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*BuildServer, error) {
	genericServer, err := c.GenericConfig.New("build.openshift.io-apiserver", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &BuildServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	parameterCodec := runtime.NewParameterCodec(c.ExtraConfig.Scheme)
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(buildapiv1.GroupName, c.ExtraConfig.Registry, c.ExtraConfig.Scheme, parameterCodec, c.ExtraConfig.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = buildapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[buildapiv1.SchemeGroupVersion.Version] = v1Storage
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
	kubeInternalClient, err := kclientsetinternal.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	kubeExternalClient, err := kclientsetexternal.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	buildClient, err := buildclientset.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imageclient.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	nodeConnectionInfoGetter, err := kubeletclient.NewNodeConnectionInfoGetter(kubeExternalClient.CoreV1().Nodes(), *c.ExtraConfig.KubeletClientConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to configure the node connection info getter: %v", err)
	}

	buildStorage, buildDetailsStorage, err := buildetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	buildConfigStorage, err := buildconfigetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			Builds:            buildClient.Build(),
			BuildConfigs:      buildClient.Build(),
			ImageStreams:      imageClient.Image(),
			ImageStreamImages: imageClient.Image(),
			ImageStreamTags:   imageClient.Image(),
		},
		ServiceAccounts: kubeInternalClient.Core(),
		Secrets:         kubeInternalClient.Core(),
	}
	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildClient.Build(),
		kubeInternalClient.Core(),
		// We use the buildapiv1 schemegroup to encode the Build that gets
		// returned. As such, we need to make sure that the GroupVersion we use
		// is the same API version that the storage is going to be used for.
		buildapiv1.SchemeGroupVersion,
		map[string]webhook.Plugin{
			"generic":   generic.New(),
			"github":    github.New(),
			"gitlab":    gitlab.New(),
			"bitbucket": bitbucket.New(),
		},
	)

	v1Storage := map[string]rest.Storage{}
	v1Storage["builds"] = buildStorage
	v1Storage["builds/clone"] = buildclone.NewStorage(buildGenerator)
	v1Storage["builds/log"] = buildlogregistry.NewREST(buildClient.Build(), kubeInternalClient.Core(), nodeConnectionInfoGetter)
	v1Storage["builds/details"] = buildDetailsStorage

	v1Storage["buildConfigs"] = buildConfigStorage
	v1Storage["buildConfigs/webhooks"] = buildConfigWebHooks
	v1Storage["buildConfigs/instantiate"] = buildconfiginstantiate.NewStorage(buildGenerator)
	v1Storage["buildConfigs/instantiatebinary"] = buildconfiginstantiate.NewBinaryStorage(buildGenerator, buildClient.Build(), kubeInternalClient.Core(), nodeConnectionInfoGetter)
	return v1Storage, nil
}
