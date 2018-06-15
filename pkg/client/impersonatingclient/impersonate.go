package impersonatingclient

import (
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/util/flowcontrol"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

// NewImpersonatingConfig wraps the config's transport to impersonate a user, including user, groups, and scopes
func NewImpersonatingConfig(user user.Info, config restclient.Config) restclient.Config {
	oldWrapTransport := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		newConfig := transport.ImpersonationConfig{
			UserName: user.GetName(),
			Groups:   user.GetGroups(),
			Extra:    user.GetExtra(),
		}
		return transport.NewImpersonatingRoundTripper(newConfig, oldWrapTransport(rt))
	}
	return config
}

// NewImpersonatingKubernetesClientset returns a Kubernetes clientset that will impersonate a user, including user, groups, and scopes
func NewImpersonatingKubernetesClientset(user user.Info, config restclient.Config) (kclientset.Interface, error) {
	impersonatingConfig := NewImpersonatingConfig(user, config)
	return kclientset.NewForConfig(&impersonatingConfig)
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
	req.SetHeader(transport.ImpersonateUserHeader, c.user.GetName())
	req.SetHeader(transport.ImpersonateGroupHeader, c.user.GetGroups()...)
	for k, vv := range c.user.GetExtra() {
		req.SetHeader(transport.ImpersonateUserExtraHeaderPrefix+k, vv...)
	}
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
