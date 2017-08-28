package internalversion

import (
	"github.com/openshift/origin/pkg/user/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type UserInterface interface {
	RESTClient() rest.Interface
	GroupsGetter
	IdentitiesGetter
	UsersGetter
	UserIdentityMappingsGetter
}

// UserClient is used to interact with features provided by the user.openshift.io group.
type UserClient struct {
	restClient rest.Interface
}

func (c *UserClient) Groups() GroupInterface {
	return newGroups(c)
}

func (c *UserClient) Identities() IdentityInterface {
	return newIdentities(c)
}

func (c *UserClient) Users() UserResourceInterface {
	return newUsers(c)
}

func (c *UserClient) UserIdentityMappings() UserIdentityMappingInterface {
	return newUserIdentityMappings(c)
}

// NewForConfig creates a new UserClient for the given config.
func NewForConfig(c *rest.Config) (*UserClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &UserClient{client}, nil
}

// NewForConfigOrDie creates a new UserClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *UserClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new UserClient for the given RESTClient.
func New(c rest.Interface) *UserClient {
	return &UserClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("user.openshift.io")
	if err != nil {
		return err
	}

	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		gv := g.GroupVersion
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *UserClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
