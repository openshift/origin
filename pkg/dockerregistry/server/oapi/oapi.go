package oapi

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

type RegistryClient interface {
	// Client returns the authenticated client to use with the server.
	Client() (ClientInterface, error)
	// UserClient returns the user client.
	UserClient(token string) (ClientInterface, error)
}

type ClientInterface interface {
	ClientImageSignaturesInterfacer
	ClientImagesInterfacer
	ClientImageStreamImagesNamespacer
	ClientImageStreamMappingsNamespacer
	ClientImageStreamSecretsNamespacer
	ClientImageStreamsNamespacer
	ClientImageStreamTagsNamespacer
	ClientLimitRangesGetter
	ClientLocalSubjectAccessReviewsNamespacer
	ClientSubjectAccessReviews
	ClientUsersInterfacer
}

type ClientUsersInterfacer interface {
	Users() ClientUserInterface
}

type ClientImagesInterfacer interface {
	Images() ClientImageInterface
}

type ClientImageSignaturesInterfacer interface {
	ImageSignatures() ClientImageSignatureInterface
}

type ClientImageStreamImagesNamespacer interface {
	ImageStreamImages(namespace string) ClientImageStreamImageInterface
}

type ClientImageStreamsNamespacer interface {
	ImageStreams(namespace string) ClientImageStreamInterface
}

type ClientImageStreamMappingsNamespacer interface {
	ImageStreamMappings(namespace string) ClientImageStreamMappingInterface
}

type ClientImageStreamSecretsNamespacer interface {
	ImageStreamSecrets(namespace string) ClientImageStreamSecretInterface
}

type ClientImageStreamTagsNamespacer interface {
	ImageStreamTags(namespace string) ClientImageStreamTagInterface
}

type ClientLimitRangesGetter interface {
	LimitRanges(namespace string) ClientLimitRangeInterface
}

type ClientLocalSubjectAccessReviewsNamespacer interface {
	LocalSubjectAccessReviews(namespace string) ClientLocalSubjectAccessReviewInterface
}

type ClientSubjectAccessReviews interface {
	SubjectAccessReviews() ClientSubjectAccessReviewInterface
}

type ClientImageSignatureInterface interface {
	Create(signature *imageapi.ImageSignature) (*imageapi.ImageSignature, error)
}

type ClientImageStreamImageInterface interface {
	Get(name, id string) (*imageapi.ImageStreamImage, error)
}

type ClientUserInterface interface {
	Get(name string, options metav1.GetOptions) (*userapi.User, error)
}

type ClientImageInterface interface {
	Get(name string, options metav1.GetOptions) (*imageapi.Image, error)
	Update(image *imageapi.Image) (*imageapi.Image, error)
}

type ClientImageStreamInterface interface {
	Get(name string, options metav1.GetOptions) (*imageapi.ImageStream, error)
	Create(stream *imageapi.ImageStream) (*imageapi.ImageStream, error)
}

type ClientImageStreamMappingInterface interface {
	Create(mapping *imageapi.ImageStreamMapping) error
}

type ClientImageStreamSecretInterface interface {
	Secrets(name string, options metav1.ListOptions) (*kapi.SecretList, error)
}

type ClientImageStreamTagInterface interface {
	Delete(name, tag string) error
}

type ClientLimitRangeInterface interface {
	List(opts metav1.ListOptions) (*kapi.LimitRangeList, error)
}

type ClientLocalSubjectAccessReviewInterface interface {
	Create(policy *authorizationapi.LocalSubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error)
}

type ClientSubjectAccessReviewInterface interface {
	Create(policy *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error)
}

type oapiClient struct {
	osClient client.Interface
	kClient  kcoreclient.CoreInterface
}

func NewAPIClient(c client.Interface, kc kcoreclient.CoreInterface) ClientInterface {
	return &oapiClient{
		osClient: c,
		kClient:  kc,
	}
}

func (c *oapiClient) Users() ClientUserInterface {
	return c.osClient.Users()
}

func (c *oapiClient) Images() ClientImageInterface {
	return c.osClient.Images()
}

func (c *oapiClient) ImageSignatures() ClientImageSignatureInterface {
	return c.osClient.ImageSignatures()
}

func (c *oapiClient) ImageStreams(namespace string) ClientImageStreamInterface {
	return c.osClient.ImageStreams(namespace)
}

func (c *oapiClient) ImageStreamImages(namespace string) ClientImageStreamImageInterface {
	return c.osClient.ImageStreamImages(namespace)
}

func (c *oapiClient) ImageStreamMappings(namespace string) ClientImageStreamMappingInterface {
	return c.osClient.ImageStreamMappings(namespace)
}

func (c *oapiClient) ImageStreamSecrets(namespace string) ClientImageStreamSecretInterface {
	return c.osClient.ImageStreamSecrets(namespace)
}

func (c *oapiClient) ImageStreamTags(namespace string) ClientImageStreamTagInterface {
	return c.osClient.ImageStreamTags(namespace)
}

func (c *oapiClient) LimitRanges(namespace string) ClientLimitRangeInterface {
	return c.kClient.LimitRanges(namespace)
}

func (c *oapiClient) LocalSubjectAccessReviews(namespace string) ClientLocalSubjectAccessReviewInterface {
	return c.osClient.LocalSubjectAccessReviews(namespace)
}

func (c *oapiClient) SubjectAccessReviews() ClientSubjectAccessReviewInterface {
	return c.osClient.SubjectAccessReviews()
}

type registryClient struct {
	config *clientcmd.Config
}

func NewRegistryClient(config *clientcmd.Config) RegistryClient {
	return &registryClient{config: config}
}

// Client returns the authenticated client to use with the server.
func (c *registryClient) Client() (ClientInterface, error) {
	oc, kc, err := c.config.Clients()
	if err != nil {
		return nil, err
	}

	return NewAPIClient(oc, kc.Core()), nil
}

func (c *registryClient) UserClient(token string) (ClientInterface, error) {
	_, kc, err := c.config.Clients()
	if err != nil {
		return nil, err
	}

	cfg := c.safeClientConfig()
	cfg.BearerToken = token

	oc, err := client.New(&cfg)
	if err != nil {
		return nil, err
	}

	return NewAPIClient(oc, kc.Core()), nil
}

// safeClientConfig returns a client config without authentication info.
func (с *registryClient) safeClientConfig() restclient.Config {
	return clientcmd.AnonymousClientConfig(с.config.OpenShiftConfig())
}
