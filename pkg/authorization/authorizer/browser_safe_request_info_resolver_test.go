package authorizer

import (
	"net/http"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiserver "k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"
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
		resolver := &kapiserver.RequestInfoResolver{
			APIPrefixes:          sets.NewString("api", "osapi", "oapi", "apis"),
			GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi"),
		}

		info, err := resolver.GetRequestInfo(tc.Request)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		if info.Verb != tc.ExpectedVerb {
			t.Errorf("%s: expected verb %s, got %s. If kapiserver.RequestInfoResolver now adjusts attributes for proxy safety, investigate removing the NewBrowserSafeRequestInfoResolver wrapper.", k, tc.ExpectedVerb, info.Verb)
		}
		if info.Subresource != tc.ExpectedSubresource {
			t.Errorf("%s: expected verb %s, got %s. If kapiserver.RequestInfoResolver now adjusts attributes for proxy safety, investigate removing the NewBrowserSafeRequestInfoResolver wrapper.", k, tc.ExpectedSubresource, info.Subresource)
		}
	}
}

func TestBrowserSafeRequestInfoResolver(t *testing.T) {
	testcases := map[string]struct {
		RequestInfo kapiserver.RequestInfo
		Context     kapi.Context
		Host        string
		Headers     http.Header

		ExpectedVerb        string
		ExpectedSubresource string
	}{
		"non-resource": {
			RequestInfo:  kapiserver.RequestInfo{IsResourceRequest: false, Verb: "GET"},
			ExpectedVerb: "GET",
		},

		"non-proxy": {
			RequestInfo:         kapiserver.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "logs"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "logs",
		},

		"unsafe proxy subresource": {
			RequestInfo:         kapiserver.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "unsafeproxy",
		},
		"unsafe proxy verb": {
			RequestInfo:  kapiserver.RequestInfo{IsResourceRequest: true, Verb: "proxy", Resource: "nodes"},
			ExpectedVerb: "unsafeproxy",
		},
		"unsafe proxy verb anonymous": {
			Context:      kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "system:anonymous", Groups: []string{"system:unauthenticated"}}),
			RequestInfo:  kapiserver.RequestInfo{IsResourceRequest: true, Verb: "proxy", Resource: "nodes"},
			ExpectedVerb: "unsafeproxy",
		},

		"proxy subresource authenticated": {
			Context:             kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "bob", Groups: []string{"system:authenticated"}}),
			RequestInfo:         kapiserver.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "proxy",
		},
		"proxy subresource custom header": {
			RequestInfo:         kapiserver.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			Headers:             http.Header{"X-Csrf-Token": []string{"1"}},
			ExpectedVerb:        "get",
			ExpectedSubresource: "proxy",
		},
	}

	for k, tc := range testcases {
		resolver := NewBrowserSafeRequestInfoResolver(
			&testContextMapper{tc.Context},
			sets.NewString("system:authenticated"),
			&testInfoResolver{tc.RequestInfo},
		)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Host = tc.Host
		req.Header = tc.Headers

		info, err := resolver.GetRequestInfo(req)
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
	context kapi.Context
}

func (t *testContextMapper) Get(req *http.Request) (kapi.Context, bool) {
	return t.context, t.context != nil
}
func (t *testContextMapper) Update(req *http.Request, ctx kapi.Context) error {
	return nil
}

type testInfoResolver struct {
	info kapiserver.RequestInfo
}

func (t *testInfoResolver) GetRequestInfo(req *http.Request) (kapiserver.RequestInfo, error) {
	return t.info, nil
}
