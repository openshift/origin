package metrics

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/dockerregistry/server/oapi"
	imageapi "github.com/openshift/origin/pkg/image/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

type oapiClient struct {
	client oapi.ClientInterface
}

// newOAPIClient wraps c with the Prometheus instrumentation.
func newOAPIClient(c oapi.ClientInterface) oapi.ClientInterface {
	return &oapiClient{
		client: c,
	}
}

func (c *oapiClient) Users() oapi.ClientUserInterface {
	return &oapiClientUsers{client: c.client.Users()}
}

func (c *oapiClient) Images() oapi.ClientImageInterface {
	return &oapiClientImages{client: c.client.Images()}
}

func (c *oapiClient) ImageSignatures() oapi.ClientImageSignatureInterface {
	return &oapiClientImageSignatures{client: c.client.ImageSignatures()}
}

func (c *oapiClient) ImageStreamImages(namespace string) oapi.ClientImageStreamImageInterface {
	return &oapiClientImageStreamImages{client: c.client.ImageStreamImages(namespace)}
}

func (c *oapiClient) ImageStreams(namespace string) oapi.ClientImageStreamInterface {
	return &oapiClientImageStreams{client: c.client.ImageStreams(namespace)}
}

func (c *oapiClient) ImageStreamMappings(namespace string) oapi.ClientImageStreamMappingInterface {
	return &oapiClientImageStreamMappings{client: c.client.ImageStreamMappings(namespace)}
}

func (c *oapiClient) ImageStreamSecrets(namespace string) oapi.ClientImageStreamSecretInterface {
	return &oapiClientImageStreamSecrets{client: c.client.ImageStreamSecrets(namespace)}
}

func (c *oapiClient) ImageStreamTags(namespace string) oapi.ClientImageStreamTagInterface {
	return &oapiClientImageStreamTags{client: c.client.ImageStreamTags(namespace)}
}

func (c *oapiClient) LimitRanges(namespace string) oapi.ClientLimitRangeInterface {
	return &oapiClientLimitRanges{client: c.client.LimitRanges(namespace)}
}

func (c *oapiClient) LocalSubjectAccessReviews(namespace string) oapi.ClientLocalSubjectAccessReviewInterface {
	return &oapiClientLocalSubjectAccessReviews{client: c.client.LocalSubjectAccessReviews(namespace)}
}

func (c *oapiClient) SubjectAccessReviews() oapi.ClientSubjectAccessReviewInterface {
	return &oapiClientSubjectAccessReviews{client: c.client.SubjectAccessReviews()}
}

type oapiClientImages struct {
	client oapi.ClientImageInterface
}

func (c *oapiClientImages) Get(name string, options metav1.GetOptions) (*imageapi.Image, error) {
	defer NewTimer(MasterAPIRequests, []string{"images.get"}).Stop()
	return c.client.Get(name, options)
}

func (c *oapiClientImages) Update(image *imageapi.Image) (*imageapi.Image, error) {
	defer NewTimer(MasterAPIRequests, []string{"images.update"}).Stop()
	return c.client.Update(image)
}

type oapiClientImageSignatures struct {
	client oapi.ClientImageSignatureInterface
}

func (c *oapiClientImageSignatures) Create(signature *imageapi.ImageSignature) (*imageapi.ImageSignature, error) {
	defer NewTimer(MasterAPIRequests, []string{"imagesignatures.create"}).Stop()
	return c.client.Create(signature)
}

type oapiClientImageStreamImages struct {
	client oapi.ClientImageStreamImageInterface
}

func (c *oapiClientImageStreamImages) Get(name, id string) (*imageapi.ImageStreamImage, error) {
	defer NewTimer(MasterAPIRequests, []string{"imagestreamimages.get"}).Stop()
	return c.client.Get(name, id)
}

type oapiClientImageStreams struct {
	client oapi.ClientImageStreamInterface
}

func (c *oapiClientImageStreams) Get(name string, options metav1.GetOptions) (*imageapi.ImageStream, error) {
	defer NewTimer(MasterAPIRequests, []string{"imagestreams.get"}).Stop()
	return c.client.Get(name, options)
}

func (c *oapiClientImageStreams) Create(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	defer NewTimer(MasterAPIRequests, []string{"imagestreams.create"}).Stop()
	return c.client.Create(stream)
}

type oapiClientImageStreamMappings struct {
	client oapi.ClientImageStreamMappingInterface
}

func (c *oapiClientImageStreamMappings) Create(mapping *imageapi.ImageStreamMapping) error {
	defer NewTimer(MasterAPIRequests, []string{"imagestreammapping.create"}).Stop()
	return c.client.Create(mapping)
}

type oapiClientImageStreamSecrets struct {
	client oapi.ClientImageStreamSecretInterface
}

func (c *oapiClientImageStreamSecrets) Secrets(name string, options metav1.ListOptions) (*kapi.SecretList, error) {
	defer NewTimer(MasterAPIRequests, []string{"imagestreamsecrets.secrets"}).Stop()
	return c.client.Secrets(name, options)
}

type oapiClientImageStreamTags struct {
	client oapi.ClientImageStreamTagInterface
}

func (c *oapiClientImageStreamTags) Delete(name, tag string) error {
	defer NewTimer(MasterAPIRequests, []string{"imagestreamtags.delete"}).Stop()
	return c.client.Delete(name, tag)
}

type oapiClientLimitRanges struct {
	client oapi.ClientLimitRangeInterface
}

func (c *oapiClientLimitRanges) List(opts metav1.ListOptions) (*kapi.LimitRangeList, error) {
	defer NewTimer(MasterAPIRequests, []string{"limitranges.list"}).Stop()
	return c.client.List(opts)
}

type oapiClientUsers struct {
	client oapi.ClientUserInterface
}

func (c *oapiClientUsers) Get(name string, options metav1.GetOptions) (*userapi.User, error) {
	defer NewTimer(MasterAPIRequests, []string{"users.get"}).Stop()
	return c.client.Get(name, options)
}

type oapiClientLocalSubjectAccessReviews struct {
	client oapi.ClientLocalSubjectAccessReviewInterface
}

func (c *oapiClientLocalSubjectAccessReviews) Create(policy *authorizationapi.LocalSubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	defer NewTimer(MasterAPIRequests, []string{"localsubjectaccessreviews.create"}).Stop()
	return c.client.Create(policy)
}

type oapiClientSubjectAccessReviews struct {
	client oapi.ClientSubjectAccessReviewInterface
}

func (c *oapiClientSubjectAccessReviews) Create(policy *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	defer NewTimer(MasterAPIRequests, []string{"subjectaccessreviews.create"}).Stop()
	return c.client.Create(policy)
}

type registryClient struct {
	client oapi.RegistryClient
}

func NewRegistryClient(client oapi.RegistryClient) oapi.RegistryClient {
	return &registryClient{client: client}
}

func (c *registryClient) Client() (oapi.ClientInterface, error) {
	client, err := c.client.Client()
	if err != nil {
		return nil, err
	}
	return newOAPIClient(client), nil
}

func (c *registryClient) UserClient(token string) (oapi.ClientInterface, error) {
	client, err := c.client.UserClient(token)
	if err != nil {
		return nil, err
	}
	return newOAPIClient(client), nil
}
