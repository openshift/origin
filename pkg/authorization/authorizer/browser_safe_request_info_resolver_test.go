package authorizer

import (
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func TestUpstreamInfoResolver(t *testing.T) {
	subresourceRequest, _ := http.NewRequest("GET", "/api/v1/namespaces/myns/pods/mypod/proxy", nil)
	proxyRequest, _ := http.NewRequest("GET", "/api/v1/proxy/nodes/mynode", nil)

	testcases := map[string]struct {
		Request             *http.Request
		ExpectedVerb        string
		ExpectedSubresource string
	}{
		"unsafe proxy subresource": {
			Request:             subresourceRequest,
			ExpectedVerb:        "get",
			ExpectedSubresource: "proxy", // should be "unsafeproxy" or similar once check moves upstream
		},
		"unsafe proxy verb": {
			Request:      proxyRequest,
			ExpectedVerb: "proxy", // should be "unsafeproxy" or similar once check moves upstream
		},
	}

	for k, tc := range testcases {
		resolver := &apirequest.RequestInfoFactory{
			APIPrefixes:          sets.NewString("api", "osapi", "oapi", "apis"),
			GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi"),
		}

		info, err := resolver.NewRequestInfo(tc.Request)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		if info.Verb != tc.ExpectedVerb {
			t.Errorf("%s: expected verb %s, got %s. If request.RequestInfoFactory now adjusts attributes for proxy safety, investigate removing the NewBrowserSafeRequestInfoResolver wrapper.", k, tc.ExpectedVerb, info.Verb)
		}
		if info.Subresource != tc.ExpectedSubresource {
			t.Errorf("%s: expected verb %s, got %s. If request.RequestInfoFactory now adjusts attributes for proxy safety, investigate removing the NewBrowserSafeRequestInfoResolver wrapper.", k, tc.ExpectedSubresource, info.Subresource)
		}
	}
}

func TestBrowserSafeRequestInfoResolver(t *testing.T) {
	testcases := map[string]struct {
		RequestInfo apirequest.RequestInfo
		Context     apirequest.Context
		Host        string
		Headers     http.Header

		ExpectedVerb        string
		ExpectedSubresource string
	}{
		"non-resource": {
			RequestInfo:  apirequest.RequestInfo{IsResourceRequest: false, Verb: "GET"},
			ExpectedVerb: "GET",
		},

		"non-proxy": {
			RequestInfo:         apirequest.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "logs"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "logs",
		},

		"unsafe proxy subresource": {
			RequestInfo:         apirequest.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "unsafeproxy",
		},
		"unsafe proxy verb": {
			RequestInfo:  apirequest.RequestInfo{IsResourceRequest: true, Verb: "proxy", Resource: "nodes"},
			ExpectedVerb: "unsafeproxy",
		},
		"unsafe proxy verb anonymous": {
			Context:      apirequest.WithUser(apirequest.NewContext(), &user.DefaultInfo{Name: "system:anonymous", Groups: []string{"system:unauthenticated"}}),
			RequestInfo:  apirequest.RequestInfo{IsResourceRequest: true, Verb: "proxy", Resource: "nodes"},
			ExpectedVerb: "unsafeproxy",
		},

		"proxy subresource authenticated": {
			Context:             apirequest.WithUser(apirequest.NewContext(), &user.DefaultInfo{Name: "bob", Groups: []string{"system:authenticated"}}),
			RequestInfo:         apirequest.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "proxy",
		},
		"proxy subresource custom header": {
			RequestInfo:         apirequest.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			Headers:             http.Header{"X-Csrf-Token": []string{"1"}},
			ExpectedVerb:        "get",
			ExpectedSubresource: "proxy",
		},
	}

	for k, tc := range testcases {
		resolver := NewBrowserSafeRequestInfoResolver(
			&testContextMapper{tc.Context},
			sets.NewString("system:authenticated"),
			&testInfoFactory{&tc.RequestInfo},
		)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Host = tc.Host
		req.Header = tc.Headers

		info, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		if info.Verb != tc.ExpectedVerb {
			t.Errorf("%s: expected verb %s, got %s", k, tc.ExpectedVerb, info.Verb)
		}
		if info.Subresource != tc.ExpectedSubresource {
			t.Errorf("%s: expected verb %s, got %s", k, tc.ExpectedSubresource, info.Subresource)
		}
	}
}

type testContextMapper struct {
	context apirequest.Context
}

func (t *testContextMapper) Get(req *http.Request) (apirequest.Context, bool) {
	return t.context, t.context != nil
}
func (t *testContextMapper) Update(req *http.Request, ctx apirequest.Context) error {
	return nil
}

type testInfoFactory struct {
	info *apirequest.RequestInfo
}

func (t *testInfoFactory) NewRequestInfo(req *http.Request) (*apirequest.RequestInfo, error) {
	return t.info, nil
}
