package integration

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"
	restclient "k8s.io/client-go/rest"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestOAuthServerHeaders tests that the Oauth Server pages that return
// browser relevant stuff (HTML) are served with appropriate headers
func TestOAuthServerHeaders(t *testing.T) {
	// Set up server
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer testserver.CleanupMasterEtcd(t, masterOptions)

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	anonConfig := restclient.AnonymousClientConfig(clientConfig)
	transport, err := restclient.TransportFor(anonConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	baseURL, err := url.Parse(clientConfig.Host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hit the login URL
	loginURL := *baseURL
	loginURL.Path = "/login"
	checkNewReqHeaders(t, transport, loginURL.String())

	// Hit the grant URL
	grantURL := *baseURL
	grantURL.Path = "/oauth/authorize/approve"
	checkNewReqHeaders(t, transport, grantURL.String())

}

func checkNewReqHeaders(t *testing.T, rt http.RoundTripper, checkUrl string) {
	req, err := http.NewRequest("GET", checkUrl, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	req.Header.Set("Accept", "text/html; charset=utf-8")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	allHeaders := http.Header{}
	for key, val := range map[string]string{
		// security related headers that we really care about, should not change
		"Cache-Control":          "no-cache, no-store",
		"Pragma":                 "no-cache",
		"Expires":                "0",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-DNS-Prefetch-Control": "off",
		"X-XSS-Protection":       "1; mode=block",

		// non-security headers, should not change
		// adding items here should be validated to make sure they do not conflict with any security headers
		"Content-Type": "text/html; charset=utf-8",
	} {
		// use set so we get the canonical form of these headers
		allHeaders.Set(key, val)
	}

	// these headers can change per request and are not important to us
	// only add items to this list if they cannot be statically checked above
	ignoredHeaders := []string{"Date", "Content-Length", "Location"}
	for _, h := range ignoredHeaders {
		resp.Header.Del(h)
	}

	if !reflect.DeepEqual(allHeaders, resp.Header) {
		t.Errorf("Header for %s does not match: expected: %#v got: %#v diff: %s",
			checkUrl, allHeaders, resp.Header, diff.ObjectDiff(allHeaders, resp.Header))
	}
}
