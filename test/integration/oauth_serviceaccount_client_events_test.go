package integration

import (
	"golang.org/x/net/html"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"reflect"
	"testing"
	"time"

	"github.com/RangelReale/osincli"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/serviceaccount"

	oauthapiv1 "github.com/openshift/api/oauth/v1"
	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	"github.com/openshift/origin/pkg/oauth/scope"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	testutil "github.com/openshift/origin/test/util"
	htmlutil "github.com/openshift/origin/test/util/html"
	testserver "github.com/openshift/origin/test/util/server"
)

type testServer struct {
	clusterAdminKubeClient   kclientset.Interface
	clusterAdminClientConfig *restclient.Config
	clusterAdminOAuthClient  *oauthclient.Clientset
	authCodes                chan string
	authErrors               chan string
	oauthServer              *httptest.Server
	masterConfig             *config.MasterConfig
}

var (
	adminUser   = "harold"
	saName      = "default"
	projectName = "test-project"
)

// TestOAuthServiceAccountClientEvent verifies that certain warning events are created when an SA is incorrectly configured
// for OAuth
func TestOAuthServiceAccountClientEvent(t *testing.T) {

	tests := map[string]struct {
		annotationPrefix    string
		annotation          string
		expectedEventReason string
		expectedEventMsg    string
		numEvents           int
		expectBadRequest    bool
	}{
		"test-good-url": {
			annotationPrefix: saoauth.OAuthRedirectModelAnnotationURIPrefix + "one",
			annotation:       "/oauthcallback",
			numEvents:        0,
		},
		"test-bad-url": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationURIPrefix + "one",
			annotation:          "foo:foo",
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    "system:serviceaccount:" + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>",
			numEvents:           1,
			expectBadRequest:    true,
		},
		"test-bad-url-parse": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationURIPrefix + "one",
			annotation:          "::",
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    "[parse ::: missing protocol scheme, system:serviceaccount:" + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>]",
			numEvents:           1,
			expectBadRequest:    true,
		},
		"test-bad-redirect-annotation-kind": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationReferencePrefix + "1",
			annotation:          `{"kind":"foo","apiVersion":"oauth.openshift.io/v1","metadata":{"creationTimestamp":null},"reference":{"group":"foo","kind":"Route","name":"route1"}}`,
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    `[no kind "foo" is registered for version "oauth.openshift.io/v1", system:serviceaccount:` + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>]",
			numEvents:           1,
			expectBadRequest:    true,
		},
		"test-bad-redirect-type-parse": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationReferencePrefix + "1",
			annotation:          `{asdf":"adsf"}`,
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    `[couldn't get version/kind; json parse error: invalid character 'a' looking for beginning of object key string, system:serviceaccount:` + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>]",
			numEvents:           1,
			expectBadRequest:    true,
		},
		"test-bad-redirect-route-not-found": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationReferencePrefix + "1",
			annotation:          buildRedirectObjectReferenceString(t, "Route", "route1", "route.openshift.io"),
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    `[routes.route.openshift.io "route1" not found, system:serviceaccount:` + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>]",
			numEvents:           1,
			expectBadRequest:    true,
		},
		"test-bad-redirect-route-wrong-group": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationReferencePrefix + "1",
			annotation:          buildRedirectObjectReferenceString(t, "Route", "route1", "foo"),
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    `system:serviceaccount:` + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>",
			numEvents:           1,
			expectBadRequest:    true,
		},
		"test-bad-redirect-reference-kind": {
			annotationPrefix:    saoauth.OAuthRedirectModelAnnotationReferencePrefix + "1",
			annotation:          buildRedirectObjectReferenceString(t, "foo", "route1", "route.openshift.io"),
			expectedEventReason: "NoSAOAuthRedirectURIs",
			expectedEventMsg:    `system:serviceaccount:` + projectName + ":" + saName + " has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>=<redirect> or create a dynamic URI using serviceaccounts.openshift.io/oauth-redirectreference.<some-value>=<reference>",
			numEvents:           1,
			expectBadRequest:    true,
		},
	}

	testServer, err := setupTestOAuthServer()
	if err != nil {
		t.Fatalf("error setting up test server: %s", err)
	}

	defer testServer.oauthServer.Close()
	defer testserver.CleanupMasterEtcd(t, testServer.masterConfig)

	for tcName, testCase := range tests {
		var redirect string = testServer.oauthServer.URL + "/oauthcallback"
		if testCase.numEvents != 0 {
			redirect = testCase.annotation
		}

		t.Logf("%s: annotationPrefix %s, annotation %s", tcName, testCase.annotationPrefix, testCase.annotation)
		sa, err := setupTestSA(testServer.clusterAdminKubeClient, testCase.annotationPrefix, redirect)
		if err != nil {
			t.Fatalf("%s: error setting up test SA: %s", tcName, err)
		}

		secret, err := setupTestSecrets(testServer.clusterAdminKubeClient, sa)
		if err != nil {
			t.Fatalf("%s: error setting up test secrets: %s", tcName, err)
		}

		runTestOAuthFlow(t, testServer, sa, secret, redirect, testCase.expectBadRequest)

		// Check events
		evList, err := testServer.clusterAdminKubeClient.Core().Events(projectName).List(metav1.ListOptions{})
		if err != nil {
			t.Fatalf("%s: err listing events", tcName)
		}

		events := collectEventsWithReason(evList, testCase.expectedEventReason)

		if testCase.numEvents != len(events) {
			t.Fatalf("%s: expected %d events, found %d", tcName, testCase.numEvents, len(events))
		}

		if testCase.numEvents != 0 && events[0].Message != testCase.expectedEventMsg {
			t.Fatalf("%s: expected event message %s, got %s", tcName, testCase.expectedEventMsg, events[0].Message)
		}

		err = testServer.clusterAdminKubeClient.Core().Events(projectName).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("%s: error deleting events: %s", tcName, err)
		}
	}
}

func collectEventsWithReason(eventList *kapi.EventList, reason string) []kapi.Event {
	var events []kapi.Event
	for _, ev := range eventList.Items {
		if ev.Reason != reason {
			continue
		}
		events = append(events, ev)
	}
	return events
}

func buildRedirectObjectReferenceString(t *testing.T, kind, name, group string) string {
	ref := &oauthapiv1.OAuthRedirectReference{
		Reference: oauthapiv1.RedirectReference{
			Kind:  kind,
			Name:  name,
			Group: group,
		},
	}
	data, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(oauthapiv1.SchemeGroupVersion), ref)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	return string(data)
}

func setupTestOAuthServer() (*testServer, error) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		return nil, err
	}

	authorizationCodes := make(chan string, 1)
	authorizationErrors := make(chan string, 1)
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if code := req.URL.Query().Get("code"); len(code) > 0 {
			authorizationCodes <- code
		}
		if err := req.URL.Query().Get("error"); len(err) > 0 {
			authorizationErrors <- err
		}
	}))

	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		return nil, err
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		return nil, err
	}
	clusterAdminOAuthClient := oauthclient.NewForConfigOrDie(clusterAdminClientConfig)
	if err != nil {
		return nil, err
	}

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, adminUser); err != nil {
		return nil, err
	}
	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, projectName, []string{saName}); err != nil {
		return nil, err
	}

	return &testServer{
		clusterAdminKubeClient:   clusterAdminKubeClientset,
		clusterAdminClientConfig: clusterAdminClientConfig,
		clusterAdminOAuthClient:  clusterAdminOAuthClient,
		authCodes:                authorizationCodes,
		authErrors:               authorizationErrors,
		oauthServer:              oauthServer,
		masterConfig:             masterConfig,
	}, nil
}

func setupTestSA(client kclientset.Interface, annotationPrefix, annotation string) (*kapi.ServiceAccount, error) {
	var serviceAccount *kapi.ServiceAccount

	// retry this a couple times.  We seem to be flaking on update conflicts and missing secrets all together
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		serviceAccount, err = client.Core().ServiceAccounts(projectName).Get(saName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Each test needs a fresh set of annotations, so override the previous ones.
		serviceAccount.Annotations = map[string]string{}

		serviceAccount.Annotations[annotationPrefix] = annotation
		serviceAccount.Annotations[saoauth.OAuthWantChallengesAnnotationPrefix] = "true"
		serviceAccount, err = client.Core().ServiceAccounts(projectName).Update(serviceAccount)
		return err
	})
	if err != nil {
		return nil, err
	}

	return serviceAccount, nil
}

func setupTestSecrets(client kclientset.Interface, sa *kapi.ServiceAccount) (*kapi.Secret, error) {
	var oauthSecret *kapi.Secret
	// retry this a couple times.  We seem to be flaking on update conflicts and missing secrets all together
	err := wait.PollImmediate(30*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		allSecrets, err := client.Core().Secrets(projectName).List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for i := range allSecrets.Items {
			secret := &allSecrets.Items[i]

			if serviceaccount.InternalIsServiceAccountToken(secret, sa) {
				oauthSecret = secret
				return true, nil
			}
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return oauthSecret, nil
}

// Run through a standard OAuth sequence for a single test. The tests vary in modifications to the SA annotations so
// the specific sequence does not matter, as long as it can generate the server_error that we expect.
func runTestOAuthFlow(t *testing.T, ts *testServer, sa *kapi.ServiceAccount, secret *kapi.Secret, redirectURL string, expectBadRequest bool) {
	oauthClientConfig := &osincli.ClientConfig{
		ClientId:     apiserverserviceaccount.MakeUsername(sa.Namespace, sa.Name),
		ClientSecret: string(secret.Data[kapi.ServiceAccountTokenKey]),
		AuthorizeUrl: ts.clusterAdminClientConfig.Host + "/oauth/authorize",
		TokenUrl:     ts.clusterAdminClientConfig.Host + "/oauth/token",
		RedirectUrl:  redirectURL,
		Scope:        scope.Join([]string{"user:info", "role:edit:" + projectName}),
		SendClientSecretInParams: true,
	}

	doOAuthFlow(t, ts.clusterAdminClientConfig, oauthClientConfig, ts.authCodes, ts.authErrors, expectBadRequest, []string{
		"GET /oauth/authorize",
		"received challenge",
		"GET /oauth/authorize",
		"redirect to /oauth/authorize/approve",
		"form",
		"POST /oauth/authorize/approve",
		"redirect to /oauth/authorize",
		"redirect to /oauthcallback",
		"code",
	})

	ts.clusterAdminOAuthClient.Oauth().OAuthClientAuthorizations().Delete(adminUser+":"+oauthClientConfig.ClientId, nil)
}

func doOAuthFlow(
	t *testing.T,
	clusterAdminClientConfig *restclient.Config,
	oauthClientConfig *osincli.ClientConfig,
	authorizationCodes chan string,
	authorizationErrors chan string,
	expectBadRequest bool,
	expectOperations []string,
) {
	drain(authorizationCodes)
	drain(authorizationErrors)

	oauthRuntimeClient, err := osincli.NewClient(oauthClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clientTransport, err := restclient.TransportFor(clusterAdminClientConfig)
	testTransport := &basicAuthTransport{rt: clientTransport}
	oauthRuntimeClient.Transport = testTransport

	authorizeRequest := oauthRuntimeClient.NewAuthorizeRequest(osincli.CODE)
	req, err := http.NewRequest("GET", authorizeRequest.GetAuthorizeUrlWithParams("").String(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set up the HTTP redirect handler
	operations := []string{}
	jar, _ := cookiejar.New(nil)
	directHTTPClient := &http.Client{
		Transport: testTransport,
		CheckRedirect: func(redirectReq *http.Request, via []*http.Request) error {
			t.Logf("302 Location: %s", redirectReq.URL.String())
			req = redirectReq
			operations = append(operations, "redirect to "+redirectReq.URL.Path)
			return nil
		},
		Jar: jar,
	}

	for {
		t.Logf("%s %s", req.Method, req.URL.String())
		operations = append(operations, req.Method+" "+req.URL.Path)

		// Always set the csrf header
		req.Header.Set("X-CSRF-Token", "1")
		resp, err := directHTTPClient.Do(req)
		if err != nil {
			t.Errorf("Error %v\n%#v\n%#v", operations, jar, err)
			return
		}
		defer resp.Body.Close()

		// Save the current URL for reference
		currentURL := req.URL

		if resp.StatusCode == 401 {
			// Set up a username and password once we're challenged
			testTransport.username = adminUser
			testTransport.password = "any-pass"
			operations = append(operations, "received challenge")
			continue
		}

		if expectBadRequest && resp.StatusCode == 400 {
			responseDump, _ := httputil.DumpResponse(resp, true)
			t.Logf("Bad Request: %s", string(responseDump))
			return
		}

		if resp.StatusCode != 200 {
			responseDump, _ := httputil.DumpResponse(resp, true)
			t.Errorf("Expected status code 200, got %v and response: %s", resp.StatusCode, string(responseDump))
			return
		}

		doc, err := html.Parse(resp.Body)
		if err != nil {
			responseDump, _ := httputil.DumpResponse(resp, true)
			t.Errorf("Error parsing response body: %s", string(responseDump))
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
		req, err = htmlutil.NewRequestFromForm(forms[0], currentURL, nil)
		if err != nil {
			t.Errorf("Error creating form response: %s", err)
			return
		}
		operations = append(operations, "form")
	}

	select {
	case <-authorizationCodes:
		operations = append(operations, "code")
	case authorizationError := <-authorizationErrors:
		operations = append(operations, "error:"+authorizationError)
	case <-time.After(5 * time.Second):
		t.Error("didn't get a code or an error")
	}

	if !reflect.DeepEqual(operations, expectOperations) {
		t.Errorf("Expected:\n%#v\nGot\n%#v", expectOperations, operations)
	}
}
