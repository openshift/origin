package integration

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/client/restclient"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/login"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestOAuthOIDC checks CLI password login against an OIDC provider
func TestOAuthOIDC(t *testing.T) {

	expectedTokenPost := url.Values{
		"grant_type":    []string{"password"},
		"client_id":     []string{"myclient"},
		"client_secret": []string{"mysecret"},
		"username":      []string{"mylogin"},
		"password":      []string{"mypassword"},
		"scope":         []string{"openid scope1 scope2"},
	}

	// id_token made at https://jwt.io/
	// {
	// 	"sub": "mysub",
	// 	"name": "John Doe",
	// 	"myidclaim": "myid",
	// 	"myemailclaim":"myemail",
	// }
	tokenResponse := `{
		"token_type":   "bearer",
		"access_token": "12345",
		"id_token":     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJteXN1YiIsIm5hbWUiOiJKb2huIERvZSIsIm15aWRjbGFpbSI6Im15aWQiLCJteWVtYWlsY2xhaW0iOiJteWVtYWlsIn0.yMx2ZQw8Su641H_kO8ec_tFaysrFEc9uFUFm4ZbLGHw"
	}`

	// Additional claims in userInfo (sub claim must match)
	userinfoResponse := `{
		"sub": "mysub",
	 	"mynameclaim":"myname",
	 	"myusernameclaim":"myusername"
	}`

	// Write cert we're going to use to verify OIDC server requests
	caFile, err := ioutil.TempFile("", "test.crt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(caFile.Name())
	if err := ioutil.WriteFile(caFile.Name(), oidcLocalhostCert, os.FileMode(0600)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get master config
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set up a dummy OIDC server
	oidcServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.String() {
		case "/token":
			if r.Method != "POST" {
				t.Fatalf("Expected POST to /token, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("Error parsing form POSTed to /token: %v", err)
			}
			if !reflect.DeepEqual(r.PostForm, expectedTokenPost) {
				t.Fatalf("Expected\n%#v\ngot\n%#v", expectedTokenPost, r.PostForm)
			}

			w.Write([]byte(tokenResponse))

		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer 12345" {
				t.Fatalf("Expected authorization header, got %#v", r.Header)
			}
			w.Write([]byte(userinfoResponse))

		default:
			t.Fatalf("Unexpected OIDC request: %v", r.URL.String())
		}
	}))
	cert, err := tls.X509KeyPair(oidcLocalhostCert, oidcLocalhostKey)
	oidcServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	oidcServer.StartTLS()
	defer oidcServer.Close()

	masterOptions.OAuthConfig.IdentityProviders[0] = configapi.IdentityProvider{
		Name:            "oidc",
		UseAsChallenger: true,
		UseAsLogin:      true,
		MappingMethod:   "claim",
		Provider: &configapi.OpenIDIdentityProvider{
			CA:           caFile.Name(),
			ClientID:     "myclient",
			ClientSecret: configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Value: "mysecret"}},
			ExtraScopes:  []string{"scope1", "scope2"},
			URLs: configapi.OpenIDURLs{
				Authorize: oidcServer.URL + "/authorize",
				Token:     oidcServer.URL + "/token",
				UserInfo:  oidcServer.URL + "/userinfo",
			},
			Claims: configapi.OpenIDClaims{
				ID:                []string{"myidclaim"},
				Email:             []string{"myemailclaim"},
				Name:              []string{"mynameclaim"},
				PreferredUsername: []string{"myusernameclaim"},
			},
		},
	}

	// Start server
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the master CA data for the login command
	masterCAFile := clientConfig.CAFile
	if masterCAFile == "" {
		// Write master ca data
		tmpFile, err := ioutil.TempFile("", "ca.crt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		if err := ioutil.WriteFile(tmpFile.Name(), clientConfig.CAData, os.FileMode(0600)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		masterCAFile = tmpFile.Name()
	}

	// Attempt a login using a redirecting auth proxy
	loginOutput := &bytes.Buffer{}
	loginOptions := &login.LoginOptions{
		Server:             clientConfig.Host,
		CAFile:             masterCAFile,
		StartingKubeConfig: &clientcmdapi.Config{},
		Reader:             bytes.NewBufferString("mylogin\nmypassword\n"),
		Out:                loginOutput,
	}
	if err := loginOptions.GatherInfo(); err != nil {
		t.Fatalf("Error logging in: %v\n%v", err, loginOutput.String())
	}
	if loginOptions.Username != "myusername" {
		t.Fatalf("Unexpected user after authentication: %#v", loginOptions)
	}
	if len(loginOptions.Config.BearerToken) == 0 {
		t.Fatalf("Expected token after authentication: %#v", loginOptions.Config)
	}

	// Ex
	userConfig := &restclient.Config{
		Host: clientConfig.Host,
		TLSClientConfig: restclient.TLSClientConfig{
			CAFile: clientConfig.CAFile,
			CAData: clientConfig.CAData,
		},
		BearerToken: loginOptions.Config.BearerToken,
	}
	userClient, err := client.New(userConfig)
	userWhoamiOptions := cmd.WhoAmIOptions{UserInterface: userClient.Users(), Out: ioutil.Discard}
	retrievedUser, err := userWhoamiOptions.WhoAmI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrievedUser.Name != "myusername" {
		t.Errorf("expected username %v, got %v", "myusername", retrievedUser.Name)
	}
	if retrievedUser.FullName != "myname" {
		t.Errorf("expected display name %v, got %v", "myname", retrievedUser.FullName)
	}
	if !reflect.DeepEqual([]string{"oidc:myid"}, retrievedUser.Identities) {
		t.Errorf("expected only oidc:myid identity, got %v", retrievedUser.Identities)
	}
}

// oidcLocalhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at Jan 29 16:00:00 2084 GMT.
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var oidcLocalhostCert = []byte(`-----BEGIN CERTIFICATE-----
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

// oidcLocalhostKey is the private key for oidcLocalhostCert.
var oidcLocalhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
