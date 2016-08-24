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

	"k8s.io/kubernetes/pkg/client/restclient"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/login"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestOAuthRequestHeader checks the following scenarios:
//  * request containing remote user header is ignored if it doesn't have client cert auth
//  * request containing remote user header is honored if it has valid client cert auth matching ClientCommonNames
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
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
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
		MappingMethod:   "claim",
		Provider: &configapi.RequestHeaderIdentityProvider{
			ChallengeURL:      proxyServer.URL + "/oauth/authorize?${query}",
			LoginURL:          "http://www.example.com/login?then=${url}",
			ClientCA:          caFile.Name(),
			ClientCommonNames: []string{"proxy"},
			Headers:           []string{"My-Remote-User", "SSO-User"},
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
	anonConfig := restclient.Config{}
	anonConfig.Host = clientConfig.Host
	anonConfig.CAFile = clientConfig.CAFile
	anonConfig.CAData = clientConfig.CAData
	anonTransport, err := restclient.TransportFor(&anonConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Use the server and CA info, with cert info
	proxyConfig := anonConfig
	proxyConfig.CertData = proxyClientCert
	proxyConfig.KeyData = proxyClientKey
	proxyTransport, err = restclient.TransportFor(&proxyConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// client cert that is valid, but not in the list of allowed common names
	otherCertConfig := anonConfig
	otherCertConfig.CertData = otherClientCert
	otherCertConfig.KeyData = otherClientKey
	otherCertTransport, err := restclient.TransportFor(&otherCertConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// client cert that has the desired common name, but does not have a valid signature
	invalidCertConfig := anonConfig
	invalidCertConfig.CertData = invalidClientCert
	invalidCertConfig.KeyData = invalidClientKey
	invalidCertTransport, err := restclient.TransportFor(&invalidCertConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authorizeURL := clientConfig.Host + "/oauth/authorize?client_id=openshift-challenging-client&response_type=token"
	proxyURL := proxyServer.URL + "/oauth/authorize?client_id=openshift-challenging-client&response_type=token"

	testcases := map[string]struct {
		transport                http.RoundTripper
		expectDirectRequestError bool
	}{
		"anonymous": {
			transport:                anonTransport,
			expectDirectRequestError: false,
		},
		"valid signature, invalid cn": {
			transport: otherCertTransport,
			// TODO: this should redirect once we add support for client-cert logins
			expectDirectRequestError: true,
		},
		"invalid signature, valid cn": {
			transport: invalidCertTransport,
			// TODO: this should redirect once we add support for client-cert logins
			expectDirectRequestError: true,
		},
	}

	for k, tc := range testcases {
		// Build the authorize request, spoofing a remote user header
		directRequest, err := http.NewRequest("GET", authorizeURL, nil)
		directRequest.Header.Set("My-Remote-User", "myuser")

		// direct request against authorizeURL should redirect to proxy
		directResponse, err := tc.transport.RoundTrip(directRequest)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		if tc.expectDirectRequestError {
			if directResponse.StatusCode != 500 {
				body, _ := ioutil.ReadAll(directResponse.Body)
				t.Logf("%s: Status:  %#v", k, directResponse.StatusCode)
				t.Logf("%s: Headers: %#v", k, directResponse.Header)
				t.Logf("%s: Body:    %s", k, string(body))
				t.Errorf("%s: Expected spoofed header to get 500 status code, got %d", k, directResponse.StatusCode)
				continue
			}
		} else {
			proxyRedirect, err := directResponse.Location()
			if err != nil {
				body, _ := ioutil.ReadAll(directResponse.Body)
				t.Logf("%s: Status:  %#v", k, directResponse.StatusCode)
				t.Logf("%s: Headers: %#v", k, directResponse.Header)
				t.Logf("%s: Body:    %s", k, string(body))
				t.Errorf("%s: expected spoofed remote user header to get 302 redirect, got error: %v", k, err)
				continue
			}
			if proxyRedirect.String() != proxyURL {
				t.Errorf("%s: expected redirect to proxy endpoint, got redirected to %v", k, proxyRedirect.String())
				continue
			}
		}

		// request to proxy without credentials should return 401
		proxyRequest, err := http.NewRequest("GET", proxyURL, nil)
		proxyRequest.Header.Set("My-Remote-User", "myuser")

		unauthenticatedProxyResponse, err := tc.transport.RoundTrip(proxyRequest)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if unauthenticatedProxyResponse.StatusCode != 401 {
			t.Errorf("%s: expected 401 status, got: %v", k, unauthenticatedProxyResponse.StatusCode)
			continue
		}

		// request to proxy with credentials should succeed with given credentials, not with passed Remote-User header
		proxyRequest.SetBasicAuth("myusername", "mypassword")

		authenticatedProxyResponse, err := tc.transport.RoundTrip(proxyRequest)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		tokenRedirect, err := authenticatedProxyResponse.Location()
		if err != nil {
			t.Errorf("%s: expected 302 redirect, got error: %v", k, err)
			continue
		}
		if tokenRedirect.Query().Get("error") != "" {
			t.Errorf("%s: expected successful token request, got error %v", k, tokenRedirect.String())
			continue
		}

		// Extract the access_token

		// group #0 is everything.                      #1                #2     #3
		accessTokenRedirectRegex := regexp.MustCompile(`(^|&)access_token=([^&]+)($|&)`)
		accessToken := ""
		if matches := accessTokenRedirectRegex.FindStringSubmatch(tokenRedirect.Fragment); matches != nil {
			accessToken = matches[2]
		}
		if accessToken == "" {
			t.Errorf("%s: Expected access token, got %s", k, tokenRedirect.String())
			continue
		}

		// Make sure we can use the token, and it represents who we expect
		userConfig := anonConfig
		userConfig.BearerToken = accessToken
		userClient, err := client.New(&userConfig)
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", k, err)
			continue
		}
		user, err := userClient.Users().Get("~")
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", k, err)
			continue
		}
		if user.Name != "myusername" {
			t.Errorf("%s: Expected myusername as the user, got %v", k, user)
			continue
		}
	}

	// Get the master CA data for the login command
	masterCAFile := anonConfig.CAFile
	if masterCAFile == "" {
		// Write master ca data
		tmpFile, err := ioutil.TempFile("", "ca.crt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		if err := ioutil.WriteFile(tmpFile.Name(), anonConfig.CAData, os.FileMode(0600)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		masterCAFile = tmpFile.Name()
	}

	// Attempt a login using a redirecting auth proxy
	loginOutput := &bytes.Buffer{}
	loginOptions := &login.LoginOptions{
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

var (
	// oadm ca create-signer-cert --name=test-ca --overwrite=true
	rootCACert = []byte(`
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 1 (0x1)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=test-ca
        Validity
            Not Before: Apr 11 18:11:23 2016 GMT
            Not After : Mar 18 18:11:24 2116 GMT
        Subject: CN=test-ca
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:b8:e7:dd:d5:05:90:93:bf:21:58:06:bd:00:b2:
                    02:a3:5f:4b:e8:6c:22:26:87:76:22:ff:0a:69:4a:
                    90:c1:b5:2f:b9:09:7d:3e:73:75:04:b9:52:9f:43:
                    44:e8:67:2b:2f:25:06:03:b2:f8:2d:a1:10:8c:de:
                    f7:bf:61:7f:82:bc:4c:aa:c2:af:ea:b3:e3:81:9b:
                    e4:58:c2:99:7e:e3:81:b5:26:57:3b:98:fa:c1:59:
                    90:24:f5:98:6a:e5:c8:1d:6a:31:f0:05:15:b6:c1:
                    17:35:0d:03:eb:c8:bd:19:28:8d:33:b0:40:8b:63:
                    95:3a:80:bb:6c:5f:d7:1e:7b:e4:27:fd:89:6b:52:
                    46:1b:7d:2d:48:b0:3e:42:d3:28:32:ce:2a:7c:d7:
                    66:d1:ec:59:a5:1c:2e:62:78:56:c6:d5:0c:64:5d:
                    2e:51:8e:7c:6e:6c:6b:71:4d:a4:54:55:cb:fc:a5:
                    29:ea:e5:df:36:2f:c6:2b:cf:86:84:54:cf:4e:2b:
                    b1:3f:e2:ea:51:60:72:eb:2c:fc:67:d0:1b:01:21:
                    1c:4a:45:78:fa:d7:7f:87:92:d7:3c:21:4c:8f:0c:
                    90:f0:bc:df:56:1b:c6:2c:9b:cf:fa:38:88:95:53:
                    3a:2d:08:76:d0:2b:67:4c:15:fd:da:ed:83:67:d0:
                    d2:2f
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Key Usage: critical
                Digital Signature, Key Encipherment, Certificate Sign
            X509v3 Basic Constraints: critical
                CA:TRUE
    Signature Algorithm: sha256WithRSAEncryption
        45:44:e3:86:5a:0b:a4:75:57:f4:75:51:cf:19:1c:b8:af:a6:
        4e:80:1f:47:93:26:a4:32:ab:35:f2:e7:67:17:ab:96:8d:9f:
        82:10:d8:f1:e1:9f:3f:93:6d:ba:5d:22:d1:72:4e:d4:d1:f6:
        24:06:00:ee:ac:d4:e4:61:b8:a6:52:04:32:f9:a1:cb:8f:53:
        73:4d:cc:b5:35:32:b9:01:77:bf:db:00:b1:79:62:95:fd:da:
        1e:b6:43:5f:48:05:bb:99:66:49:05:db:14:c3:65:82:77:6d:
        d7:ec:b9:6e:0d:7d:8f:79:72:64:fc:e1:ee:15:0e:45:62:ec:
        ac:3b:b2:dd:bc:84:89:6d:d1:ac:c5:04:79:d4:f6:e0:ee:b3:
        1a:45:db:24:89:38:12:4a:3a:9d:4c:32:7b:cf:ba:a7:5b:44:
        be:3d:44:ca:63:59:3e:19:4e:d2:0c:c8:36:0b:87:22:fd:8e:
        34:ba:60:3c:d7:81:0f:5c:35:7f:6c:64:ae:cd:18:49:a7:07:
        54:cf:7d:94:92:f3:13:a4:f1:6c:2b:aa:4a:5b:30:f9:23:d0:
        1c:e2:56:6d:4d:c5:b9:19:2e:bd:9d:bf:43:2b:e9:8e:ef:e7:
        b6:dd:ea:22:52:ae:e6:94:48:1a:c4:1e:e6:04:b5:c1:86:de:
        49:03:ab:3a
-----BEGIN CERTIFICATE-----
MIICxDCCAaygAwIBAgIBATANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDEwd0ZXN0
LWNhMCAXDTE2MDQxMTE4MTEyM1oYDzIxMTYwMzE4MTgxMTI0WjASMRAwDgYDVQQD
Ewd0ZXN0LWNhMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuOfd1QWQ
k78hWAa9ALICo19L6GwiJod2Iv8KaUqQwbUvuQl9PnN1BLlSn0NE6GcrLyUGA7L4
LaEQjN73v2F/grxMqsKv6rPjgZvkWMKZfuOBtSZXO5j6wVmQJPWYauXIHWox8AUV
tsEXNQ0D68i9GSiNM7BAi2OVOoC7bF/XHnvkJ/2Ja1JGG30tSLA+QtMoMs4qfNdm
0exZpRwuYnhWxtUMZF0uUY58bmxrcU2kVFXL/KUp6uXfNi/GK8+GhFTPTiuxP+Lq
UWBy6yz8Z9AbASEcSkV4+td/h5LXPCFMjwyQ8LzfVhvGLJvP+jiIlVM6LQh20Ctn
TBX92u2DZ9DSLwIDAQABoyMwITAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUw
AwEB/zANBgkqhkiG9w0BAQsFAAOCAQEARUTjhloLpHVX9HVRzxkcuK+mToAfR5Mm
pDKrNfLnZxerlo2fghDY8eGfP5Ntul0i0XJO1NH2JAYA7qzU5GG4plIEMvmhy49T
c03MtTUyuQF3v9sAsXlilf3aHrZDX0gFu5lmSQXbFMNlgndt1+y5bg19j3lyZPzh
7hUORWLsrDuy3byEiW3RrMUEedT24O6zGkXbJIk4Eko6nUwye8+6p1tEvj1EymNZ
PhlO0gzINguHIv2ONLpgPNeBD1w1f2xkrs0YSacHVM99lJLzE6TxbCuqSlsw+SPQ
HOJWbU3FuRkuvZ2/Qyvpju/ntt3qIlKu5pRIGsQe5gS1wYbeSQOrOg==
-----END CERTIFICATE-----
`)

	// oadm create-api-client-config --basename=proxy --client-dir=. --user=proxy
	proxyClientCert = []byte(`
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 2 (0x2)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=test-ca
        Validity
            Not Before: Apr 11 18:12:17 2016 GMT
            Not After : Mar 18 18:12:18 2116 GMT
        Subject: CN=proxy
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:db:09:79:3d:37:2d:69:2a:2d:57:cf:87:27:a7:
                    6e:07:00:a6:af:71:19:18:1e:f3:04:00:09:54:52:
                    26:67:03:f1:a6:ef:35:e6:73:cb:ee:46:75:11:5f:
                    30:46:dc:d1:fb:2c:68:bb:a8:e0:60:0f:fa:59:f2:
                    d1:40:a1:79:29:83:8e:a6:b6:2c:22:c1:0a:3c:04:
                    74:ae:5d:a1:3d:db:9b:61:ea:bb:d1:77:20:26:fb:
                    c1:ec:e6:9a:0d:ef:df:8c:02:35:27:8b:69:9c:01:
                    a2:f6:bd:f2:a0:43:15:42:05:b2:77:1c:b4:21:58:
                    fd:23:65:e7:bb:e1:3b:9b:ab:0e:fd:5e:15:da:97:
                    3f:23:50:53:67:c9:2c:77:f9:fb:62:ee:4c:df:6b:
                    e1:4e:40:ef:f7:de:ba:6d:fe:32:be:f7:e5:4a:a5:
                    33:5e:ca:84:8c:d4:3e:24:18:9e:a4:b4:a8:02:3d:
                    45:5b:ac:66:06:72:70:ea:14:9b:14:9a:b6:50:29:
                    78:bf:49:80:43:ba:da:8d:03:dc:52:6d:4a:be:2f:
                    5c:1d:2a:27:65:4c:2a:bc:45:69:80:ec:2e:fe:55:
                    81:24:09:b4:2f:b8:5b:77:e3:cc:56:3e:b9:3d:57:
                    91:de:17:08:b2:c6:77:5d:9f:f4:b2:8f:d8:8d:a9:
                    2e:81
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Key Usage: critical
                Digital Signature, Key Encipherment
            X509v3 Extended Key Usage:
                TLS Web Client Authentication
            X509v3 Basic Constraints: critical
                CA:FALSE
    Signature Algorithm: sha256WithRSAEncryption
        0e:bb:6c:d9:7a:f4:b8:57:a3:ea:d3:36:2b:83:31:3d:ed:48:
        c5:7f:b2:ba:20:33:82:03:22:a4:3e:4c:54:60:66:74:17:be:
        ac:a6:28:86:0f:eb:b0:33:f7:5c:ba:d4:52:97:da:5d:00:04:
        bc:90:61:76:2c:d6:51:37:b9:8a:ea:c3:63:b7:77:01:d1:4a:
        56:98:fb:61:e1:94:b2:fb:c2:da:19:a1:8b:f3:33:fa:4c:b5:
        0f:7f:2b:3b:83:63:48:28:bc:d4:ff:e6:93:ee:a7:3f:b5:47:
        4b:9b:47:96:cb:b5:cc:e7:df:27:24:54:b7:3e:ec:e6:67:52:
        40:78:03:bd:7f:ec:3b:90:56:f1:bb:63:04:f0:6e:43:07:13:
        23:e9:b2:9d:84:25:13:5f:a1:76:3b:d9:72:cf:05:8e:2e:a6:
        9d:9b:68:d4:36:76:95:76:68:4e:1c:90:bb:22:c4:6d:3c:bd:
        16:bf:57:06:de:f6:76:1a:2a:10:dc:f5:d9:8f:23:a6:39:49:
        34:66:6d:74:2c:81:2d:0f:49:a4:d2:f3:8c:a9:dc:72:8b:7b:
        2b:95:37:9a:f5:b4:7f:9d:61:fe:04:c1:53:48:bc:26:8e:f8:
        01:8f:ac:24:4d:44:ac:7d:4d:fd:5b:a2:ff:b9:33:33:2e:83:
        81:d2:66:54
-----BEGIN CERTIFICATE-----
MIIC1DCCAbygAwIBAgIBAjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDEwd0ZXN0
LWNhMCAXDTE2MDQxMTE4MTIxN1oYDzIxMTYwMzE4MTgxMjE4WjAQMQ4wDAYDVQQD
EwVwcm94eTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANsJeT03LWkq
LVfPhyenbgcApq9xGRge8wQACVRSJmcD8abvNeZzy+5GdRFfMEbc0fssaLuo4GAP
+lny0UCheSmDjqa2LCLBCjwEdK5doT3bm2Hqu9F3ICb7wezmmg3v34wCNSeLaZwB
ova98qBDFUIFsncctCFY/SNl57vhO5urDv1eFdqXPyNQU2fJLHf5+2LuTN9r4U5A
7/feum3+Mr735UqlM17KhIzUPiQYnqS0qAI9RVusZgZycOoUmxSatlApeL9JgEO6
2o0D3FJtSr4vXB0qJ2VMKrxFaYDsLv5VgSQJtC+4W3fjzFY+uT1Xkd4XCLLGd12f
9LKP2I2pLoECAwEAAaM1MDMwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsG
AQUFBwMCMAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBAA67bNl69LhX
o+rTNiuDMT3tSMV/srogM4IDIqQ+TFRgZnQXvqymKIYP67Az91y61FKX2l0ABLyQ
YXYs1lE3uYrqw2O3dwHRSlaY+2HhlLL7wtoZoYvzM/pMtQ9/KzuDY0govNT/5pPu
pz+1R0ubR5bLtczn3yckVLc+7OZnUkB4A71/7DuQVvG7YwTwbkMHEyPpsp2EJRNf
oXY72XLPBY4upp2baNQ2dpV2aE4ckLsixG08vRa/Vwbe9nYaKhDc9dmPI6Y5STRm
bXQsgS0PSaTS84yp3HKLeyuVN5r1tH+dYf4EwVNIvCaO+AGPrCRNRKx9Tf1bov+5
MzMug4HSZlQ=
-----END CERTIFICATE-----`)

	proxyClientKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA2wl5PTctaSotV8+HJ6duBwCmr3EZGB7zBAAJVFImZwPxpu81
5nPL7kZ1EV8wRtzR+yxou6jgYA/6WfLRQKF5KYOOprYsIsEKPAR0rl2hPdubYeq7
0XcgJvvB7OaaDe/fjAI1J4tpnAGi9r3yoEMVQgWydxy0IVj9I2Xnu+E7m6sO/V4V
2pc/I1BTZ8ksd/n7Yu5M32vhTkDv9966bf4yvvflSqUzXsqEjNQ+JBiepLSoAj1F
W6xmBnJw6hSbFJq2UCl4v0mAQ7rajQPcUm1Kvi9cHSonZUwqvEVpgOwu/lWBJAm0
L7hbd+PMVj65PVeR3hcIssZ3XZ/0so/YjakugQIDAQABAoIBAEenNrkW1s0jVgf2
xLDtLaouxVh5OAtS/I6fcG3cHeHvQVspv8kuslS1SdCwAfv8etie83gIS7ZBI9XP
ADMTX6578euJhrCr06xEjOMJkBjLQW5ruptQS/1UuGDGIzlR8iA8DKVuDtNRGb17
7+YLa+XYNUSP6EFMeirdSEyG5tgKJ32j1SQAtIedhnrMRtdIfjLmus0c2efIXvgz
f26d3OclRy50X+P61jns/5ya+aocYKfzQ3Gp8ZKeIGZ6vw3tgID18eQ8vrjUJJWk
43UQg+axShqJTm/+unLpS3dJcXSSMu1OCzdCOnyiYiqL0KhJy8YC7doTQTQTq7VJ
SBkoQtECgYEA8S1I4FtSwU+Wv0Wa8b7QticOGOxDzfnEgkLQHC1I0hVnATKGKpvN
luOT8UBblwZssozFJ0UzpKWbPWYS7l+4G634A7qxu7+Byv9BwZITyg36LlGEbJXU
god22G6+Z6HeSQIFPElp8nY+UtpuzXmdvijlm3/RzViDiJx2PcS5A4UCgYEA6H/W
IMge3Alc0imTK3TIrNP6sPjztvKv6JVxrxN5GAzHx3mlwj8m+0naXeXqo/6krq52
wGaUCZPzehpN6R6H/d5VfANa9x1wCBHCPBEhPN+rnFiUlT2q5uNcg9uDwYisxJ96
+aInjsPaoCxAIHJFmxxFJH26Y7JBQydRoGRR+c0CgYEA8ILEhlkMMhN4tc5oMmSk
JsLT4C7df29xdKXEfBT85eTKD/ueqKcvYyYYxyHzNK0HgRe5FOyCD9PG+HfusSFr
rM7U4oMv85eLjDD6FlviuEEwGTjZ4p+YiYMmFbh60UYvMod9SR29NjqM9Hs4vFhn
4tdOAsB5LVrz8Sx3Dio8hzECgYBWe8bw5r/j5W+rlV9zGLvU3f0we0pc0SVyBLUH
BN1UftyJbMyl1svvSWd66h0/52bmu2rc4stKTMiSsNouTvcTDfMKcE0UAtU7iy+P
HGgatrClNaX/ZbL+s7AkNDFseiSZ9yDNXu4MAvp9/jfUWe1eZ0Oo8UO19gakrimE
2gxMOQKBgGYuj4I5TmtROJDTRxrRRzQVnoTV3GesequaW/y5UM4xr2ThLevnPCMI
dSmVatvGsqYsQbPKOAp3ZcMQiqTIPUFeYSuzs3TuTs/tZ1cwfseN4M2bs258ynsT
51PHf9W7BDmvHZn8JMg688SSklatAUj4h3nnGWco1de9YL1bBLPg
-----END RSA PRIVATE KEY-----
`)

	// oadm create-api-client-config --basename=other --client-dir=. --user=other
	otherClientCert = []byte(`
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 3 (0x3)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=test-ca
        Validity
            Not Before: Apr 11 18:12:25 2016 GMT
            Not After : Mar 18 18:12:26 2116 GMT
        Subject: CN=other
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:d8:6d:9e:41:51:d3:e9:99:b9:6d:37:4d:72:32:
                    6d:e8:3e:01:38:15:30:cd:5c:fb:0e:e1:76:01:32:
                    38:cf:1b:0e:8c:ec:21:5b:87:25:aa:6b:ac:6d:4b:
                    a9:a5:c4:5e:aa:43:32:70:96:9f:30:dd:c7:ba:0f:
                    a8:de:73:72:b5:10:9f:55:0a:80:bf:10:cc:c7:e3:
                    55:9b:6d:e1:13:6b:c2:d7:be:1c:c4:29:7b:db:06:
                    bd:7e:22:a9:be:1a:af:cb:59:98:cf:0d:a5:e7:f7:
                    cc:cd:92:05:3e:c8:a6:1e:cf:a3:05:90:b8:a8:76:
                    7a:a4:44:78:82:e4:7d:ba:1b:6e:4b:6f:1b:39:96:
                    04:c3:ec:28:1f:ac:c5:36:09:2e:71:23:00:35:44:
                    6e:ac:73:7b:5a:ad:c9:5c:35:4e:0c:5f:d6:09:9c:
                    a0:a5:2c:ce:d7:5e:d6:93:e1:9c:b4:ec:61:bb:9f:
                    ff:32:dc:64:9a:d5:bf:7f:20:84:a9:e7:5d:69:b6:
                    87:42:e6:a2:31:1c:32:50:6a:20:18:3e:f6:f8:c7:
                    b8:63:eb:a2:35:da:4f:eb:34:f3:e5:e8:da:06:fd:
                    c9:19:4e:45:b3:5d:e8:be:ed:18:e8:b5:30:42:eb:
                    70:64:72:76:03:30:04:81:38:f3:7c:09:98:5b:1d:
                    0f:dd
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Key Usage: critical
                Digital Signature, Key Encipherment
            X509v3 Extended Key Usage:
                TLS Web Client Authentication
            X509v3 Basic Constraints: critical
                CA:FALSE
    Signature Algorithm: sha256WithRSAEncryption
        61:bf:f3:81:d2:c9:46:e3:bb:68:0d:ae:b3:ce:56:1f:bf:3b:
        93:ba:65:54:04:37:25:5e:bf:2a:b6:79:2f:bd:17:3f:eb:85:
        9a:ce:78:ff:f8:b5:5a:3d:f9:99:1d:24:41:2c:0d:d1:c9:63:
        19:19:75:b2:a6:65:da:d6:a5:ae:31:57:ec:8f:d6:0d:d9:86:
        5e:b8:f1:98:a7:43:12:1c:d0:71:d2:5c:2f:a3:bb:5f:89:fc:
        dd:9a:fc:fb:8a:9b:ed:73:3b:6d:25:90:c9:70:96:88:d0:67:
        d7:10:17:35:e9:6e:d4:2b:61:f6:d0:4d:02:75:73:7a:cf:03:
        ed:d2:e2:3b:6f:cf:58:2e:92:e8:b6:c2:e1:1b:5d:33:46:3f:
        95:53:67:7a:69:92:be:2d:e8:59:cd:71:16:a4:a4:89:80:ee:
        67:97:47:84:a8:0e:f7:fe:7c:2e:97:b1:f5:11:84:30:90:1d:
        a7:44:55:15:93:c9:fc:16:16:28:2c:cd:8c:1d:82:a0:ff:35:
        61:ec:8e:ae:59:88:bf:87:55:85:79:cd:20:58:79:c3:6b:4d:
        78:43:c0:48:44:6d:78:24:e2:26:24:99:97:81:b9:43:a4:6d:
        1e:dd:31:53:5b:36:49:cc:df:58:e8:f2:a8:25:30:cd:69:a8:
        c1:0d:c7:84
-----BEGIN CERTIFICATE-----
MIIC1DCCAbygAwIBAgIBAzANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDEwd0ZXN0
LWNhMCAXDTE2MDQxMTE4MTIyNVoYDzIxMTYwMzE4MTgxMjI2WjAQMQ4wDAYDVQQD
EwVvdGhlcjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANhtnkFR0+mZ
uW03TXIybeg+ATgVMM1c+w7hdgEyOM8bDozsIVuHJaprrG1LqaXEXqpDMnCWnzDd
x7oPqN5zcrUQn1UKgL8QzMfjVZtt4RNrwte+HMQpe9sGvX4iqb4ar8tZmM8Npef3
zM2SBT7Iph7PowWQuKh2eqREeILkfbobbktvGzmWBMPsKB+sxTYJLnEjADVEbqxz
e1qtyVw1Tgxf1gmcoKUsztde1pPhnLTsYbuf/zLcZJrVv38ghKnnXWm2h0LmojEc
MlBqIBg+9vjHuGProjXaT+s08+Xo2gb9yRlORbNd6L7tGOi1MELrcGRydgMwBIE4
83wJmFsdD90CAwEAAaM1MDMwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsG
AQUFBwMCMAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBAGG/84HSyUbj
u2gNrrPOVh+/O5O6ZVQENyVevyq2eS+9Fz/rhZrOeP/4tVo9+ZkdJEEsDdHJYxkZ
dbKmZdrWpa4xV+yP1g3Zhl648ZinQxIc0HHSXC+ju1+J/N2a/PuKm+1zO20lkMlw
lojQZ9cQFzXpbtQrYfbQTQJ1c3rPA+3S4jtvz1gukui2wuEbXTNGP5VTZ3ppkr4t
6FnNcRakpImA7meXR4SoDvf+fC6XsfURhDCQHadEVRWTyfwWFigszYwdgqD/NWHs
jq5ZiL+HVYV5zSBYecNrTXhDwEhEbXgk4iYkmZeBuUOkbR7dMVNbNknM31jo8qgl
MM1pqMENx4Q=
-----END CERTIFICATE-----`)

	otherClientKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA2G2eQVHT6Zm5bTdNcjJt6D4BOBUwzVz7DuF2ATI4zxsOjOwh
W4clqmusbUuppcReqkMycJafMN3Hug+o3nNytRCfVQqAvxDMx+NVm23hE2vC174c
xCl72wa9fiKpvhqvy1mYzw2l5/fMzZIFPsimHs+jBZC4qHZ6pER4guR9uhtuS28b
OZYEw+woH6zFNgkucSMANURurHN7Wq3JXDVODF/WCZygpSzO117Wk+GctOxhu5//
MtxkmtW/fyCEqeddabaHQuaiMRwyUGogGD72+Me4Y+uiNdpP6zTz5ejaBv3JGU5F
s13ovu0Y6LUwQutwZHJ2AzAEgTjzfAmYWx0P3QIDAQABAoIBAQCQF8Nid8lf4NIc
jdJJMpwMIKQNI8afI8We7ar0NuytrrTsTBYVaxA/u3pMNjDXxbrFHwIJBa8tCKt+
DAkBOdnoBQ4fv2NiUhwVBR0s42YT2Q4bN17Nl1T3yTAGN6vNftUFzTw4tjx8CXZY
c1x8pXg8UT+XZ/gZaPBUR6X4d4nhikGqSiILNh6uDjuYUxxMgea0qrAnx4HvBcBF
I1X5zg+turWcuTXoR39Ijn3UNnNZrp8XUqjA850dzQQnZqrcDnD0lqV1HOcF5V2H
VABBIVL8Jzm7mn+k6+NTVC3eWFK+EPwY8/OwHGa3O9LsA0l3knsG8x4FvaRFXTSY
fYSkExudAoGBANwE79pvjGE9kJ5MgJVg7klNv5XWIoaZnmiaPovM5LX7Fylw6vV3
QGEj8x3VO3EqBB66g70SrqoNgCmVNx3oe8+KVX1+8XVhX4DIQLbLJBFfvaNZV7sh
IOM00hhwdKZk+szb2CS3yRo2rsmD1yEr7djsnC1l7UiLn4TJ9bA6oK+nAoGBAPvS
V07KwKKFQUv7cLwPDy1b8G7JbFOmYuc8Zber3S9YtrpFKjX2lR9bXk6iPSa2k485
cqs1RM2/Mrw0uXKpW3jrwVE6dy2IyKLuMBcvKWlUVY02cGA0hV7A2CJKGcPxsFEP
txj5R+VN/FDcm7RzE0jmJNay+5PmDfchom4WXFTbAoGAWhHXUvfpYwF+C5/L39sf
kXi3npJb7fhDZhUG19pYIruYvslQFo7sFxhNdYAOZoRJzX6TYbqdMFZ4ig1g0+iR
juPVnZtzI5dqLmFMRMiiik5EZvOzO5MTUJAWFhUrW9bo6SZytI1cUVPjd/F2B0lh
hDVQtjEM0279LbIz1yIZF+8CgYEArRMlRJcfjNPPTBy1n9st0DwXZN11YYzC/zDI
rFMoAymS9TUiTNJ8LYALsjnZk6j6g/607C0Ba/OUODx4lPEHWHWYeW6YiKgxVaIl
VVnpuWXoItUeqVCPtc8O/Yo2aTDMwPnvGvAB1P0jhKQLNBu/TmQ3P4TmWgFM6eSp
Eca2kO8CgYAstEpSdnMQAHE92HsTSBA+aFm5jfYE/2papDcVE/Q2AqMN+ZjZvfnj
vWyX2MBY8yccNwUyNiwbEfiy9A+XLtNpsuVvNGzOp5CQAs/wIPTCfRD7zVUtIhUN
PVEo4cjWU2JK68lSTyW3UWdoPwcKIdDlnure/al7NpIG2g6weBubpQ==
-----END RSA PRIVATE KEY-----
`)

	// invalidClientCert is a client cert with the desired name which is NOT signed by the root CA crt, but by another signer with the same name
	// oadm ca create-signer-cert --name=test-ca --overwrite=true
	// oadm create-api-client-config --basename=invalid --client-dir=. --user=invalid
	invalidClientCert = []byte(`
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 2 (0x2)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=test-ca
        Validity
            Not Before: Apr 11 18:17:29 2016 GMT
            Not After : Mar 18 18:17:30 2116 GMT
        Subject: CN=invalid
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:ac:87:5e:71:36:39:6d:2b:33:40:0e:ff:d6:3d:
                    67:a5:8b:3d:7e:56:c5:3f:49:a9:42:7d:6f:da:30:
                    cc:0f:cd:64:cc:20:91:e4:41:b2:9c:54:f8:9a:fe:
                    ba:7d:e6:2b:2f:ff:fc:c8:7b:4f:bf:3d:61:5c:18:
                    6e:10:6f:a8:33:9e:54:8f:f7:ac:34:57:f4:ff:00:
                    c7:24:07:dd:df:47:e1:bc:0f:d6:41:b5:e4:5c:c0:
                    36:90:4b:2e:b2:97:a9:2c:7f:c4:f7:7a:2b:96:1b:
                    a4:20:ba:db:df:4b:72:ff:e2:ae:46:79:5b:5d:72:
                    41:16:3d:a4:c5:31:cf:12:0c:ca:59:d5:72:c9:fe:
                    87:51:b4:54:0f:eb:46:79:95:8b:2b:ba:a3:51:71:
                    87:06:c2:5b:80:59:74:c4:d8:bd:c6:7f:56:e9:8f:
                    95:d1:85:1f:67:39:20:33:1f:3a:ba:9a:81:c6:32:
                    b6:a6:e3:1d:15:97:19:c9:71:9e:95:ec:d3:38:3b:
                    2a:28:37:f8:cf:ea:c3:3c:af:84:b9:d6:64:8f:e1:
                    cd:29:d3:9a:ba:48:82:50:85:0f:07:d2:d4:e9:83:
                    42:9f:22:25:4d:55:9d:38:32:9c:f1:07:17:14:bf:
                    80:7b:c5:88:6e:f7:60:50:ab:95:32:a3:0f:98:74:
                    49:21
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Key Usage: critical
                Digital Signature, Key Encipherment
            X509v3 Extended Key Usage:
                TLS Web Client Authentication
            X509v3 Basic Constraints: critical
                CA:FALSE
    Signature Algorithm: sha256WithRSAEncryption
        92:5d:46:49:82:df:80:84:8c:7a:4d:4a:4c:ab:13:20:70:ee:
        36:42:84:3c:30:67:51:a6:b4:c8:e2:0f:13:c4:2b:51:9f:2d:
        7a:0d:be:77:3e:90:67:81:55:f8:3d:b5:c6:00:a1:ca:86:d1:
        83:67:d6:7d:4c:e9:c0:af:53:3b:23:5d:17:1f:8b:c0:c5:ae:
        a3:f2:7c:b5:7b:9a:fc:1b:09:a8:78:ed:12:13:fd:ed:97:0c:
        e4:eb:f8:b2:63:d1:bf:89:db:84:1e:45:f8:5b:5b:d2:93:c2:
        26:5c:61:b4:a9:05:30:45:5e:f5:c4:95:8e:98:83:4d:41:61:
        5d:cb:83:a6:72:b6:af:70:64:8a:72:5a:1f:20:cb:8b:7c:82:
        52:26:45:9d:58:da:c8:0b:e0:ac:00:f0:d4:12:85:2c:2b:a5:
        29:db:54:e6:83:e3:48:d2:61:65:e6:13:31:09:cd:c8:ba:39:
        3c:f7:ca:ab:93:ea:21:12:5f:49:0d:46:17:15:cc:ae:72:a8:
        66:97:56:f3:2f:39:75:b5:f9:3e:ff:5a:4f:3b:8c:16:4d:bf:
        70:55:c5:b7:ee:74:d7:39:4b:da:f9:da:39:84:25:62:24:a8:
        b8:f3:2d:6b:e5:71:60:26:cb:71:ad:bc:25:2a:f9:3a:ec:25:
        b9:c3:5c:e4
-----BEGIN CERTIFICATE-----
MIIC1jCCAb6gAwIBAgIBAjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDEwd0ZXN0
LWNhMCAXDTE2MDQxMTE4MTcyOVoYDzIxMTYwMzE4MTgxNzMwWjASMRAwDgYDVQQD
EwdpbnZhbGlkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArIdecTY5
bSszQA7/1j1npYs9flbFP0mpQn1v2jDMD81kzCCR5EGynFT4mv66feYrL//8yHtP
vz1hXBhuEG+oM55Uj/esNFf0/wDHJAfd30fhvA/WQbXkXMA2kEsuspepLH/E93or
lhukILrb30ty/+KuRnlbXXJBFj2kxTHPEgzKWdVyyf6HUbRUD+tGeZWLK7qjUXGH
BsJbgFl0xNi9xn9W6Y+V0YUfZzkgMx86upqBxjK2puMdFZcZyXGelezTODsqKDf4
z+rDPK+EudZkj+HNKdOaukiCUIUPB9LU6YNCnyIlTVWdODKc8QcXFL+Ae8WIbvdg
UKuVMqMPmHRJIQIDAQABozUwMzAOBgNVHQ8BAf8EBAMCBaAwEwYDVR0lBAwwCgYI
KwYBBQUHAwIwDAYDVR0TAQH/BAIwADANBgkqhkiG9w0BAQsFAAOCAQEAkl1GSYLf
gISMek1KTKsTIHDuNkKEPDBnUaa0yOIPE8QrUZ8teg2+dz6QZ4FV+D21xgChyobR
g2fWfUzpwK9TOyNdFx+LwMWuo/J8tXua/BsJqHjtEhP97ZcM5Ov4smPRv4nbhB5F
+Ftb0pPCJlxhtKkFMEVe9cSVjpiDTUFhXcuDpnK2r3BkinJaHyDLi3yCUiZFnVja
yAvgrADw1BKFLCulKdtU5oPjSNJhZeYTMQnNyLo5PPfKq5PqIRJfSQ1GFxXMrnKo
ZpdW8y85dbX5Pv9aTzuMFk2/cFXFt+501zlL2vnaOYQlYiSouPMta+VxYCbLca28
JSr5OuwlucNc5A==
-----END CERTIFICATE-----`)

	invalidClientKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEArIdecTY5bSszQA7/1j1npYs9flbFP0mpQn1v2jDMD81kzCCR
5EGynFT4mv66feYrL//8yHtPvz1hXBhuEG+oM55Uj/esNFf0/wDHJAfd30fhvA/W
QbXkXMA2kEsuspepLH/E93orlhukILrb30ty/+KuRnlbXXJBFj2kxTHPEgzKWdVy
yf6HUbRUD+tGeZWLK7qjUXGHBsJbgFl0xNi9xn9W6Y+V0YUfZzkgMx86upqBxjK2
puMdFZcZyXGelezTODsqKDf4z+rDPK+EudZkj+HNKdOaukiCUIUPB9LU6YNCnyIl
TVWdODKc8QcXFL+Ae8WIbvdgUKuVMqMPmHRJIQIDAQABAoIBAE7weTPPnaLnm0F6
G3DJE71Y4kAGL6XvbDRx9FWe8h9g2PfVByurK6//6OfyGR41zBjgRabtVOWpjfx3
aRS4IfvMO+DLb81bWUu77WH8/3WEDDLiBCR4tw4BHHYVED7CybMEmviou3ypFQWs
uaGHggy2iQrRyA4Pktw8REG9soMM+s+T0zlfexbeXgz7OJYd5QStBslI5ZJhHa1I
LW94hrU0Yj1ONP2hCfMc5H808zOkMUSZMgRMTXXZQzo2XdujLelRxQ1oaZTU4aQM
SwZG1vzdDjjFhXW1sjD7G2DoyTxbDVOIfqYOmO/t7witXo2g52hIXhb4RFoxD0eZ
dFDRyakCgYEA0mU9X2rc1GU4OlKUOd4phsQfLmRk4ya7i2Zica3n4PFvaz+XoIZ0
NZf2xwZlY5MX21tBadIFW8C4GilDscgLrP8P2gW15hP6bKpHuB2nkJcGdNi6Tg8f
0Vvf7L4RHamolN7yk39zJoC9KUDaPAtqGB4niUDCsjc6LdJTqW/oCHMCgYEA0ezs
Dxe8j9l34BEqZxu71NjUKovUQJv+0kz301W5gVCFAY5oBDfFf42IHl5q+B7TFLaP
6xEEdyh0K6GrGTt9YtWB3HYPEStzBf0fXAg7fJHPn12mn8/B0UFMkJKyuRnPOW7t
3WOROzChSpWWKVUMFJIBCyfMvEjUgm8SzUluxxsCgYBe8g8DK09ijhcUwsVfY/Fr
fr/viKC6nXUPEHImiOtWaL32MSl06Jgyw1Q7Npi0meGvPPxFC+EdKdgq/iotZXBX
bncx1Vfj72oYdbON09wVdQIV4uQYa9zY9tQTmyZQM4r/O6lOhLprSreSkVCqvh/v
qFQBLXdvQ1r+6KaWlQiqHwKBgG7s6FeZTVQdr5BAwc02BGyWHpZUyNVTGLV7YkDT
vXAtYfrOivwflEawPMr/TTrK3vLE/QtTNK7aO3iKtuRgYQMGmtYptBB4ixERDa8N
0pEiYzlsvQ0ZNOsjvBdwzOuuTaeljB8965IBQlks7entPLLp648/epnLSi+aDa9Y
LCcdAoGBAL1qU+P+8+8Lfdj+/8GfaZdZQnqeOzLkxzGDcuVDNYACIVGmGJqL01Gx
RUVzUCnG68qdoai960Yo0w41U90hsCpBny50SShYu3kL67Gqn03UcEYDw5pmUPwn
NykkJ6u51LArDs6E8hA0aoMrQbZGZiod93dPHlbFhR3+4t4l8wDJ
-----END RSA PRIVATE KEY-----
`)
)
