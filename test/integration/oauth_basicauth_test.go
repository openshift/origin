package integration

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/crypto"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/util/tokencmd"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const (
	basicAuthRemoteCACert     = "remote-ca.crt"
	basicAuthRemoteServerCert = "remote-server.crt"
	basicAuthRemoteServerKey  = "remote-server.key"
	basicAuthClientCert       = "client.crt"
	basicAuthClientKey        = "client.key"
)

var (
	basicAuthCerts = map[string][]byte{
		basicAuthRemoteCACert: []byte(`-----BEGIN CERTIFICATE-----
MIIC6DCCAdKgAwIBAgIBATALBgkqhkiG9w0BAQswJjEkMCIGA1UEAwwbb3BlbnNo
aWZ0LXNpZ25lckAxNDI4MzI4NDA5MCAXDTE1MDQwNjEzNTMyOFoYDzIwNjUwMzI0
MTM1MzI5WjAmMSQwIgYDVQQDDBtvcGVuc2hpZnQtc2lnbmVyQDE0MjgzMjg0MDkw
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDHkZqXa6lECvQ/UQtIOqFH
XJYU83krenJvGcplGgxmuVNbMegaS0Qp6IpzFD3/z3kbWYMrTkv2VAY8yWcHS21w
oxOyNuUmqyQWOIQEL8T6CBwmd2TyctUjlYC+5RaJL3y+fNpAQNsKM5W1AxCamjki
YkqKN/YUEhP8PCFabRgNtMYIDHzy1WK7wfVFMbS3VHtqiMtmN6mdGvLeRNyKR96b
gB0gn24wtaxvhWuc5RipuHzNmcxxiBN0EFbujYDdxo82DqXRdpti9feO7gYhsx+n
6DbC3n1CIfjz9gMyS1Sj6aW91NjnQ1HaqmefgHFxP2DjLxOyR3tAagQMcVYS5oIL
AgMBAAGjIzAhMA4GA1UdDwEB/wQEAwIApDAPBgNVHRMBAf8EBTADAQH/MAsGCSqG
SIb3DQEBCwOCAQEAdtxjmPzOBuxJEApmEOfOIYlhaCD19wR/S/ViK1u7vPlk3ZG1
FdUYGpNHfFL+LT91VSh7u/cXh+PDgQcX+M+V9sJpG874Q8qD61XQdhDeXeyLO2AY
Q9cRRECYNu5xgjDQJKmVu1gvYYlMyv2zM3bbuERGlPYZvsXkBoNm8NA2SfPGPZHK
0XMRcMqC9I6vbwA5t0ayR9q3NeUW6ANCJ4IxQg95ITuQSBRQykrkn4hP3J+7fatg
ZHuTqw/FOVdyO8ure+G/9ZpMC7ILWxn9B8fIU+pQS3lEdyrZtG72aDqwjfyZPNcn
QEd5Ffk4IoyOWAsz5EpZxUul9cMbVhDu1WtLZQ==
-----END CERTIFICATE-----
`),

		basicAuthRemoteServerCert: []byte(`-----BEGIN CERTIFICATE-----
MIIDeTCCAmOgAwIBAgIBAjALBgkqhkiG9w0BAQswJjEkMCIGA1UEAwwbb3BlbnNo
aWZ0LXNpZ25lckAxNDI4MzI4NDA5MCAXDTE1MDQwNjEzNTMyOVoYDzIwNjUwMzI0
MTM1MzMwWjAUMRIwEAYDVQQDEwkxMjcuMC4wLjEwggEiMA0GCSqGSIb3DQEBAQUA
A4IBDwAwggEKAoIBAQC/3EvxEyBXCYh7Myrxynbr+S/01uZxAdIhRRHeyTApJqK2
RJabCswXIdM7Wj0hos3uahyF00L6CzPD3r01kwHXpfWavlBJULOLGrsWDSLH112L
8TVcHh216JoJjBmzaMiDDHGCWyoEMWwzSJ1cqxmlWd36wB0mMyDayAu7heRD4wfO
gFe2FbeDtEI3Py/TqL9xLPf0h3kFa18kHnhaCRy0xtYdD79dfVsYmIZe7FoTyhO8
g8Xdl0uerGjs3MBZPyqCTyVRFHiFuw53Z5U4OR06qWYf+kU9LvMuMMGlM/0gYZL/
2Xi/UTbBy8x48l48NypO1Bu60QmvEIWPvmfzUnOlAgMBAAGjgcUwgcIwDgYDVR0P
AQH/BAQDAgCgMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwgYwG
A1UdEQSBhDCBgYIJMTI3LjAuMC4xgg0xOTIuMTY4LjEuMTA5ghtrdWJlcm5ldGVz
LXJvLmRlZmF1bHQubG9jYWyCGGt1YmVybmV0ZXMuZGVmYXVsdC5sb2NhbIIJbG9j
YWxob3N0ghdvcGVuc2hpZnQuZGVmYXVsdC5sb2NhbIcEfwAAAYcEwKgBbTALBgkq
hkiG9w0BAQsDggEBABybqmyzPMq+pqS0qBDECw7PZYmmosCdtesFSFrD2PcOTnMz
ABWgyUrLH1Rfu98faTHPkLX0BptQqpg1TmjZmZicYJvrNs5vw/r+RUxyIdb0dsSg
EL3JOFNJfRThv1q703sJbR+GyQoT3sFRv34oqoCPEYyZq1p6wjvCXd6kq7bCgAxD
+kuaz4XL+BqpX/iIU57xxnhqWasImKS5HI/L7Blus///M1Oo1Saab/riKWKCOhdp
fEWSCmqxkfu8n8b6NvILqqxjnPyX+gjxb966i1AtsUs3g/1Z7mlR88lDrV247Pjk
CrAs4P4A2tMglxlhJeM1UqJImhATQMihPlCIGZ8=
-----END CERTIFICATE-----
`),

		basicAuthRemoteServerKey: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAv9xL8RMgVwmIezMq8cp26/kv9NbmcQHSIUUR3skwKSaitkSW
mwrMFyHTO1o9IaLN7mochdNC+gszw969NZMB16X1mr5QSVCzixq7Fg0ix9ddi/E1
XB4dteiaCYwZs2jIgwxxglsqBDFsM0idXKsZpVnd+sAdJjMg2sgLu4XkQ+MHzoBX
thW3g7RCNz8v06i/cSz39Id5BWtfJB54WgkctMbWHQ+/XX1bGJiGXuxaE8oTvIPF
3ZdLnqxo7NzAWT8qgk8lURR4hbsOd2eVODkdOqlmH/pFPS7zLjDBpTP9IGGS/9l4
v1E2wcvMePJePDcqTtQbutEJrxCFj75n81JzpQIDAQABAoIBAQCg9eroV4l8O806
vtS6gYd/tVEcceZmzIZDzzSM2fEDtRwxGh3X+Rj8Fy6lzrEWtQVbjb5cL78zE47c
NtQ6TBjxmJQSvLOSrpfSjhyDBYY2bmJW84g2vjVi1b/VXqp5L+F4wEnCeUUou2Xx
KGyMwCcF5/0pT1+lGqPnqIjfTBcTM8InM21PdIVPi7uWT2NfvKWvoPAynT/n/Vn9
3FHQ47IavgIBfthJT8nT2FScE47bN3YvtDGQnD+M3orvTNDoH/+uuhz3EwX213yI
daA5UJZCx+BElJVYFzGcdWlTelSAsGlrh4MLe+eZVXaFvkbkRI5EUOrPdQ8F0aXW
3/PEjWiJAoGBAO2YvdsfsFZe3mcAi8Rcpyky1jXfDW/S92kmPA1VwctE5tNn3hIg
4rVETITudRif8lLGt8u9X5MqgNgLZmMjzzxOu7DaWFOGDiQmEbE3r9AQBlf5gdgV
Zchg+T3WuynVWilt8+Kol8iJvCF4gpKUr23VYVO6RtzlN+qwzE8rlOWjAoGBAM64
qZMGj78CixCz0HAY2d+Q5bzOVaREL84yAMbwxZ0wcLL4YvBeAaJ86TrKd4V5XQgn
w5JUOhEBzE29rU4EKZjHILZSsm+6sZVmjD4OLySuKTN4oKCS62GiFAlgLjw3rvJR
peXcePUjBSe90sCuo1D4lNvCvribK6lZTIFfMAYXAoGAOSAJPb7/ubRzipZSBHM2
aaxkXm1zoJg7jhd4RsiAoKu/R8LoXLl1aJm0QB3JH5ONQqOumxi7+vk0Iz2Sb3Gz
qM9RRzMoG2TWz5Arns1BwyenLs25j0eNwkC2jEytkWBPnjhmc++PFtMu3WlJE48W
IrU0Alp+ISwnZpD9fmd/FDsCgYAlWlS5zlu3Bfye3f7x4mur7AC3JwlujyucNIjT
abordw9GJ3+pMzNUawGxr9f89DsNODIshK+hVxPVkEp6aGIjywdsKnE3oyJnfook
xGdcV2P2evt7SFDj6Wd5cjmog99Gxd4WNMpecR+DWNd2HZhBD0nGk9/md5NiHFKo
pcyFrQKBgQCQs7UQnVJNFe881ongZRPPbipafqu9A8MGOFN4BK/PqmiN+RdTc4ii
2vg6VVuVmMFyVQaf79lqkc3stlxdJ4YfUFgv4WYTSFF0yvU2+bn5uMVo4ltL5jgL
BzpfxtUnw+Tjm9YlZWiwaSn8vAGOZ7Nvud4I3wr5SEo2soKDLF8Dog==
-----END RSA PRIVATE KEY-----
`),

		basicAuthClientCert: []byte(`-----BEGIN CERTIFICATE-----
MIIC9jCCAeCgAwIBAgIBBTALBgkqhkiG9w0BAQswJjEkMCIGA1UEAwwbb3BlbnNo
aWZ0LXNpZ25lckAxNDI4MzI4NDA5MCAXDTE1MDQwNjEzNTMzMloYDzIwNjUwMzI0
MTM1MzMzWjAiMSAwHgYDVQQDExdzeXN0ZW06b3BlbnNoaWZ0LWNsaWVudDCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALhYytXmJDy8FExmfGQqP1qo8SG7
19SyYheGyBSspN4+hESUudiU7wefQQ50N5Z3ykro83a3OgcsE52kaeo0OM0dIxNK
oloiwSKSbC+5fNP4eG818SPCEdu1ww70oe5QynLzhDDYxXKRSYuJlvlu+6NW8xu2
Ok9kYh3P7J60EyG+TvxXiQoMqH8HPCcTbJ+W09iDjkNPWVKguELjqNqNA35iDVCX
X4j7/hMvR9ajL9D40eg4mz7VwCg6xosf/D6/naZwdDoTutS9wjwQHDsSoHvLyBwQ
XxQsqGuZlkrjy0eE84V2XvjFhJT2hrOcM77O9nkQ1LqoZPAqNBgHVWKm4tkCAwEA
AaM1MDMwDgYDVR0PAQH/BAQDAgCgMBMGA1UdJQQMMAoGCCsGAQUFBwMCMAwGA1Ud
EwEB/wQCMAAwCwYJKoZIhvcNAQELA4IBAQBi8eTprGaDbbp7ooPr1jAFU61l9Kty
pty5DMIQdxW10LfgBx6Z3cDQTjrFqh9xnfp7Adfv2Cj2RR+idPVA38FSpexvmfoC
fRptyn5nFkkINCmVrEmuVaEp9yJz1K2+L4UD3EfVhbFWZZZ/wATL2XTf3c8UicBh
78QD+7NkY06cV76MyTOMl/NnYA41a3rL6+XIYglaOnWDba0cMIToKOEsQ1pEufDM
3pD6s4KDVNLJQtpKTyfBDUVgESu0xR3Peqavqzy0NnF19XcB+lvIgzALDrPXszo5
9cYNPlc1SOF90vNaMytEOaFekrHBDUlYhUENxxvT1IDLO1ADLh+KOhfE
-----END CERTIFICATE-----
`),

		basicAuthClientKey: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAuFjK1eYkPLwUTGZ8ZCo/WqjxIbvX1LJiF4bIFKyk3j6ERJS5
2JTvB59BDnQ3lnfKSujzdrc6BywTnaRp6jQ4zR0jE0qiWiLBIpJsL7l80/h4bzXx
I8IR27XDDvSh7lDKcvOEMNjFcpFJi4mW+W77o1bzG7Y6T2RiHc/snrQTIb5O/FeJ
Cgyofwc8JxNsn5bT2IOOQ09ZUqC4QuOo2o0DfmINUJdfiPv+Ey9H1qMv0PjR6Dib
PtXAKDrGix/8Pr+dpnB0OhO61L3CPBAcOxKge8vIHBBfFCyoa5mWSuPLR4TzhXZe
+MWElPaGs5wzvs72eRDUuqhk8Co0GAdVYqbi2QIDAQABAoIBAFf70Ptb9ymhnpKE
S5RG8avkfAncrHtQlC6kXnQ3ngyQi/JrhXPQSXE62gL7Btji3YL5QdIES0bHC6mu
ofps6DtFT8tSUMByW/mTJt10SxakPV7ewPOPGZTiYHGP4oVqu+U3Qn1JyJsQqqhV
h+AOzz07L5anV5cy0v1lkoqAaa0tZvhA2kIezanErhTLFSn/eqoo2i0omxUtE/Ii
cAH0/kLTfmajzKoUmvCDEt5ORhIO/AsImKVvRR5Ub4Ra+IEp1oVRnt1MNaoU+JeO
RXhO8lIdN7JE50TNW0Oa4ZCVLWOhpP36nbwDTmUEX9NZH2PKqe78n5l2Ew+4HFwQ
MGhrdPECgYEA3s8gsZ59F2COB01ptyGdlMH0ItyM3jDSYtfucQ0IbVQgIbG4ptzj
elUK31xDwR1QOhC5MKAyXY4QIqIc00s3CHoUZ30dsqhZbnR/A0X1fYQDQ3OtcIME
QK1wFQGhE5ccrwv2jwYhKHp63Fp2Oh6qQBVZ5/nmuNn3sTW8OOmNRzUCgYEA087m
CnVVHZmD1tcdwYfa6Nw1meOEMABBva91TYEulUQpxC/hMMmdXRyEOEbmX7JyYxGP
kzr+aFe+3olsNTwovNuxYxWSsJ9YSFEsHCLvVN4jGtAkMaxh2d3RdsgjKrjzl/ux
GeOGbRNELSuHZW5l7ZRuh2m30nT2j1Kk+N2rzZUCgYAvsQ5CdrY35sb/8SYLuPpN
+SYUwDi25qRh2+6B7FQ9cqBeFfh8XxOh/8oP/WPTVj7x7tp0+hVNyTbS8vhQkez5
t4fejv1oXHioF++H99WQRE2ehog9aQ3j+jvfgzXDR7kwDtN70cgPLghWWlasIhw3
E1rnOKqWLrHCEMp1NCi1cQKBgHmN9JEt8xIQpwvl2orVl7kpn41Yd+VAUHo2tsAr
EfvR6ZJQ1BC2tBvaoLrXXaCv/VuDmX0qTxS8vqph/XqzssFn525w1AWO/RBLnV/s
YKO49DaQGyVyw5lP5sUfaKc9C3c+l82+uMfiVa8CmyqH5/EnzSLjdf5O560rBchZ
Fx7dAoGAJufXgW4B7H9Y3DHlQneU5T2usGDSjt6pVq1rvQTRnUkFS8jw2Dy2IMe/
KBh1bBySMyW/vqB7QIZwDwbAgigmqKMbepgw+P4JXf/aTGZcuBsgBzm4kBN+QRQ0
g8qmeYV0/DIn0JuN/8IJ3rvYF6NZUgp1nH5trNCIWstL/cwmCTk=
-----END RSA PRIVATE KEY-----
`),
	}
)

func TestOAuthBasicAuthPassword(t *testing.T) {
	expectedLogin := "username"
	expectedPassword := "password"
	expectedAuthHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(expectedLogin+":"+expectedPassword))

	testcases := map[string]struct {
		RemoteStatus  int
		RemoteHeaders http.Header
		RemoteBody    []byte

		ExpectUsername  string
		ExpectSuccess   bool
		ExpectErrStatus int32
	}{
		"success": {
			RemoteStatus:   200,
			RemoteHeaders:  http.Header{"Content-Type": []string{"application/json"}},
			RemoteBody:     []byte(`{"sub":"remoteusername"}`),
			ExpectSuccess:  true,
			ExpectUsername: "remoteusername",
		},
		"401": {
			RemoteStatus:    401,
			RemoteHeaders:   http.Header{"Content-Type": []string{"application/json"}},
			RemoteBody:      []byte(`{"error":"bad-user"}`),
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 401,
		},
		"301": {
			RemoteStatus:    301,
			RemoteHeaders:   http.Header{"Location": []string{"http://www.example.com"}},
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
		"302": {
			RemoteStatus:    302,
			RemoteHeaders:   http.Header{"Location": []string{"http://www.example.com"}},
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
		"303": {
			RemoteStatus:    303,
			RemoteHeaders:   http.Header{"Location": []string{"http://www.example.com"}},
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
		"304": {
			RemoteStatus:    304,
			RemoteHeaders:   http.Header{"Location": []string{"http://www.example.com"}},
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
		"305": {
			RemoteStatus:    305,
			RemoteHeaders:   http.Header{"Location": []string{"http://www.example.com"}},
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
		"404": {
			RemoteStatus:    404,
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
		"500": {
			RemoteStatus:    500,
			ExpectSuccess:   false,
			ExpectUsername:  "",
			ExpectErrStatus: 500,
		},
	}

	// Create tempfiles with certs and keys we're going to use
	certNames := map[string]string{}
	for certName, certContents := range basicAuthCerts {
		f, err := ioutil.TempFile("", certName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(f.Name())
		if err := ioutil.WriteFile(f.Name(), certContents, os.FileMode(0600)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		certNames[certName] = f.Name()
	}

	// Build client cert pool
	clientCAs, err := util.CertPoolFromFile(certNames[basicAuthRemoteCACert])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build remote handler
	var (
		remoteStatus  int
		remoteHeaders http.Header
		remoteBody    []byte
	)
	remoteHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			w.WriteHeader(http.StatusUnauthorized)
			t.Fatalf("Expected TLS")
		}
		if len(req.TLS.VerifiedChains) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			t.Fatalf("Expected peer cert verified by server")
		}
		if req.Header.Get("Authorization") != expectedAuthHeader {
			w.WriteHeader(http.StatusUnauthorized)
			t.Fatalf("Expected auth header %s got %s", expectedAuthHeader, req.Header.Get("Authorization"))
		}

		for k, values := range remoteHeaders {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(remoteStatus)
		w.Write(remoteBody)
	})

	// Start remote server
	remoteAddr, err := testserver.FindAvailableBindAddress(9443, 9999)
	if err != nil {
		t.Fatalf("Couldn't get free address for test server: %v", err)
	}
	remoteServer := &http.Server{
		Addr:           remoteAddr,
		Handler:        remoteHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig: crypto.SecureTLSConfig(&tls.Config{
			// RequireAndVerifyClientCert lets us limit requests to ones with a valid client certificate
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  clientCAs,
		}),
	}
	go func() {
		if err := remoteServer.ListenAndServeTLS(certNames[basicAuthRemoteServerCert], certNames[basicAuthRemoteServerKey]); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	// Build master config
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterOptions)

	masterOptions.OAuthConfig.IdentityProviders[0] = configapi.IdentityProvider{
		Name:            "basicauth",
		UseAsChallenger: true,
		UseAsLogin:      true,
		MappingMethod:   "claim",
		Provider: &configapi.BasicAuthPasswordIdentityProvider{
			RemoteConnectionInfo: configapi.RemoteConnectionInfo{
				URL: fmt.Sprintf("https://%s", remoteAddr),
				CA:  certNames[basicAuthRemoteCACert],
				ClientCert: configapi.CertInfo{
					CertFile: certNames[basicAuthClientCert],
					KeyFile:  certNames[basicAuthClientKey],
				},
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

	// Use the server and CA info
	anonConfig := restclient.Config{}
	anonConfig.Host = clientConfig.Host
	anonConfig.CAFile = clientConfig.CAFile
	anonConfig.CAData = clientConfig.CAData

	for k, tc := range testcases {
		// Specify the remote server's response
		remoteStatus = tc.RemoteStatus
		remoteHeaders = tc.RemoteHeaders
		remoteBody = tc.RemoteBody

		// Attempt to obtain a token
		accessToken, err := tokencmd.RequestToken(&anonConfig, nil, expectedLogin, expectedPassword)

		// Expected error
		if !tc.ExpectSuccess {
			if err == nil {
				t.Errorf("%s: Expected error, got token=%v", k, accessToken)
			} else if statusErr, ok := err.(*apierrs.StatusError); !ok {
				t.Errorf("%s: expected status error, got %#v", k, err)
			} else if statusErr.ErrStatus.Code != tc.ExpectErrStatus {
				t.Errorf("%s: expected error status %d, got %#v", k, tc.ExpectErrStatus, statusErr)
			}
			continue
		}

		// Expected success
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", k, err)
			continue
		}

		// Make sure we can use the token, and it represents who we expect
		userConfig := anonConfig
		userConfig.BearerToken = accessToken
		userClient, err := userclient.NewForConfig(&userConfig)
		if err != nil {
			t.Fatalf("%s: Unexpected error: %v", k, err)
		}

		user, err := userClient.Users().Get("~", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("%s: Unexpected error: %v", k, err)
		}
		if user.Name != tc.ExpectUsername {
			t.Fatalf("%s: Expected %v as the user, got %v", k, tc.ExpectUsername, user)
		}

	}

}
