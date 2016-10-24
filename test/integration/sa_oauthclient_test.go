package integration

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/scope"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	testutil "github.com/openshift/origin/test/util"
	htmlutil "github.com/openshift/origin/test/util/html"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestSAAsOAuthClient(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
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
	redirectURL := oauthServer.URL + "/oauthcallback"

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

	promptingClient, err := clusterAdminClient.OAuthClients().Create(&oauthapi.OAuthClient{
		ObjectMeta:            kapi.ObjectMeta{Name: "prompting-client"},
		Secret:                "prompting-client-secret",
		RedirectURIs:          []string{redirectURL},
		GrantMethod:           oauthapi.GrantHandlerPrompt,
		RespondWithChallenges: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// get the SA ready with redirect URIs and secret annotations
	var defaultSA *kapi.ServiceAccount

	// retry this a couple times.  We seem to be flaking on update conflicts and missing secrets all together
	err = kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		defaultSA, err = clusterAdminKubeClient.ServiceAccounts(projectName).Get("default")
		if err != nil {
			return err
		}
		if defaultSA.Annotations == nil {
			defaultSA.Annotations = map[string]string{}
		}
		defaultSA.Annotations[saoauth.OAuthRedirectModelAnnotationURIPrefix+"one"] = redirectURL
		defaultSA.Annotations[saoauth.OAuthWantChallengesAnnotationPrefix] = "true"
		defaultSA, err = clusterAdminKubeClient.ServiceAccounts(projectName).Update(defaultSA)
		return err
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

	// Test with a normal OAuth client
	{
		oauthClientConfig := &osincli.ClientConfig{
			ClientId:                 promptingClient.Name,
			ClientSecret:             promptingClient.Secret,
			AuthorizeUrl:             clusterAdminClientConfig.Host + "/oauth/authorize",
			TokenUrl:                 clusterAdminClientConfig.Host + "/oauth/token",
			RedirectUrl:              redirectURL,
			SendClientSecretInParams: true,
		}
		t.Log("Testing unrestricted scope")
		oauthClientConfig.Scope = ""
		// approval steps are needed for unscoped access
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, true, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauth/approve",
			"form",
			"POST /oauth/approve",
			"redirect to /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:user:full",
		})
		// verify the persisted client authorization looks like we expect
		if clientAuth, err := clusterAdminClient.OAuthClientAuthorizations().Get("harold:" + oauthClientConfig.ClientId); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		} else if !reflect.DeepEqual(clientAuth.Scopes, []string{"user:full"}) {
			t.Fatalf("Unexpected scopes: %v", clientAuth.Scopes)
		} else {
			// update the authorization to not contain any approved scopes
			clientAuth.Scopes = nil
			if _, err := clusterAdminClient.OAuthClientAuthorizations().Update(clientAuth); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		}
		// approval steps are needed again for unscoped access
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, true, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauth/approve",
			"form",
			"POST /oauth/approve",
			"redirect to /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:user:full",
		})
		// with the authorization stored, approval steps are skipped
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, true, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:user:full",
		})

		// Approval step is needed again
		t.Log("Testing restricted scope")
		oauthClientConfig.Scope = "user:info user:check-access"
		// filter to disapprove of granting the user:check-access scope
		deniedScope := false
		inputFilter := func(inputType, name, value string) bool {
			if inputType == "checkbox" && name == "scope" && value == "user:check-access" {
				deniedScope = true
				return false
			}
			return true
		}
		// our token only gets the approved one
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, inputFilter, authorizationCodes, authorizationErrors, true, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauth/approve",
			"form",
			"POST /oauth/approve",
			"redirect to /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:user:info",
		})
		if !deniedScope {
			t.Errorf("Expected form filter to deny user:info scope")
		}
		// second time, we approve all, and our token gets all requested scopes
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauth/approve",
			"form",
			"POST /oauth/approve",
			"redirect to /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:" + oauthClientConfig.Scope,
		})
		// third time, the approval steps is not needed, and the token gets all requested scopes
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:" + oauthClientConfig.Scope,
		})

		// Now request an unscoped token again, and no approval should be needed
		t.Log("Testing unrestricted scope")
		oauthClientConfig.Scope = ""
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, true, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:user:full",
		})

		clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)
	}

	{
		oauthClientConfig := &osincli.ClientConfig{
			ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
			ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
			AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
			TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
			RedirectUrl:  redirectURL,
			Scope:        scope.Join([]string{"user:info", "role:edit:" + projectName}),
			SendClientSecretInParams: true,
		}
		t.Log("Testing allowed scopes")
		// First time, the approval steps are needed
		// Second time, the approval steps are skipped
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, true, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauth/approve",
			"form",
			"POST /oauth/approve",
			"redirect to /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:" + oauthClientConfig.Scope,
		})
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, true, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:" + oauthClientConfig.Scope,
		})
		clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)
	}

	{
		oauthClientConfig := &osincli.ClientConfig{
			ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
			ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
			AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
			TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
			RedirectUrl:  redirectURL,
			Scope:        scope.Join([]string{"user:info", "role:edit:other-ns"}),
			SendClientSecretInParams: true,
		}
		t.Log("Testing disallowed scopes")
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, false, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"error:access_denied",
		})
		clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)
	}

	{
		t.Log("Testing invalid scopes")
		oauthClientConfig := &osincli.ClientConfig{
			ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
			ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
			AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
			TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
			RedirectUrl:  redirectURL,
			Scope:        scope.Join([]string{"unknown-scope"}),
			SendClientSecretInParams: true,
		}
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, false, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"error:invalid_scope",
		})
		clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)
	}

	{
		t.Log("Testing allowed scopes with failed API call")
		oauthClientConfig := &osincli.ClientConfig{
			ClientId:     serviceaccount.MakeUsername(defaultSA.Namespace, defaultSA.Name),
			ClientSecret: string(oauthSecret.Data[kapi.ServiceAccountTokenKey]),
			AuthorizeUrl: clusterAdminClientConfig.Host + "/oauth/authorize",
			TokenUrl:     clusterAdminClientConfig.Host + "/oauth/token",
			RedirectUrl:  redirectURL,
			Scope:        scope.Join([]string{"user:info"}),
			SendClientSecretInParams: true,
		}
		// First time, the approval is needed
		// Second time, the approval is skipped
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauth/approve",
			"form",
			"POST /oauth/approve",
			"redirect to /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:" + oauthClientConfig.Scope,
		})
		runOAuthFlow(t, clusterAdminClientConfig, projectName, oauthClientConfig, nil, authorizationCodes, authorizationErrors, true, false, []string{
			"GET /oauth/authorize",
			"received challenge",
			"GET /oauth/authorize",
			"redirect to /oauthcallback",
			"code",
			"scope:" + oauthClientConfig.Scope,
		})
		clusterAdminClient.OAuthClientAuthorizations().Delete("harold:" + oauthClientConfig.ClientId)
	}
}

func drain(ch chan string) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

type basicAuthTransport struct {
	rt       http.RoundTripper
	username string
	password string
}

func (b *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(b.username) > 0 || len(b.password) > 0 {
		req.SetBasicAuth(b.username, b.password)
	}
	return b.rt.RoundTrip(req)
}

func runOAuthFlow(
	t *testing.T,
	clusterAdminClientConfig *restclient.Config,
	projectName string,
	oauthClientConfig *osincli.ClientConfig,
	inputFilter htmlutil.InputFilterFunc,
	authorizationCodes chan string,
	authorizationErrors chan string,
	expectGrantSuccess bool,
	expectBuildSuccess bool,
	expectOperations []string,
) {
	drain(authorizationCodes)
	drain(authorizationErrors)

	oauthRuntimeClient, err := osincli.NewClient(oauthClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testTransport := &basicAuthTransport{rt: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	oauthRuntimeClient.Transport = testTransport

	authorizeRequest := oauthRuntimeClient.NewAuthorizeRequest(osincli.CODE)
	req, err := http.NewRequest("GET", authorizeRequest.GetAuthorizeUrlWithParams("opaque-state").String(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	operations := []string{}
	jar, _ := cookiejar.New(nil)
	directHTTPClient := &http.Client{
		Transport: testTransport,
		CheckRedirect: func(redirectReq *http.Request, via []*http.Request) error {
			glog.Infof("302 Location: %s", redirectReq.URL.String())
			req = redirectReq
			operations = append(operations, "redirect to "+redirectReq.URL.Path)
			return nil
		},
		Jar: jar,
	}

	for {
		glog.Infof("%s %s", req.Method, req.URL.String())
		operations = append(operations, req.Method+" "+req.URL.Path)

		// Always set the csrf header
		req.Header.Set("X-CSRF-Token", "1")
		resp, err := directHTTPClient.Do(req)
		if err != nil {
			glog.Infof("%#v", operations)
			glog.Infof("%#v", jar)
			glog.Errorf("Error %v\n%#v\n%#v", err, err, resp)
			t.Errorf("Error %v\n%#v\n%#v", err, err, resp)
			return
		}
		defer resp.Body.Close()

		// Save the current URL for reference
		currentURL := req.URL

		if resp.StatusCode == 401 {
			// Set up a username and password once we're challenged
			testTransport.username = "harold"
			testTransport.password = "any-pass"
			operations = append(operations, "received challenge")
			continue
		}

		if resp.StatusCode != 200 {
			responseDump, _ := httputil.DumpResponse(resp, true)
			t.Errorf("Unexpected response %s", string(responseDump))
			return
		}

		doc, err := html.Parse(resp.Body)
		if err != nil {
			t.Error(err)
			return
		}
		forms := htmlutil.GetElementsByTagName(doc, "form")
		// if there's a single form, submit it
		if len(forms) > 1 {
			t.Errorf("More than one form encountered: %d", len(forms))
			return
		}
		if len(forms) == 0 {
			break
		}
		req, err = htmlutil.NewRequestFromForm(forms[0], currentURL, inputFilter)
		if err != nil {
			t.Error(err)
			return
		}
		operations = append(operations, "form")
	}

	authorizationCode := ""
	select {
	case authorizationCode = <-authorizationCodes:
		operations = append(operations, "code")
	case authorizationError := <-authorizationErrors:
		operations = append(operations, "error:"+authorizationError)
	case <-time.After(5 * time.Second):
		t.Error("didn't get a code or an error")
	}

	if len(authorizationCode) > 0 {
		accessRequest := oauthRuntimeClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, &osincli.AuthorizeData{Code: authorizationCode})
		accessData, err := accessRequest.GetToken()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		operations = append(operations, fmt.Sprintf("scope:%v", accessData.ResponseData["scope"]))

		whoamiConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
		whoamiConfig.BearerToken = accessData.AccessToken
		whoamiClient, err := client.New(&whoamiConfig)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		_, err = whoamiClient.Builds(projectName).List(kapi.ListOptions{})
		if expectBuildSuccess && err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !expectBuildSuccess && !kapierrors.IsForbidden(err) {
			t.Errorf("expected forbidden error, got %v", err)
			return
		}

		user, err := whoamiClient.Users().Get("~")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if user.Name != "harold" {
			t.Errorf("expected %v, got %v", "harold", user.Name)
			return
		}
	}

	if !reflect.DeepEqual(operations, expectOperations) {
		t.Errorf("Expected:\n%#v\nGot\n%#v", expectOperations, operations)
	}

}
