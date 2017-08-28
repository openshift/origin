package client

import (
	restclient "k8s.io/client-go/rest"
	authclientv1 "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/authorization/v1"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageclientv1 "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1"
	userclientv1 "github.com/openshift/origin/pkg/user/generated/clientset/typed/user/v1"
)

// RegistryClient provides Origin and Kubernetes clients to Docker Registry.
type RegistryClient interface {
	// Client returns the authenticated client to use with the server.
	Client() (Interface, error)
	// ClientFromToken returns a client based on the user bearer token.
	ClientFromToken(token string) (Interface, error)
}

// Interface contains client methods that registry use to communicate with
// Origin or Kubernetes API.
type Interface interface {
	ImageSignaturesInterfacer
	ImagesInterfacer
	ImageStreamImagesNamespacer
	ImageStreamMappingsNamespacer
	ImageStreamSecretsNamespacer
	ImageStreamsNamespacer
	ImageStreamTagsNamespacer
	LimitRangesGetter
	LocalSubjectAccessReviewsNamespacer
	SelfSubjectAccessReviewsNamespacer
	UsersInterfacer
}

type apiClient struct {
	oc    osclient.Interface
	kube  kcoreclient.CoreInterface
	auth  authclientv1.AuthorizationV1Interface
	image imageclientv1.ImageV1Interface
	user  userclientv1.UserV1Interface
}

func newAPIClient(
	c osclient.Interface,
	kc kcoreclient.CoreInterface,
	authClient authclientv1.AuthorizationV1Interface,
	imageClient imageclientv1.ImageV1Interface,
	userClient userclientv1.UserV1Interface,
) Interface {
	return &apiClient{
		oc:    c,
		kube:  kc,
		auth:  authClient,
		image: imageClient,
		user:  userClient,
	}
}

func (c *apiClient) Users() UserInterface {
	return c.user.Users()
}

func (c *apiClient) Images() ImageInterface {
	return c.image.Images()
}

func (c *apiClient) ImageSignatures() ImageSignatureInterface {
	return c.image.ImageSignatures()
}

func (c *apiClient) ImageStreams(namespace string) ImageStreamInterface {
	return c.image.ImageStreams(namespace)
}

func (c *apiClient) ImageStreamImages(namespace string) ImageStreamImageInterface {
	return c.image.ImageStreamImages(namespace)
}

func (c *apiClient) ImageStreamMappings(namespace string) ImageStreamMappingInterface {
	return c.image.ImageStreamMappings(namespace)
}

func (c *apiClient) ImageStreamTags(namespace string) ImageStreamTagInterface {
	return c.image.ImageStreamTags(namespace)
}

func (c *apiClient) ImageStreamSecrets(namespace string) ImageStreamSecretInterface {
	// FIXME: When we generate expansions for images clientset, replace this with
	// the clientset and get rid of the legacy client alltogether.
	return c.oc.ImageStreamSecrets(namespace)
}

func (c *apiClient) LimitRanges(namespace string) LimitRangeInterface {
	return c.kube.LimitRanges(namespace)
}

func (c *apiClient) LocalSubjectAccessReviews(namespace string) LocalSubjectAccessReviewInterface {
	return c.auth.LocalSubjectAccessReviews(namespace)
}

func (c *apiClient) SelfSubjectAccessReviews() SelfSubjectAccessReviewInterface {
	return c.auth.SelfSubjectAccessReviews()
}

type registryClient struct {
	kubeConfig      *restclient.Config
	openshiftConfig *restclient.Config
}

// NewRegistryClient provides a new registry client.
func NewRegistryClient(config *clientcmd.Config) RegistryClient {
	return &registryClient{
		kubeConfig:      config.KubeConfig(),
		openshiftConfig: config.OpenShiftConfig(),
	}
}

// Client returns the authenticated client to use with the server.
func (c *registryClient) Client() (Interface, error) {
	return newAPIClient(
		osclient.NewOrDie(c.openshiftConfig),
		kcoreclient.NewForConfigOrDie(c.kubeConfig),
		authclientv1.NewForConfigOrDie(c.kubeConfig),
		imageclientv1.NewForConfigOrDie(c.kubeConfig),
		userclientv1.NewForConfigOrDie(c.kubeConfig),
	), nil
}

// ClientFromToken returns the client based on the bearer token.
func (c *registryClient) ClientFromToken(token string) (Interface, error) {
	newClient := *c
	newOpenshiftConfig := clientcmd.AnonymousClientConfig(newClient.openshiftConfig)
	newOpenshiftConfig.BearerToken = token

	newKubeconfig := *newClient.kubeConfig
	newKubeconfig.BearerToken = token

	newClient.kubeConfig = &newKubeconfig
	newClient.openshiftConfig = &newOpenshiftConfig

	return newClient.Client()
}
