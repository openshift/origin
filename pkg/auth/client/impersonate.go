package client

import (
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/client"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

type impersonatingRoundTripper struct {
	user     user.Info
	delegate http.RoundTripper
}

// NewImpersonatingRoundTripper will add headers to impersonate a user, including user, groups, and scopes
func NewImpersonatingRoundTripper(user user.Info, delegate http.RoundTripper) http.RoundTripper {
	return &impersonatingRoundTripper{user: user, delegate: delegate}
}

func (rt *impersonatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = utilnet.CloneRequest(req)
	req.Header.Del(authenticationapi.ImpersonateUserHeader)
	req.Header.Del(authenticationapi.ImpersonateGroupHeader)
	req.Header.Del(authenticationapi.ImpersonateUserScopeHeader)

	req.Header.Set(authenticationapi.ImpersonateUserHeader, rt.user.GetName())
	for _, group := range rt.user.GetGroups() {
		req.Header.Add(authenticationapi.ImpersonateGroupHeader, group)
	}
	for _, scope := range rt.user.GetExtra()[authorizationapi.ScopesKey] {
		req.Header.Add(authenticationapi.ImpersonateUserScopeHeader, scope)
	}
	return rt.delegate.RoundTrip(req)
}

// NewImpersonatingConfig wraps the config's transport to impersonate a user, including user, groups, and scopes
func NewImpersonatingConfig(user user.Info, config restclient.Config) restclient.Config {
	oldWrapTransport := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return NewImpersonatingRoundTripper(user, oldWrapTransport(rt))
	}
	return config
}

// NewImpersonatingOpenShiftClient returns an OpenShift client that will impersonate a user, including user, groups, and scopes
func NewImpersonatingOpenShiftClient(user user.Info, config restclient.Config) (client.Interface, error) {
	impersonatingConfig := NewImpersonatingConfig(user, config)
	return client.New(&impersonatingConfig)
}

// NewImpersonatingKubernetesClientset returns a Kubernetes clientset that will impersonate a user, including user, groups, and scopes
func NewImpersonatingKubernetesClientset(user user.Info, config restclient.Config) (kclientset.Interface, error) {
	impersonatingConfig := NewImpersonatingConfig(user, config)
	return kclientset.NewForConfig(&impersonatingConfig)
}

func NewImpersonatingKubernetesClientsetFromRESTClient(user user.Info, client restclient.Interface) kclientset.Interface {
	return kclientset.New(NewImpersonatingRESTClient(user, client))
}

// impersonatingRESTClient sets impersonating user, groups, and scopes headers per request
type impersonatingRESTClient struct {
	user     user.Info
	delegate restclient.Interface
}

func NewImpersonatingRESTClient(user user.Info, client restclient.Interface) restclient.Interface {
	return &impersonatingRESTClient{user: user, delegate: client}
}

// Verb does the impersonation per request by setting the proper headers
func (c impersonatingRESTClient) impersonate(req *restclient.Request) *restclient.Request {
	req.SetHeader(authenticationapi.ImpersonateUserHeader, c.user.GetName())
	req.SetHeader(authenticationapi.ImpersonateGroupHeader, c.user.GetGroups()...)
	req.SetHeader(authenticationapi.ImpersonateUserScopeHeader, c.user.GetExtra()[authorizationapi.ScopesKey]...)
	return req
}

func (c impersonatingRESTClient) Verb(verb string) *restclient.Request {
	return c.impersonate(c.delegate.Verb(verb))
}

func (c impersonatingRESTClient) Post() *restclient.Request {
	return c.impersonate(c.delegate.Post())
}

func (c impersonatingRESTClient) Put() *restclient.Request {
	return c.impersonate(c.delegate.Put())
}

func (c impersonatingRESTClient) Patch(pt types.PatchType) *restclient.Request {
	return c.impersonate(c.delegate.Patch(pt))
}

func (c impersonatingRESTClient) Get() *restclient.Request {
	return c.impersonate(c.delegate.Get())
}

func (c impersonatingRESTClient) Delete() *restclient.Request {
	return c.impersonate(c.delegate.Delete())
}

func (c impersonatingRESTClient) APIVersion() schema.GroupVersion {
	return c.delegate.APIVersion()
}

func (c impersonatingRESTClient) GetRateLimiter() flowcontrol.RateLimiter {
	return c.delegate.GetRateLimiter()
}
