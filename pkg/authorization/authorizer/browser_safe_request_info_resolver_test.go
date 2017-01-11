package authorizer

import (
	"net/http"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apiserver/request"
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
		resolver := &request.RequestInfoFactory{
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
		RequestInfo request.RequestInfo
		Context     kapi.Context
		Host        string
		Headers     http.Header

		ExpectedVerb        string
		ExpectedSubresource string
	}{
		"non-resource": {
			RequestInfo:  request.RequestInfo{IsResourceRequest: false, Verb: "GET"},
			ExpectedVerb: "GET",
		},

		"non-proxy": {
			RequestInfo:         request.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "logs"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "logs",
		},

		"unsafe proxy subresource": {
			RequestInfo:         request.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "unsafeproxy",
		},
		"unsafe proxy verb": {
			RequestInfo:  request.RequestInfo{IsResourceRequest: true, Verb: "proxy", Resource: "nodes"},
			ExpectedVerb: "unsafeproxy",
		},
		"unsafe proxy verb anonymous": {
			Context:      kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "system:anonymous", Groups: []string{"system:unauthenticated"}}),
			RequestInfo:  request.RequestInfo{IsResourceRequest: true, Verb: "proxy", Resource: "nodes"},
			ExpectedVerb: "unsafeproxy",
		},

		"proxy subresource authenticated": {
			Context:             kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "bob", Groups: []string{"system:authenticated"}}),
			RequestInfo:         request.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
			ExpectedVerb:        "get",
			ExpectedSubresource: "proxy",
		},
		"proxy subresource custom header": {
			RequestInfo:         request.RequestInfo{IsResourceRequest: true, Verb: "get", Resource: "pods", Subresource: "proxy"},
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
	context kapi.Context
}

func (t *testContextMapper) Get(req *http.Request) (kapi.Context, bool) {
	return t.context, t.context != nil
}
func (t *testContextMapper) Update(req *http.Request, ctx kapi.Context) error {
	return nil
}

type testInfoFactory struct {
	info *request.RequestInfo
}

func (t *testInfoFactory) NewRequestInfo(req *http.Request) (*request.RequestInfo, error) {
	return t.info, nil
}
