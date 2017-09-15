package apiserver

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	"github.com/openshift/origin/pkg/image/importer"
	imageimporter "github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/importer/dockerv1client"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagesecret"
	"github.com/openshift/origin/pkg/image/registry/imagesignature"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimport"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
)

type ImageAPIServerConfig struct {
	GenericConfig *genericapiserver.Config

	CoreAPIServerClientConfig          *restclient.Config
	LimitVerifier                      imageadmission.LimitVerifier
	RegistryHostnameRetriever          imageapi.RegistryHostnameRetriever
	AllowedRegistriesForImport         *configapi.AllowedRegistries
	MaxImagesBulkImportedPerRepository int

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type ImageAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*ImageAPIServerConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *ImageAPIServerConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *ImageAPIServerConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// New returns a new instance of ImageAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*ImageAPIServer, error) {
	genericServer, err := c.ImageAPIServerConfig.GenericConfig.SkipComplete().New("image.openshift.io-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
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

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(imageapiv1.GroupName, c.Registry, c.Scheme, metav1.ParameterCodec, c.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = imageapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[imageapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *ImageAPIServerConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.makeV1Storage.Do(func() {
		c.v1Storage, c.v1StorageErr = c.newV1RESTStorage()
	})

	return c.v1Storage, c.v1StorageErr
}

func (c *ImageAPIServerConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	// TODO: allow the system CAs and the local CAs to be joined together.
	importTransport, err := restclient.TransportFor(&restclient.Config{})
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

	coreClient, err := coreclient.NewForConfig(c.CoreAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	authorizationClient, err := authorizationclient.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imageclient.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	imageStorage, err := imageetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	imageRegistry := image.NewRegistry(imageStorage)
	imageSignatureStorage := imagesignature.NewREST(imageClient)
	imageStreamSecretsStorage := imagesecret.NewREST(coreClient)
	imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage, err := imagestreametcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.RegistryHostnameRetriever, authorizationClient.SubjectAccessReviews(), c.LimitVerifier)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry, c.RegistryHostnameRetriever)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	importerCache, err := imageimporter.NewImageStreamLayerCache(imageimporter.DefaultImageStreamLayerCacheSize)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	importerFn := func(r importer.RepositoryRetriever) imageimporter.Interface {
		return imageimporter.NewImageStreamImporter(r, c.MaxImagesBulkImportedPerRepository, flowcontrol.NewTokenBucketRateLimiter(2.0, 3), &importerCache)
	}
	importerDockerClientFn := func() dockerv1client.Client {
		return dockerv1client.NewClient(20*time.Second, false)
	}
	imageStreamImportStorage := imagestreamimport.NewREST(
		importerFn,
		imageStreamRegistry,
		internalImageStreamStorage,
		imageStorage,
		imageClient,
		importTransport,
		insecureImportTransport,
		importerDockerClientFn,
		c.AllowedRegistriesForImport,
		c.RegistryHostnameRetriever,
		authorizationClient.SubjectAccessReviews())
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)

	v1Storage := map[string]rest.Storage{}
	v1Storage["images"] = imageStorage
	v1Storage["imagesignatures"] = imageSignatureStorage
	v1Storage["imageStreams/secrets"] = imageStreamSecretsStorage
	v1Storage["imageStreams"] = imageStreamStorage
	v1Storage["imageStreams/status"] = imageStreamStatusStorage
	v1Storage["imageStreamImports"] = imageStreamImportStorage
	v1Storage["imageStreamImages"] = imageStreamImageStorage
	v1Storage["imageStreamMappings"] = imageStreamMappingStorage
	v1Storage["imageStreamTags"] = imageStreamTagStorage
	return v1Storage, nil
}
