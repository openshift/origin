package integration

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
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

	_, tokenHeaderLocation := checkNewReqAndRoundTrip(t, transport, http.MethodGet, masterURL.String(), nil, nil, false, http.StatusFound)

	if len(tokenHeaderLocation) == 0 {
		t.Fatalf("no Location header")
	}

	authRedirect, err := url.Parse(tokenHeaderLocation)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	_, authHeaderLocation := checkNewReqAndRoundTrip(t, transport, http.MethodGet, authRedirect.String(), nil, nil, true, http.StatusFound)

	if len(authHeaderLocation) == 0 {
		t.Fatalf("no Location header")
	}

	displayRespGet, _ := checkNewReqAndRoundTrip(t, transport, http.MethodGet, authHeaderLocation, nil, nil, false, http.StatusOK)

	code, csrf, loc := getCodeCSRFFromDisplay(t, displayRespGet)
	masterURL.Path = loc
	values := url.Values{}
	values.Set("code", code)
	values.Set("csrf", csrf)
	headers := map[string]string{"content-type": "application/x-www-form-urlencoded"}

	// no csrf cookie fails
	b1, _ := checkNewReqAndRoundTrip(t, transport, http.MethodPost, masterURL.String(), headers, strings.NewReader(values.Encode()), false, http.StatusBadRequest)
	assertBodyContains(t, b1, "Could not check CSRF token. Please try again.")

	// empty csrf cookie fails
	headers["cookie"] = "csrf="
	b2, _ := checkNewReqAndRoundTrip(t, transport, http.MethodPost, masterURL.String(), headers, strings.NewReader(values.Encode()), false, http.StatusBadRequest)
	assertBodyContains(t, b2, "Could not check CSRF token. Please try again.")

	// wrong csrf cookie fails
	headers["cookie"] = "csrf=123"
	b3, _ := checkNewReqAndRoundTrip(t, transport, http.MethodPost, masterURL.String(), headers, strings.NewReader(values.Encode()), false, http.StatusBadRequest)
	assertBodyContains(t, b3, "Could not check CSRF token. Please try again.")

	// technically any matching csrf gets past the check (do not send code as it is one use only)
	b4, _ := checkNewReqAndRoundTrip(t, transport, http.MethodPost, masterURL.String(), headers, strings.NewReader(headers["cookie"]), false, http.StatusBadRequest)
	assertBodyContains(t, b4, "Error handling auth request: Requested parameter not sent")

	// correct csrf cookie works
	headers["cookie"] = "csrf=" + csrf
	displayRespPost, _ := checkNewReqAndRoundTrip(t, transport, http.MethodPost, masterURL.String(), headers, strings.NewReader(values.Encode()), false, http.StatusOK)

	apiToken := getTokenFromDisplay(t, displayRespPost)

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

func assertBodyContains(t *testing.T, body []byte, sub string) {
	t.Helper()

	if !bytes.Contains(body, []byte(sub)) {
		t.Fatalf("request body %s missing data %s", string(body), sub)
	}
}

func getCodeCSRFFromDisplay(t *testing.T, body []byte) (code, csrf, loc string) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))

	var codeSeen, csrfSeen, locSeen bool

	for tokenType := tokenizer.Next(); tokenType != html.ErrorToken; tokenType = tokenizer.Next() {
		if token := tokenizer.Token(); tokenType == html.StartTagToken {
			if token.Data == "form" && getVal("method", token.Attr) == "post" {
				if locSeen {
					t.Fatalf("loc seen more than once in body %s, first loc=%s", string(body), loc)
				}
				loc = getVal("action", token.Attr)
				locSeen = true
			}
			if token.Data == "input" && getVal("name", token.Attr) == "code" {
				if codeSeen {
					t.Fatalf("code seen more than once in body %s, first code=%s", string(body), code)
				}
				code = getVal("value", token.Attr)
				codeSeen = true
			}
			if token.Data == "input" && getVal("name", token.Attr) == "csrf" {
				if csrfSeen {
					t.Fatalf("csrf seen more than once in body %s, first csrf=%s", string(body), csrf)
				}
				csrf = getVal("value", token.Attr)
				csrfSeen = true
			}
		}
	}

	if len(code) == 0 || len(csrf) == 0 || len(loc) == 0 {
		t.Fatalf("could not find code or csrf or loc in body %s, saw: code=%s, csrf=%s, loc=%s", string(body), code, csrf, loc)
	}

	return code, csrf, loc
}

func getVal(key string, attrs []html.Attribute) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
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

	t.Fatalf("API Token not found in display, body %s", string(body))
	return ""
}

func checkNewReqAndRoundTrip(t *testing.T, rt http.RoundTripper, method, url string, headers map[string]string, reqBody io.Reader, doBasicAuth bool, expectedCode int) ([]byte, string) {
	t.Helper()

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	req.Header.Set("Accept", "text/html; charset=UTF-8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if doBasicAuth {
		req.SetBasicAuth("foo", "bar")
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if resp.StatusCode != expectedCode {
		t.Fatalf("unexpected response code %v, body=%s", resp.StatusCode, string(body))
	}

	return body, resp.Header.Get("Location")
}
