package integration

import (
	"net/http"
	"net/url"
	"testing"

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

func checkNewReqHeaders(t *testing.T, rt http.RoundTripper, check_url string) {
	req, err := http.NewRequest("GET", check_url, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	req.Header.Set("Accept", "text/html; charset=UTF-8")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	checkImportantHeaders := map[string]string{
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-DNS-Prefetch-Control": "off",
		"X-XSS-Protection":       "1; mode=block",
	}

	for key, val := range checkImportantHeaders {
		header := resp.Header.Get(key)
		if header != val {
			t.Errorf("While probing %s expected header %s: %s, got {%v}", check_url, key, val, header)
		}
	}
}
