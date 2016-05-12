// +build integration

package integration

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/RangelReale/osincli"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/client"
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
	if defaultSA.Annotations == nil {
		defaultSA.Annotations = map[string]string{}
	}
	defaultSA.Annotations[saoauth.OAuthRedirectURISecretAnnotationPrefix+"one"] = oauthServer.URL
	defaultSA.Annotations[saoauth.OAuthWantChallengesAnnotationPrefix] = "true"
	defaultSA, err = clusterAdminKubeClient.ServiceAccounts(projectName).Update(defaultSA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allSecrets, err := clusterAdminKubeClient.Secrets(projectName).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var oauthSecret *kapi.Secret
	for i := range allSecrets.Items {
		secret := allSecrets.Items[i]
		if serviceaccount.IsServiceAccountToken(&secret, defaultSA) {
			secret.Annotations[saoauth.OAuthClientSecretAnnotation] = "true"
			if _, err := clusterAdminKubeClient.Secrets(projectName).Update(&secret); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oauthSecret = &secret
			break
		}
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
	runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, authorizationCodes, authorizationErrors, false, true)

	oauthClientConfig = &osincli.ClientConfig{
		ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
		ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
		AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
		TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
		RedirectUrl:  oauthServer.URL,
		Scope:        scope.Join([]string{"user:info", "role:edit:other-ns"}),
		SendClientSecretInParams: true,
	}
	runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, authorizationCodes, authorizationErrors, true, false)

	oauthClientConfig = &osincli.ClientConfig{
		ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
		ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
		AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
		TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
		RedirectUrl:  oauthServer.URL,
		Scope:        scope.Join([]string{"user:info"}),
		SendClientSecretInParams: true,
	}
	runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, authorizationCodes, authorizationErrors, false, false)

}

func runOAuthFlow(t *testing.T, clusterAdminClientConfig *restclient.Config, projectName string, oauthClientConfig *osincli.ClientConfig, authorizationCodes, authorizationErrors chan string, expectAuthCodeError, expectBuildSuccess bool) {
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

	authenticatedAuthorizeHTTPRequest, err := http.NewRequest("GET", authorizeURL.String(), nil)
	authenticatedAuthorizeHTTPRequest.Header.Add("X-CSRF-Token", "csrf-01")
	authenticatedAuthorizeHTTPRequest.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("harold:any-pass")))
	authentictedAuthorizeResponse, err := directHTTPClient.Do(authenticatedAuthorizeHTTPRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authorizationCode := ""
	select {
	case authorizationCode = <-authorizationCodes:
		if expectAuthCodeError {
			response, _ := httputil.DumpResponse(authentictedAuthorizeResponse, true)
			t.Fatalf("got a code:\n %v", string(response))
		}
	case <-authorizationErrors:
		if !expectAuthCodeError {
			response, _ := httputil.DumpResponse(authentictedAuthorizeResponse, true)
			t.Fatalf("unexpected error:\n %v", string(response))
		}
		return
	case <-time.After(10 * time.Second):
		response, _ := httputil.DumpResponse(authentictedAuthorizeResponse, true)
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
