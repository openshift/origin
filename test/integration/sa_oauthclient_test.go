// +build integration

package integration

import (
	"crypto/tls"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/RangelReale/osincli"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/oauth/scope"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestSAAsOAuthClient(t *testing.T) {
	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authorizationCodes := make(chan string, 1)
	authorizationErrors := make(chan string, 1)
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Logf("fake pod server got %v", req.URL)

		if code := req.URL.Query().Get("code"); len(code) > 0 {
			authorizationCodes <- code
		}
		if err := req.URL.Query().Get("error"); len(err) > 0 {
			authorizationErrors <- err
		}
	}))
	defer oauthServer.Close()

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projectName := "hammer-project"
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, projectName, []string{"default"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// get the SA ready with redirect URIs and secret annotations
	defaultSA, err := clusterAdminKubeClient.ServiceAccounts(projectName).Get("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// retry this a couple times.  We seem to be flaking on update conflicts and missing secrets all together
	err = wait.PollImmediate(30*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		if defaultSA.Annotations == nil {
			defaultSA.Annotations = map[string]string{}
		}
		defaultSA.Annotations[saoauth.OAuthRedirectURISecretAnnotationPrefix+"one"] = oauthServer.URL
		defaultSA.Annotations[saoauth.OAuthWantChallengesAnnotationPrefix] = "true"
		defaultSA, err = clusterAdminKubeClient.ServiceAccounts(projectName).Update(defaultSA)
		if err != nil {
			t.Logf("unexpected err: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var oauthSecret *kapi.Secret
	// retry this a couple times.  We seem to be flaking on update conflicts and missing secrets all together
	err = wait.PollImmediate(30*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		allSecrets, err := clusterAdminKubeClient.Secrets(projectName).List(kapi.ListOptions{})
		if err != nil {
			return false, err
		}
		for i := range allSecrets.Items {
			secret := allSecrets.Items[i]
			if serviceaccount.IsServiceAccountToken(&secret, defaultSA) {
				oauthSecret = &secret
				return true, nil
			}
		}

		return false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oauthClientConfig := &osincli.ClientConfig{
		ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
		ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
		AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
		TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
		RedirectUrl:  oauthServer.URL,
		Scope:        scope.Join([]string{"user:info", "role:edit:" + projectName}),
		SendClientSecretInParams: true,
	}
	runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, authorizationCodes, authorizationErrors, true, true)
	clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)

	oauthClientConfig = &osincli.ClientConfig{
		ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
		ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
		AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
		TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
		RedirectUrl:  oauthServer.URL,
		Scope:        scope.Join([]string{"user:info", "role:edit:other-ns"}),
		SendClientSecretInParams: true,
	}
	runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, authorizationCodes, authorizationErrors, false, false)
	clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)

	oauthClientConfig = &osincli.ClientConfig{
		ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
		ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
		AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
		TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
		RedirectUrl:  oauthServer.URL,
		Scope:        scope.Join([]string{"user:info"}),
		SendClientSecretInParams: true,
	}
	runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, authorizationCodes, authorizationErrors, true, false)
	clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)
}

var grantCSRFRegex = regexp.MustCompile(`input type="hidden" name="csrf" value="([^"]*)"`)
var grantThenRegex = regexp.MustCompile(`input type="hidden" name="then" value="([^"]*)"`)

func runOAuthFlow(t *testing.T, clusterAdminClientConfig *restclient.Config, projectName string, oauthClientConfig *osincli.ClientConfig, authorizationCodes, authorizationErrors chan string, expectGrantSuccess, expectBuildSuccess bool) {
	oauthRuntimeClient, err := osincli.NewClient(oauthClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oauthRuntimeClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	directHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// make sure we're prompted for a password
	authorizeRequest := oauthRuntimeClient.NewAuthorizeRequest(osincli.CODE)
	authorizeURL := authorizeRequest.GetAuthorizeUrlWithParams("opaque-scope")
	authorizeHTTPRequest, err := http.NewRequest("GET", authorizeURL.String(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	authorizeHTTPRequest.Header.Add("X-CSRF-Token", "csrf-01")
	authorizeResponse, err := directHTTPClient.Do(authorizeHTTPRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authorizeResponse.StatusCode != http.StatusUnauthorized {
		response, _ := httputil.DumpResponse(authorizeResponse, true)
		t.Fatalf("didn't get an unauthorized:\n %v", string(response))
	}

	// first we should get a redirect to a grant flow
	authenticatedAuthorizeHTTPRequest1, err := http.NewRequest("GET", authorizeURL.String(), nil)
	authenticatedAuthorizeHTTPRequest1.Header.Add("X-CSRF-Token", "csrf-01")
	authenticatedAuthorizeHTTPRequest1.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("harold:any-pass")))
	authentictedAuthorizeResponse1, err := directHTTPClient.Transport.RoundTrip(authenticatedAuthorizeHTTPRequest1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authentictedAuthorizeResponse1.StatusCode != http.StatusFound {
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse1, true)
		t.Fatalf("unexpected status :\n %v", string(response))
	}

	// second we get a webpage with a prompt.  Yeah, this next bit gets nasty
	authenticatedAuthorizeHTTPRequest2, err := http.NewRequest("GET", clusterAdminClientConfig.Host+authentictedAuthorizeResponse1.Header.Get("Location"), nil)
	authenticatedAuthorizeHTTPRequest2.Header.Add("X-CSRF-Token", "csrf-01")
	authenticatedAuthorizeHTTPRequest2.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("harold:any-pass")))
	authentictedAuthorizeResponse2, err := directHTTPClient.Transport.RoundTrip(authenticatedAuthorizeHTTPRequest2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authentictedAuthorizeResponse2.StatusCode != http.StatusOK {
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse2, true)
		t.Fatalf("unexpected status :\n %v", string(response))
	}
	// have to parse the page to get the csrf value.  Yeah, that's nasty, I can't think of another way to do it without creating a new grant handler
	body, err := ioutil.ReadAll(authentictedAuthorizeResponse2.Body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expectGrantSuccess {
		if !strings.Contains(string(body), "requested illegal scopes") {
			t.Fatalf("missing expected message: %v", string(body))
		}
		return
	}
	csrfMatches := grantCSRFRegex.FindStringSubmatch(string(body))
	if len(csrfMatches) != 2 {
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse2, false)
		t.Fatalf("unexpected body :\n %v\n%v", string(response), string(body))
	}
	thenMatches := grantThenRegex.FindStringSubmatch(string(body))
	if len(thenMatches) != 2 {
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse2, false)
		t.Fatalf("unexpected body :\n %v\n%v", string(response), string(body))
	}
	t.Logf("CSRF is %v", csrfMatches)
	t.Logf("then is %v", thenMatches)

	// third we respond and approve the grant, then let the transport follow redirects and give us the code
	postBody := strings.NewReader(url.Values(map[string][]string{
		"then":         {thenMatches[1]},
		"csrf":         {csrfMatches[1]},
		"client_id":    {oauthClientConfig.ClientId},
		"user_name":    {"harold"},
		"scopes":       {oauthClientConfig.Scope},
		"redirect_uri": {clusterAdminClientConfig.Host},
		"approve":      {"true"},
	}).Encode())
	authenticatedAuthorizeHTTPRequest3, err := http.NewRequest("POST", clusterAdminClientConfig.Host+origin.OpenShiftApprovePrefix, postBody)
	authenticatedAuthorizeHTTPRequest3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authenticatedAuthorizeHTTPRequest3.Header.Add("X-CSRF-Token", csrfMatches[1])
	authenticatedAuthorizeHTTPRequest3.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("harold:any-pass")))
	for i := range authentictedAuthorizeResponse2.Cookies() {
		cookie := authentictedAuthorizeResponse2.Cookies()[i]
		authenticatedAuthorizeHTTPRequest3.AddCookie(cookie)
	}
	authentictedAuthorizeResponse3, err := directHTTPClient.Transport.RoundTrip(authenticatedAuthorizeHTTPRequest3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authentictedAuthorizeResponse3.StatusCode != http.StatusFound {
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse3, true)
		t.Fatalf("unexpected status :\n %v", string(response))
	}

	// fourth, the grant redirects us again to have us send the code to the server
	authenticatedAuthorizeHTTPRequest4, err := http.NewRequest("GET", clusterAdminClientConfig.Host+authentictedAuthorizeResponse3.Header.Get("Location"), nil)
	authenticatedAuthorizeHTTPRequest4.Header.Add("X-CSRF-Token", "csrf-01")
	authenticatedAuthorizeHTTPRequest4.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("harold:any-pass")))
	authentictedAuthorizeResponse4, err := directHTTPClient.Transport.RoundTrip(authenticatedAuthorizeHTTPRequest4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authentictedAuthorizeResponse4.StatusCode != http.StatusFound {
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse4, true)
		t.Fatalf("unexpected status :\n %v", string(response))
	}

	authenticatedAuthorizeHTTPRequest5, err := http.NewRequest("GET", authentictedAuthorizeResponse4.Header.Get("Location"), nil)
	authenticatedAuthorizeHTTPRequest5.Header.Add("X-CSRF-Token", "csrf-01")
	authenticatedAuthorizeHTTPRequest5.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("harold:any-pass")))
	authentictedAuthorizeResponse5, err := directHTTPClient.Do(authenticatedAuthorizeHTTPRequest5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authorizationCode := ""
	select {
	case authorizationCode = <-authorizationCodes:
	case <-time.After(10 * time.Second):
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse5, true)
		t.Fatalf("didn't get a code:\n %v", string(response))
	}

	accessRequest := oauthRuntimeClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, &osincli.AuthorizeData{Code: authorizationCode})
	accessData, err := accessRequest.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	whoamiConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	whoamiConfig.BearerToken = accessData.AccessToken
	whoamiClient, err := client.New(&whoamiConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := whoamiClient.Builds(projectName).List(kapi.ListOptions{}); !kapierrors.IsForbidden(err) && !expectBuildSuccess {
		t.Fatalf("unexpected error: %v", err)
	}

	user, err := whoamiClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "harold" {
		t.Fatalf("expected %v, got %v", "harold", user.Name)
	}
}
