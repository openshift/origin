package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"golang.org/x/net/html"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestOAuthRequestTokenEndpoint tests obtaining and using a bearer token from the request and display token endpoints.
func TestOAuthRequestTokenEndpoint(t *testing.T) {
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

	// Hit the token request endpoint
	masterURL, err := url.Parse(clientConfig.Host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterURL.Path = "/oauth/token/request"

	_, tokenHeaderLocation := checkNewReqAndRoundTrip(t, transport, masterURL.String(), false, http.StatusFound)

	if len(tokenHeaderLocation) == 0 {
		t.Fatalf("no Location header")
	}

	authRedirect, err := url.Parse(tokenHeaderLocation)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	_, authHeaderLocation := checkNewReqAndRoundTrip(t, transport, authRedirect.String(), true, http.StatusFound)

	if len(authHeaderLocation) == 0 {
		t.Fatalf("no Location header")
	}

	displayResp, _ := checkNewReqAndRoundTrip(t, transport, authHeaderLocation, false, http.StatusOK)
	apiToken := getTokenFromDisplay(t, displayResp)

	// Verify use of the bearer token
	userConfig := restclient.AnonymousClientConfig(clientConfig)
	userConfig.BearerToken = apiToken
	userClient, err := userclient.NewForConfig(userConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	user, err := userClient.Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "foo" {
		t.Fatalf("expected foo as the user, got %v", user.Name)
	}
}

// Parse the HTML body for the API token contained in the first <code></code> block
func getTokenFromDisplay(t *testing.T, body []byte) string {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))

	var seenCode bool
	for tokenType := tokenizer.Next(); tokenType != html.ErrorToken; tokenType = tokenizer.Next() {
		token := tokenizer.Token()
		if tokenType == html.StartTagToken && token.Data == "code" {
			seenCode = true
		}
		if seenCode && tokenType == html.TextToken {
			return token.Data
		}
	}

	t.Fatalf("API Token not found in display")
	return ""
}

func checkNewReqAndRoundTrip(t *testing.T, rt http.RoundTripper, url string, doBasicAuth bool, expectedCode int) ([]byte, string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	req.Header.Set("Accept", "text/html; charset=UTF-8")

	if doBasicAuth {
		req.SetBasicAuth("foo", "bar")
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedCode {
		t.Fatalf("unexpected response code %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	return body, resp.Header.Get("Location")
}
