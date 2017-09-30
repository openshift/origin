package internalversion

import (
	"github.com/openshift/origin/pkg/security/generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type SecurityInterface interface {
	RESTClient() rest.Interface
	PodSecurityPolicyReviewsGetter
	PodSecurityPolicySelfSubjectReviewsGetter
	PodSecurityPolicySubjectReviewsGetter
	SecurityContextConstraintsGetter
}

// SecurityClient is used to interact with features provided by the security.openshift.io group.
type SecurityClient struct {
	restClient rest.Interface
}

func (c *SecurityClient) PodSecurityPolicyReviews(namespace string) PodSecurityPolicyReviewInterface {
	return newPodSecurityPolicyReviews(c, namespace)
}

func (c *SecurityClient) PodSecurityPolicySelfSubjectReviews(namespace string) PodSecurityPolicySelfSubjectReviewInterface {
	return newPodSecurityPolicySelfSubjectReviews(c, namespace)
}

func (c *SecurityClient) PodSecurityPolicySubjectReviews(namespace string) PodSecurityPolicySubjectReviewInterface {
	return newPodSecurityPolicySubjectReviews(c, namespace)
}

func (c *SecurityClient) SecurityContextConstraints() SecurityContextConstraintsInterface {
	return newSecurityContextConstraints(c)
}

// NewForConfig creates a new SecurityClient for the given config.
func NewForConfig(c *rest.Config) (*SecurityClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SecurityClient{client}, nil
}

// NewForConfigOrDie creates a new SecurityClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SecurityClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SecurityClient for the given RESTClient.
func New(c rest.Interface) *SecurityClient {
	return &SecurityClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("security.openshift.io")
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
func (c *SecurityClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
