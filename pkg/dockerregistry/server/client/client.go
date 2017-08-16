package client

import (
	restclient "k8s.io/client-go/rest"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	authclientv1 "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/authorization/v1"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"

	"github.com/openshift/origin/pkg/client"

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
	SubjectAccessReviews
	UsersInterfacer
}

type apiClient struct {
	oc    client.Interface
	kube  kcoreclient.CoreInterface
	auth  authclientv1.AuthorizationV1Interface
	image imageclientv1.ImageV1Interface
	user  userclientv1.UserV1Interface
}

func newAPIClient(
	c client.Interface,
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
	return c.oc.ImageStreamSecrets(namespace)
}

func (c *apiClient) LimitRanges(namespace string) LimitRangeInterface {
	return c.kube.LimitRanges(namespace)
}

func (c *apiClient) LocalSubjectAccessReviews(namespace string) LocalSubjectAccessReviewInterface {
	return c.auth.LocalSubjectAccessReviews(namespace)
}

func (c *apiClient) SubjectAccessReviews() SubjectAccessReviewInterface {
	return c.auth.SelfSubjectAccessReviews()
}

type registryClient struct {
	config *clientcmd.Config
}

func NewRegistryClient(config *clientcmd.Config) RegistryClient {
	return &registryClient{config: config}
}

// Client returns the authenticated client to use with the server.
func (c *registryClient) Client() (Interface, error) {
	oc, kc, err := c.config.Clients()
	if err != nil {
		return nil, err
	}

	return newAPIClient(
		oc,
		kc.Core(),
		authclientv1.NewForConfigOrDie(c.config.KubeConfig()),
		imageclientv1.NewForConfigOrDie(c.config.KubeConfig()),
		userclientv1.NewForConfigOrDie(c.config.KubeConfig()),
	), nil
}

func (c *registryClient) ClientFromToken(token string) (Interface, error) {
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

	kubeConfig := c.config.KubeConfig()
	kubeConfig.BearerToken = token

	return newAPIClient(
		oc,
		kc.Core(),
		authclientv1.NewForConfigOrDie(kubeConfig),
		imageclientv1.NewForConfigOrDie(kubeConfig),
		userclientv1.NewForConfigOrDie(kubeConfig),
	), nil
}

// safeClientConfig returns a client config without authentication info.
func (с *registryClient) safeClientConfig() restclient.Config {
	return clientcmd.AnonymousClientConfig(с.config.OpenShiftConfig())
}
