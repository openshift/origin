// +build integration,etcd

package integration

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"

	kclient "k8s.io/kubernetes/pkg/client"
	clientcmdapi "k8s.io/kubernetes/pkg/client/clientcmd/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var (
	rootCACert = []byte(`-----BEGIN CERTIFICATE-----
MIIC7DCCAdagAwIBAgIBATALBgkqhkiG9w0BAQswKDEmMCQGA1UEAwwdMTAuMTMu
MTI5LjE0OTo4NDQzQDE0MjU2NzUyNzQwIBcNMTUwMzA2MjA1NDM0WhgPMjExNTAy
MTAyMDU0MzVaMCgxJjAkBgNVBAMMHTEwLjEzLjEyOS4xNDk6ODQ0M0AxNDI1Njc1
Mjc0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuOnLZ0PgeEKnbV7D
93g6fcllMh6ngCnQpEoaWSHTWjPbv/qDU/jRQU2l/KHOkMXKsbNiasRT6ZIWlUFc
W/Jgd1Tz7zjh+pgJHLEtKdWVPwP/8ruUhQotrb1E/q1g21wqczPxfb+Z9s6+AnkF
FLooBCCRa8wpC+TtcAaT7/yEJfN6IUhcT9XFmLzKTPz76UXBHMN+KDeK0k0u77a9
vj+eAedB6Xg9lfpvIclvjgy6cvQ9oavYTJ8Q5mYZdIdspmSzFjAyZUylgpEIpPkN
e8dcqiA0hc2Mq/pwwn/F3i4va/NO7+Od9gRkAtvuvCUASXuCmon6pRYAZEImevRt
GbRlkQIDAQABoyMwITAOBgNVHQ8BAf8EBAMCAKQwDwYDVR0TAQH/BAUwAwEB/zAL
BgkqhkiG9w0BAQsDggEBAD30//8aJkPLtJ0G6/0oa+zjKBZH04PyWCjTsgaDCHVm
z/AntWxKR5fc+z/NXfnhV8M8/zb4ZGHp+jczozvcXZxUgftlUFNxV7sY8NXdJNrs
t+oFURLIibIjxN0vlz7py16RxXy693t6PzfQB/69ZB/AI3VfyOdJ1cvaV/kOce21
Kp/jmVz5DUhQI60zcUOE4at81emo3uYK7Pz9iil2Wu2lK4+1uP4LdZRRLEUXWqNb
VmAB7OAhfJ2/x/BsPIvbI1aGp7DjjQgaeBwXD/mW8AUJHHbdvWUYz1yNyQ2XDWZm
X2kxcf0iGTwuqufTmw7EcDc/dWIdJ6bsB007/M9bz3g=
-----END CERTIFICATE-----
`)

	clientCert = []byte(`-----BEGIN CERTIFICATE-----
MIIC+DCCAeKgAwIBAgIBBTALBgkqhkiG9w0BAQswKDEmMCQGA1UEAwwdMTAuMTMu
MTI5LjE0OTo4NDQzQDE0MjU2NzUyNzQwIBcNMTUwMzA2MjA1NDM2WhgPMjExNTAy
MTAyMDU0MzdaMCIxIDAeBgNVBAMTF3N5c3RlbTpvcGVuc2hpZnQtY2xpZW50MIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwbkwwrV4j3xqmUhyKErAzfAI
UX5atGGJHt+oRmZ3BzeAl6CpGLGLSiYso4j5JmLo0qpvQroSw66oOoVMw2851nhI
OZHo7aGvJ9elgmwa7ghDg4DN3TUe8y9Ex+JUDnAAK5dY0DV8UK7Aa2SAxIlSMGIu
VuUcjwlC9w37D3VxDFoa6XO+SUBiRUJyjiDlLNUegyV60jimxVTZbTb8r90lbGc8
iB5j6py5ZCF/UMRY5LEuIum/7dKvH2A03q2n3Y58qcAhWIp6lP9DeJe3CMvuK64C
BwvT9jm9TioRGZskqfV3mLYyhxp1q2FKK03umQ5KKNYqvppFUYVKdzXgFjxfMQID
AQABozUwMzAOBgNVHQ8BAf8EBAMCAKAwEwYDVR0lBAwwCgYIKwYBBQUHAwIwDAYD
VR0TAQH/BAIwADALBgkqhkiG9w0BAQsDggEBACY2Lu5kl9a2cBi3+WB/oo7iG82S
9IdIaABDLFWCGp4e0PEGAfROlNcCrhmEizkhPXeDNrjmDKeShnu1E/8RgwBtDrym
v9WQBa/HI3ZbO3hDdR2pNo6c+y3MqDJHO8/4l7hV5DYY9ypfB85mZQ7uxKaFawqs
GqLZNJWjpG6T9yUDaj2fO+etXb5dPTZiSytw1Z4l2GDGElLRjVS4k2aP0Lo/BLXG
1nUatU3KcCEeb1ifghFjDLESo8mUwfBl9v1vO75rIFRDfoPFMqIHGhRmP+fbnCI8
fWh90BcEhK0TheTyEHBPtpKKiYz5BWyNlkCuTmhygaRCa9SQWl2nRi2XVGY=
-----END CERTIFICATE-----
`)

	clientKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAwbkwwrV4j3xqmUhyKErAzfAIUX5atGGJHt+oRmZ3BzeAl6Cp
GLGLSiYso4j5JmLo0qpvQroSw66oOoVMw2851nhIOZHo7aGvJ9elgmwa7ghDg4DN
3TUe8y9Ex+JUDnAAK5dY0DV8UK7Aa2SAxIlSMGIuVuUcjwlC9w37D3VxDFoa6XO+
SUBiRUJyjiDlLNUegyV60jimxVTZbTb8r90lbGc8iB5j6py5ZCF/UMRY5LEuIum/
7dKvH2A03q2n3Y58qcAhWIp6lP9DeJe3CMvuK64CBwvT9jm9TioRGZskqfV3mLYy
hxp1q2FKK03umQ5KKNYqvppFUYVKdzXgFjxfMQIDAQABAoIBADg/olXWxUO8V2Nc
crEaS3NAT9oBuyqG636IaF7Qn5z7052zK4Yc/xmvjeSJ//XSYFHS5O1WA97Hltcv
H0PbxspsMGRu5lghSy9hYRBGfWdCBQBo5N1m8C6iOfFj2Q48HQCLOGF0Nj1jEEHe
c7kdOj0MNPJMIgeyI7yCVbR+YC26dfxiaIfRtyzsScsNX/pP1AH9lEd9c6reMvms
UxjplUkYjk4gbngmKJjd2MD8dc8XqR5V+Oq1uwOQG/EZhSBlyxwRVMFwd5/opb2Q
JqMZvd458MQ2C2RSZALXDYYmCMbXU76Crg7N3+y34d+uwkIufUhFfTR7BuUkvlzt
5vb+WTECgYEAyVCSxvTbB9y3IQpsRlJeBwVnWNHclZft4g9PtqQgA6VaWqSoYvFz
t2m2/L3O39zEgalM6HesVT8EdiIWvp08eHYvFTb9jaqxHIbwdrzxtvBn5SJ6CjCX
xA+uWv3AbH+H2t/ZCiPAKebcOfefmce6/8cKrNKFXs5KojR6tpM2Lo0CgYEA9li3
JDTRbCGLsuyVhDT3l4przzJC0DwU1j8zBJfUrBtuPMhDISHHvq9opSoTQsRXz9EH
ruQe3/XCvE/y988E3oVh+ikmBX01xCsIUB0jUhItVQ7GSacY0+UgZmH6Zw7xJjO5
zwaYGnejOxHIs0XmeajbhYl+bAGEym+iV682rDUCgYEAn0ox2VtFNCNgg7RLmBj0
bXnJHG5xq6xbfdO/rzSOYFQl+jLvSdrjRO1Q7QsC9f8pPa9IO2j14z3Jue+fL5Qa
lPZuqsqoNcAqA/iBrHI0kBwJGTT+e7GXZHtD6pt99luyk20rvuoq0vzopLVag8OW
I2zK9ZReE3YHd/EuZ+hzpsECgYBiTXSHliwboidE9vOTFi/W4P20aLIQtmj6Na3+
HzhWlXuf9aoUBo7WoNh5UBjvg7omy5rtR0qqxD85Ng4WpR2kTkWStejeN+DErwda
MMZvcaF1V7f4nB1kMQKE2IQ7q9K/E9UJr+/yX9tbLvWP1EzsL12qI/u2zcRXo8R8
iQagIQKBgQDA4Ag/ShDEb0x6rFFJwMaHUT/TT8Yv8Ul7paIcAX6c/1jwgEh9T20F
6UWl+OcIpJp7DYNNLB/hesYqs76QvZDd9nIiW1bzuC0LHuSzyEoe234gpAfrRfYs
qLwYJxjzwYTLvLYPU5vHmdg8v5wIXh0TaRTDTdKViISGD09aiXSYzw==
-----END RSA PRIVATE KEY-----
`)
)

// TestOAuthRequestHeader checks the following scenarios:
//  * request containing remote user header is ignored if it doesn't have client cert auth
//  * request containing remote user header is honored if it has client cert auth
//  * unauthenticated requests are redirected to an auth proxy
//  * login command succeeds against a request-header identity provider via redirection to an auth proxy
func TestOAuthRequestHeader(t *testing.T) {
	// Test data used by auth proxy
	users := map[string]string{
		"myusername": "mypassword",
	}

	// Write cert we're going to use to verify OAuth requestheader requests
	caFile, err := ioutil.TempFile("", "test.crt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(caFile.Name())
	if err := ioutil.WriteFile(caFile.Name(), rootCACert, os.FileMode(0600)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get master config
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterURL, _ := url.Parse(masterOptions.OAuthConfig.MasterPublicURL)

	// Set up an auth proxy
	var proxyTransport http.RoundTripper
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decide whether to challenge
		username, password, hasBasicAuth := r.BasicAuth()
		if correctPassword, hasUser := users[username]; !hasBasicAuth || !hasUser || password != correctPassword {
			w.Header().Set("WWW-Authenticate", "Basic realm=Protected Area")
			w.WriteHeader(401)
			return
		}

		// Swap the scheme and host to the master, keeping path and params the same
		proxyURL := r.URL
		proxyURL.Scheme = masterURL.Scheme
		proxyURL.Host = masterURL.Host

		// Build a request, copying the original method, body, and headers, overriding the remote user headers
		proxyRequest, _ := http.NewRequest(r.Method, proxyURL.String(), r.Body)
		proxyRequest.Header = r.Header
		proxyRequest.Header.Set("My-Remote-User", username)
		proxyRequest.Header.Set("SSO-User", "")

		// Round trip to the back end
		response, err := proxyTransport.RoundTrip(r)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		defer response.Body.Close()

		// Copy response back to originator
		for k, v := range response.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(response.StatusCode)
		if _, err := io.Copy(w, response.Body); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}))
	defer proxyServer.Close()

	masterOptions.OAuthConfig.IdentityProviders[0] = configapi.IdentityProvider{
		Name:            "requestheader",
		UseAsChallenger: true,
		UseAsLogin:      true,
		Provider: runtime.EmbeddedObject{
			Object: &configapi.RequestHeaderIdentityProvider{
				ChallengeURL: proxyServer.URL + "/oauth/authorize?${query}",
				LoginURL:     "http://www.example.com/login?then=${url}",
				ClientCA:     caFile.Name(),
				Headers:      []string{"My-Remote-User", "SSO-User"},
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

	// Use the server and CA info, but no client cert info
	anonConfig := kclient.Config{}
	anonConfig.Host = clientConfig.Host
	anonConfig.CAFile = clientConfig.CAFile
	anonConfig.CAData = clientConfig.CAData
	anonTransport, err := kclient.TransportFor(&anonConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Use the server and CA info, with cert info
	proxyConfig := anonConfig
	proxyConfig.CertData = clientCert
	proxyConfig.KeyData = clientKey
	proxyTransport, err = kclient.TransportFor(&proxyConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build the authorize request, spoofing a remote user header
	authorizeURL := clientConfig.Host + "/oauth/authorize?client_id=openshift-challenging-client&response_type=token"
	req, err := http.NewRequest("GET", authorizeURL, nil)
	req.Header.Set("My-Remote-User", "myuser")

	// Make the request without cert auth
	resp, err := anonTransport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	proxyRedirect, err := resp.Location()
	if err != nil {
		t.Fatalf("expected spoofed remote user header to get 302 redirect, got error: %v", err)
	}
	if proxyRedirect.String() != proxyServer.URL+"/oauth/authorize?client_id=openshift-challenging-client&response_type=token" {
		t.Fatalf("expected redirect to proxy endpoint, got redirected to %v", proxyRedirect.String())
	}

	// Request the redirected URL, which should cause the proxy to make the same request with cert auth
	req, err = http.NewRequest("GET", proxyRedirect.String(), nil)
	req.Header.Set("My-Remote-User", "myuser")
	req.SetBasicAuth("myusername", "mypassword")

	resp, err = proxyTransport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tokenRedirect, err := resp.Location()
	if err != nil {
		t.Fatalf("expected 302 redirect, got error: %v", err)
	}
	if tokenRedirect.Query().Get("error") != "" {
		t.Fatalf("expected successful token request, got error %v", tokenRedirect.String())
	}

	// Extract the access_token

	// group #0 is everything.                      #1                #2     #3
	accessTokenRedirectRegex := regexp.MustCompile(`(^|&)access_token=([^&]+)($|&)`)
	accessToken := ""
	if matches := accessTokenRedirectRegex.FindStringSubmatch(tokenRedirect.Fragment); matches != nil {
		accessToken = matches[2]
	}
	if accessToken == "" {
		t.Fatalf("Expected access token, got %s", tokenRedirect.String())
	}

	// Make sure we can use the token, and it represents who we expect
	userConfig := anonConfig
	userConfig.BearerToken = accessToken
	userClient, err := client.New(&userConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	user, err := userClient.Users().Get("~")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if user.Name != "myusername" {
		t.Fatalf("Expected myusername as the user, got %v", user)
	}

	// Get the master CA data for the login command
	masterCAFile := userConfig.CAFile
	if masterCAFile == "" {
		// Write master ca data
		tmpFile, err := ioutil.TempFile("", "ca.crt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		if err := ioutil.WriteFile(tmpFile.Name(), userConfig.CAData, os.FileMode(0600)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		masterCAFile = tmpFile.Name()
	}

	// Attempt a login using a redirecting auth proxy
	loginOutput := &bytes.Buffer{}
	loginOptions := &cmd.LoginOptions{
		Server:             anonConfig.Host,
		CAFile:             masterCAFile,
		StartingKubeConfig: &clientcmdapi.Config{},
		Reader:             bytes.NewBufferString("myusername\nmypassword\n"),
		Out:                loginOutput,
	}
	if err := loginOptions.GatherInfo(); err != nil {
		t.Fatalf("Error trying to determine server info: %v\n%v", err, loginOutput.String())
	}
	if loginOptions.Username != "myusername" {
		t.Fatalf("Unexpected user after authentication: %#v", loginOptions)
	}
	if len(loginOptions.Config.BearerToken) == 0 {
		t.Fatalf("Expected token after authentication: %#v", loginOptions.Config)
	}
}
