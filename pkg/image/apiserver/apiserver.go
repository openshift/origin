package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	imageapiv1 "github.com/openshift/api/image/v1"
	imageclientv1 "github.com/openshift/client-go/image/clientset/versioned"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/image/apis/image/validation/whitelist"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	"github.com/openshift/origin/pkg/image/apiserver/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/apiserver/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagesecret"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagesignature"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/apiserver/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestreamimport"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestreamtag"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	"github.com/openshift/origin/pkg/image/importer"
	imageimporter "github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/importer/dockerv1client"
)

type ExtraConfig struct {
	KubeAPIServerClientConfig          *restclient.Config
	LimitVerifier                      imageadmission.LimitVerifier
	RegistryHostnameRetriever          registryhostname.RegistryHostnameRetriever
	AllowedRegistriesForImport         *configapi.AllowedRegistries
	MaxImagesBulkImportedPerRepository int
	AdditionalTrustedCA                []byte

	// TODO these should all become local eventually
	Scheme *runtime.Scheme
	Codecs serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
	startFns      []func(<-chan struct{})
}

type ImageAPIServerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type ImageAPIServer struct {
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
func (c *ImageAPIServerConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

// New returns a new instance of ImageAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*ImageAPIServer, error) {
	genericServer, err := c.GenericConfig.New("image.openshift.io-apiserver", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &ImageAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(imageapiv1.GroupName, c.ExtraConfig.Scheme, metav1.ParameterCodec, c.ExtraConfig.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[imageapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	if err := s.GenericAPIServer.AddPostStartHook("image.openshift.io-apiserver-caches", func(context genericapiserver.PostStartHookContext) error {
		for _, fn := range c.ExtraConfig.startFns {
			go fn(context.StopCh)
		}
		return nil
	}); err != nil {
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
	cfg := restclient.Config{}

	tlsConfig := &tls.Config{}

	var err error
	tlsConfig.RootCAs, err = x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("unable to get system cert pool for default transport for image importing: %v", err)
	}
	if tlsConfig.RootCAs == nil {
		tlsConfig.RootCAs = x509.NewCertPool()
	}

	if len(c.ExtraConfig.AdditionalTrustedCA) != 0 {
		if ok := tlsConfig.RootCAs.AppendCertsFromPEM(c.ExtraConfig.AdditionalTrustedCA); !ok {
			return nil, fmt.Errorf("No valid certificates read from %v", c.ExtraConfig.AdditionalTrustedCA)
		}
	}

	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
	})

	importTransport, err := restclient.HTTPWrappersForConfig(&cfg, transport)
	if err != nil {
		return nil, fmt.Errorf("unable to configure a default transport for importing: %v", err)
	}

	insecureImportTransport, err := restclient.TransportFor(&restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to configure a default transport for importing: %v", err)
	}

	coreClient, err := coreclient.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	authorizationClient, err := authorizationclient.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imageclient.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	imageV1Client, err := imageclientv1.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	imageStorage, err := imageetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	var whitelister whitelist.RegistryWhitelister
	if c.ExtraConfig.AllowedRegistriesForImport != nil {
		whitelister, err = whitelist.NewRegistryWhitelister(
			*c.ExtraConfig.AllowedRegistriesForImport,
			c.ExtraConfig.RegistryHostnameRetriever)
		if err != nil {
			return nil, fmt.Errorf("error building registry whitelister: %v", err)
		}
	} else {
		whitelister = whitelist.WhitelistAllRegistries()
	}

	imageLayerIndex := imagestreametcd.NewImageLayerIndex(imageV1Client.Image().Images())
	c.ExtraConfig.startFns = append(c.ExtraConfig.startFns, imageLayerIndex.Run)

	imageRegistry := image.NewRegistry(imageStorage)
	imageSignatureStorage := imagesignature.NewREST(imageClient.Image())
	imageStreamSecretsStorage := imagesecret.NewREST(coreClient)
	imageStreamStorage, imageStreamLayersStorage, imageStreamStatusStorage, internalImageStreamStorage, err := imagestreametcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.ExtraConfig.RegistryHostnameRetriever, authorizationClient.SubjectAccessReviews(), c.ExtraConfig.LimitVerifier, whitelister, imageLayerIndex)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry, c.ExtraConfig.RegistryHostnameRetriever)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry, whitelister)
	importerCache, err := imageimporter.NewImageStreamLayerCache(imageimporter.DefaultImageStreamLayerCacheSize)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	importerFn := func(r importer.RepositoryRetriever) imageimporter.Interface {
		return imageimporter.NewImageStreamImporter(r, c.ExtraConfig.MaxImagesBulkImportedPerRepository, flowcontrol.NewTokenBucketRateLimiter(2.0, 3), &importerCache)
	}
	importerDockerClientFn := func() dockerv1client.Client {
		return dockerv1client.NewClient(20*time.Second, false)
	}
	imageStreamImportStorage := imagestreamimport.NewREST(
		importerFn,
		imageStreamRegistry,
		internalImageStreamStorage,
		imageStorage,
		imageV1Client.ImageV1(),
		importTransport,
		insecureImportTransport,
		importerDockerClientFn,
		whitelister,
		authorizationClient.SubjectAccessReviews())
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)

	v1Storage := map[string]rest.Storage{}
	v1Storage["images"] = imageStorage
	v1Storage["imagesignatures"] = imageSignatureStorage
	v1Storage["imageStreams/secrets"] = imageStreamSecretsStorage
	v1Storage["imageStreams"] = imageStreamStorage
	v1Storage["imageStreams/layers"] = imageStreamLayersStorage
	v1Storage["imageStreams/status"] = imageStreamStatusStorage
	v1Storage["imageStreamImports"] = imageStreamImportStorage
	v1Storage["imageStreamImages"] = imageStreamImageStorage
	v1Storage["imageStreamMappings"] = imageStreamMappingStorage
	v1Storage["imageStreamTags"] = imageStreamTagStorage
	return v1Storage, nil
}
