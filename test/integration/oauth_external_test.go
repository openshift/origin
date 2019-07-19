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

	kauthn "k8s.io/api/authentication/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
	"github.com/openshift/oc/pkg/helpers/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
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
					authorizationv1.ScopesKey: []string{
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
	authServerMetadata := oauthdiscovery.OauthAuthorizationServerMetadata{
		Issuer:                        authServer.URL,
		AuthorizationEndpoint:         authServer.URL + "/oauth/authorize",
		TokenEndpoint:                 authServer.URL + "/oauth/token",
		ResponseTypesSupported:        []string{"code", "token"},
		GrantTypesSupported:           []string{"authorization_code", "implicit"},
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

	user, err := userv1client.NewForConfigOrDie(clientConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if user.Name != "user" {
		t.Errorf("expected %v, got %v", "user", user.Name)
	}
}

// authLocalhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at Jan 29 16:00:00 2084 GMT.
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var authLocalhostCert = []byte(`-----BEGIN CERTIFICATE-----	
MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zANBgkqhkiG9w0BAQsFADAS	
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw	
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB	
iQKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9SjY1bIw4	
iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZBl2+XsDul	
rKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQABo2gwZjAO	
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw	
AwEB/zAuBgNVHREEJzAlggtleGFtcGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAA	
AAAAATANBgkqhkiG9w0BAQsFAAOBgQCEcetwO59EWk7WiJsG4x8SY+UIAA+flUI9	
tyC4lNhbcF2Idq9greZwbYCqTTTr2XiRNSMLCOjKyI7ukPoPjo16ocHj+P3vZGfs	
h1fIw3cSS2OolhloGw/XM6RWPWtPAlGykKLciQrBru5NAPvCMsb/I1DAceTiotQM	
fblo6RBxUQ==	
-----END CERTIFICATE-----`)

// authLocalhostKey is the private key for authLocalhostCert.
var authLocalhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----	
MIICXgIBAAKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9	
SjY1bIw4iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZB	
l2+XsDulrKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQAB	
AoGAGRzwwir7XvBOAy5tM/uV6e+Zf6anZzus1s1Y1ClbjbE6HXbnWWF/wbZGOpet	
3Zm4vD6MXc7jpTLryzTQIvVdfQbRc6+MUVeLKwZatTXtdZrhu+Jk7hx0nTPy8Jcb	
uJqFk541aEw+mMogY/xEcfbWd6IOkp+4xqjlFLBEDytgbIECQQDvH/E6nk+hgN4H	
qzzVtxxr397vWrjrIgPbJpQvBsafG7b0dA4AFjwVbFLmQcj2PprIMmPcQrooz8vp	
jy4SHEg1AkEA/v13/5M47K9vCxmb8QeD/asydfsgS5TeuNi8DoUBEmiSJwma7FXY	
fFUtxuvL7XvjwjN5B30pNEbc6Iuyt7y4MQJBAIt21su4b3sjXNueLKH85Q+phy2U	
fQtuUE9txblTu14q3N7gHRZB4ZMhFYyDy8CKrN2cPg/Fvyt0Xlp/DoCzjA0CQQDU	
y2ptGsuSmgUtWj3NM9xuwYPm+Z/F84K6+ARYiZ6PYj013sovGKUFfYAqVXVlxtIX	
qyUBnu3X9ps8ZfjLZO7BAkEAlT4R5Yl6cGhaJQYZHOde3JEMhNRcVFMO8dJDaFeo	
f9Oeos0UUothgiDktdQHxdNEwLjQf7lJJBzV+5OtwswCWA==	
-----END RSA PRIVATE KEY-----`)
