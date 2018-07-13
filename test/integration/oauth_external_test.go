package integration

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/RangelReale/osin"

	kauthn "k8s.io/api/authentication/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	"github.com/openshift/origin/pkg/oc/lib/tokencmd"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestWebhookTokenAuthn checks Tokens directly against an external
// authenticator
func TestOauthExternal(t *testing.T) {
	authToken := "BoringToken"
	authTestUser := "user"
	authTestUID := "42"
	authTestGroups := []string{"testgroup"}

	expectedTokenPost := kauthn.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1beta1",
			Kind:       "TokenReview",
		},
		Spec: kauthn.TokenReviewSpec{Token: authToken},
	}

	tokenResponse := kauthn.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1beta1",
			Kind:       "TokenReview",
		},
		Status: kauthn.TokenReviewStatus{
			Authenticated: true,
			User: kauthn.UserInfo{
				Username: authTestUser,
				UID:      authTestUID,
				Groups:   authTestGroups,
				Extra: map[string]kauthn.ExtraValue{
					authorizationapi.ScopesKey: []string{
						"user:info",
					},
				},
			},
		},
	}

	// Write cert we're going to use to verify auth server requests
	caFile, err := ioutil.TempFile("", "test.crt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(caFile.Name())
	if err := ioutil.WriteFile(caFile.Name(), authLocalhostCert, os.FileMode(0600)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var authServerURL string

	// Set up a dummy authenticator server
	authServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authenticate":
			if r.Method != "POST" {
				t.Fatalf("Expected POST to /authenticate, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("Error parsing form POSTed to /token: %v", err)
			}
			var tokenPost kauthn.TokenReview
			if err = json.NewDecoder(r.Body).Decode(&tokenPost); err != nil {
				t.Fatalf("Expected TokenReview structure in POST request: %v", err)
			}
			if !reflect.DeepEqual(tokenPost, expectedTokenPost) {
				t.Fatalf("Expected\n%#v\ngot\n%#v", expectedTokenPost, tokenPost)
			}
			if err = json.NewEncoder(w).Encode(tokenResponse); err != nil {
				t.Fatalf("Failed to encode Token Review response: %v", err)
			}

		case "/oauth/authorize":
			w.Header().Set("Location", fmt.Sprintf("%s/oauth/token/implicit?code=%s", authServerURL, authToken))
			w.WriteHeader(http.StatusFound)

		case "/oauth/token":
			w.Write([]byte(fmt.Sprintf(`{"access_token":%q, "token_type":"Bearer"}`, authToken)))
		default:
			t.Fatalf("Unexpected request: %v", r.URL.Path)
		}
	}))
	cert, err := tls.X509KeyPair(authLocalhostCert, authLocalhostKey)
	authServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	authServer.StartTLS()
	defer authServer.Close()
	authServerURL = authServer.URL

	authConfigFile, err := ioutil.TempFile("", "test.cfg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(authConfigFile.Name())
	authConfigObj := kclientcmdapi.Config{
		Clusters: map[string]*kclientcmdapi.Cluster{
			"authService": {
				CertificateAuthority: caFile.Name(),
				Server:               authServer.URL + "/authenticate",
			},
		},
		AuthInfos: map[string]*kclientcmdapi.AuthInfo{
			"apiServer": {
				ClientCertificateData: authLocalhostCert,
				ClientKeyData:         authLocalhostKey,
			},
		},
		CurrentContext: "webhook",
		Contexts: map[string]*kclientcmdapi.Context{
			"webhook": {
				Cluster:  "authService",
				AuthInfo: "apiServer",
			},
		},
	}
	if err := kclientcmd.WriteToFile(authConfigObj, authConfigFile.Name()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authServerMetadataFile, err := ioutil.TempFile("", "metadata.cfg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(authServerMetadataFile.Name())
	authServerMetadata := oauthutil.OauthAuthorizationServerMetadata{
		Issuer:                        authServer.URL,
		AuthorizationEndpoint:         authServer.URL + "/oauth/authorize",
		TokenEndpoint:                 authServer.URL + "/oauth/token",
		ResponseTypesSupported:        osin.AllowedAuthorizeType{osin.CODE, osin.TOKEN},
		GrantTypesSupported:           osin.AllowedAccessType{osin.AUTHORIZATION_CODE, "implicit"},
		CodeChallengeMethodsSupported: []string{"plain", "S256"},
	}
	authServerMetadataSerialized, _ := json.MarshalIndent(authServerMetadata, "", "  ")
	authServerMetadataFile.Write(authServerMetadataSerialized)
	authServerMetadataFile.Sync()
	authServerMetadataFile.Close()

	// Get master config
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterOptions)

	masterOptions.OAuthConfig = nil
	masterOptions.AuthConfig.OAuthMetadataFile = authServerMetadataFile.Name()
	masterOptions.AuthConfig.WebhookTokenAuthenticators = []configapi.WebhookTokenAuthenticator{
		{
			ConfigFile: authConfigFile.Name(),
			CacheTTL:   "10s",
		},
	}

	// Start server
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	anonymousConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	//The client needs to connect to both servers, so trust both certs
	anonymousConfig.TLSClientConfig.CAData = append(anonymousConfig.TLSClientConfig.CAData, authLocalhostCert...)

	accessToken, err := tokencmd.RequestToken(anonymousConfig, bytes.NewBufferString("user\npass"), "", "")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if accessToken != authToken {
		t.Errorf("Expected accessToken=%q, got %q", authToken, accessToken)
	}

	clientConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	clientConfig.BearerToken = accessToken

	user, err := userclient.NewForConfigOrDie(clientConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if user.Name != "user" {
		t.Errorf("expected %v, got %v", "user", user.Name)
	}
}
