// +build integration,etcd

package integration

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const (
	// oadm ca create-signer-cert --cert=sni-ca.crt --key=sni-ca.key --name=sni-signer --serial=sni-serial.txt
	sniCACert = "sni-ca.crt"

	// oadm ca create-server-cert --cert=sni.crt --key=sni.key --hostnames=127.0.0.1,customhost.com,*.wildcardhost.com --signer-cert=sni-ca.crt --signer-key=sni-ca.key --signer-serial=sni-serial.txt
	sniServerCert = "sni-server.crt"
	sniServerKey  = "sni-server.key"
)

func TestSNI(t *testing.T) {
	// Create tempfiles with certs and keys we're going to use
	certNames := map[string]string{}
	for certName, certContents := range sniCerts {
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

	// Build master config
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set custom cert
	masterOptions.ServingInfo.NamedCertificates = []configapi.NamedCertificate{
		{
			Names: []string{"customhost.com"},
			CertInfo: configapi.CertInfo{
				CertFile: certNames[sniServerCert],
				KeyFile:  certNames[sniServerKey],
			},
		},
		{
			Names: []string{"*.wildcardhost.com"},
			CertInfo: configapi.CertInfo{
				CertFile: certNames[sniServerCert],
				KeyFile:  certNames[sniServerKey],
			},
		},
	}

	// Start server
	_, err = testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build transports
	sniRoots, err := util.CertPoolFromFile(certNames[sniCACert])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sniConfig := &tls.Config{RootCAs: sniRoots}

	generatedRoots, err := util.CertPoolFromFile(masterOptions.ServiceAccountConfig.MasterCA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	generatedConfig := &tls.Config{RootCAs: generatedRoots}

	insecureConfig := &tls.Config{InsecureSkipVerify: true}

	tests := map[string]struct {
		Hostname   string
		TLSConfig  *tls.Config
		ExpectedOK bool
	}{
		"sni client -> generated ip": {
			Hostname:  "127.0.0.1",
			TLSConfig: sniConfig,
		},
		"sni client -> generated hostname": {
			Hostname:  "openshift",
			TLSConfig: sniConfig,
		},
		"sni client -> sni host": {
			Hostname:   "customhost.com",
			TLSConfig:  sniConfig,
			ExpectedOK: true,
		},
		"sni client -> sni wildcard host": {
			Hostname:   "www.wildcardhost.com",
			TLSConfig:  sniConfig,
			ExpectedOK: true,
		},
		"sni client -> invalid ip": {
			Hostname:  "10.10.10.10",
			TLSConfig: sniConfig,
		},
		"sni client -> invalid host": {
			Hostname:  "invalidhost.com",
			TLSConfig: sniConfig,
		},

		"generated client -> generated ip": {
			Hostname:   "127.0.0.1",
			TLSConfig:  generatedConfig,
			ExpectedOK: true,
		},
		"generated client -> generated hostname": {
			Hostname:   "openshift",
			TLSConfig:  generatedConfig,
			ExpectedOK: true,
		},
		"generated client -> sni host": {
			Hostname:  "customhost.com",
			TLSConfig: generatedConfig,
		},
		"generated client -> sni wildcard host": {
			Hostname:  "www.wildcardhost.com",
			TLSConfig: generatedConfig,
		},
		"generated client -> invalid ip": {
			Hostname:  "10.10.10.10",
			TLSConfig: generatedConfig,
		},
		"generated client -> invalid host": {
			Hostname:  "invalidhost.com",
			TLSConfig: generatedConfig,
		},

		"insecure client -> generated ip": {
			Hostname:   "127.0.0.1",
			TLSConfig:  insecureConfig,
			ExpectedOK: true,
		},
		"insecure client -> generated hostname": {
			Hostname:   "openshift",
			TLSConfig:  insecureConfig,
			ExpectedOK: true,
		},
		"insecure client -> sni host": {
			Hostname:   "customhost.com",
			TLSConfig:  insecureConfig,
			ExpectedOK: true,
		},
		"insecure client -> sni wildcard host": {
			Hostname:   "www.wildcardhost.com",
			TLSConfig:  insecureConfig,
			ExpectedOK: true,
		},
		"insecure client -> invalid ip": {
			Hostname:   "10.10.10.10",
			TLSConfig:  insecureConfig,
			ExpectedOK: true,
		},
		"insecure client -> invalid host": {
			Hostname:   "invalidhost.com",
			TLSConfig:  insecureConfig,
			ExpectedOK: true,
		},
	}

	masterPublicURL, err := url.Parse(masterOptions.MasterPublicURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for k, tc := range tests {
		u := *masterPublicURL
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		u.Path = "/healthz"

		if _, port, err := net.SplitHostPort(u.Host); err == nil {
			u.Host = net.JoinHostPort(tc.Hostname, port)
		} else {
			u.Host = tc.Hostname
		}

		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		transport := &http.Transport{
			// Custom Dial func to always dial the real master, no matter what host is asked for
			Dial: func(network, addr string) (net.Conn, error) {
				// t.Logf("%s: Dialing for %s", k, addr)
				return net.Dial(network, masterPublicURL.Host)
			},
			TLSClientConfig: tc.TLSConfig,
		}
		resp, err := transport.RoundTrip(req)
		if tc.ExpectedOK && err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if !tc.ExpectedOK && err == nil {
			t.Errorf("%s: expected error, got none", k)
			continue
		}
		if err == nil {
			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", k, err)
				continue
			}
			if string(data) != "ok" {
				t.Errorf("%s: expected %q, got %q", k, "ok", string(data))
				continue
			}
		}
	}
}

var (
	sniCerts = map[string][]byte{
		sniCACert: []byte(`-----BEGIN CERTIFICATE-----
MIICxjCCAbCgAwIBAgIBATALBgkqhkiG9w0BAQswFTETMBEGA1UEAxMKc25pLXNp
Z25lcjAgFw0xNTEwMTMwNTEyMzFaGA8yMDY1MDkzMDA1MTIzMlowFTETMBEGA1UE
AxMKc25pLXNpZ25lcjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANOb
OG1vBlq8UdJZJbumpiSEZbC7Jkyh4IceLmRvojGlMPgyoT67PedZAOD4EAbMVh+h
tAYZgcTC8ASCA1UelXFPng5OwurQv1uBdJTLiTBzPusDenWaCZc/zf2EJEM03WZ9
k4VgYecYkOmSZqvDj5Dl4lvEsVUL9RBaNMzgbzXsZE1+brlUKR+UmF2yX56vBj2R
WiQHOIQgjYLaAciHJsZVkqznoN155vPggX791l62fC5Ungil6TQTPdcizBR+deLN
S+HFxH0+YjEPiTb8PdoSUH+W3Y6d2zSzebm1GZUOKtgOQTXhmIuB3GGwOSDLC9Rq
6a1z3HTZNBMQ7Y8NOJsCAwEAAaMjMCEwDgYDVR0PAQH/BAQDAgCkMA8GA1UdEwEB
/wQFMAMBAf8wCwYJKoZIhvcNAQELA4IBAQCQ2laesgvXmT6EKXvnASbKPt35Lr26
Jp0mayAGhJgf17WzQnmN0IFyZyu0H81TdIydximxKX6KWMrTk4z/7CbUa07AwVCy
zBecwL0ajIgGakKqiiH3EJKrlg4jNKNOKboMuouNoROrwc5UkWfjSATFkjTTShDO
Qd8JrAEBtEBaXr0Xueb5rdlrR/j7UMEpjUT7bGUxnhgF/h1TJ6cIiRpVKpA8NxyL
ZBkouK3hPEeu92K7U/NBBE//YRQz6EghixQSv/ZEGmlsU8z6g8ay+d9iZa5DhYsh
/IYxG0ykvGUH9d1AWplmHAqPwrcWSym49cZEmiHx/tO/wRp6+51lyfcn
-----END CERTIFICATE-----
`),

		sniServerCert: []byte(`-----BEGIN CERTIFICATE-----
MIIDIDCCAgqgAwIBAgIBAzALBgkqhkiG9w0BAQswFTETMBEGA1UEAxMKc25pLXNp
Z25lcjAgFw0xNTEwMTMwNTMzMTVaGA8yMDY1MDkzMDA1MzMxNlowHTEbMBkGA1UE
AxMSKi53aWxkY2FyZGhvc3QuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEA2hh1F9DXwkCDLi20i4ehoL40DhHFPsVfHa4/nIvmcOgJE6qZCh2oH+H8
vDLQg4wNNpa47le5AuYqupXcXeTtrGq/AwXSnrC3LVJYqleAnVgLoL5+Y2NEj8Hx
fzdWGhkCMDtA1QdZeq8HpCv3ZziRUxiZ/ddI4rZjFsCDoZUAhGGDzHCqkKbsuBI8
bNkz9V0FfXn8OprfRCUPtMJrHusNsWCHrQ4ceRYOzsa9y4IVQnmvcpMlh7qu7kiO
AbbFm41J8W/BEMxIBwJOITB2qAgkOKxF48IpsDeCWbOJ2qedisR5PNl/te4Qv/Kt
D6FTqpIHv/cSkn9fz2ji6Jy574oR7wIDAQABo3UwczAOBgNVHQ8BAf8EBAMCAKAw
EwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYDVR0TAQH/BAIwADA+BgNVHREENzA1ghIq
LndpbGRjYXJkaG9zdC5jb22CDmN1c3RvbWhvc3QuY29tggkxMjcuMC4wLjGHBH8A
AAEwCwYJKoZIhvcNAQELA4IBAQB5Q7Pbx9jBP566XgjQGnpoNK5jJf3CqkjBKSdG
OdoumjDEx2Ast3h1edXBVnU0DPxbfxMo3lBIAiJ+sNWGErYlIDVdpglFVyYIn+V6
71gUDaA+1rXBS3f0QEQ9pOh3b4qSWbmYr9mbYRQus1cFYq+KTsLmzuNGvRwSvvE7
5nSTgUozXTF4fSyWGGTcy13ZFg6mLlMoivjVswUJsi2nLf/yejnwdJGs4ZR06qCx
dYB2LaUCbI6AQlaQMVrxsVXTfUkstKF5cuPKq/MenBcH88j3uqj8BF6YjR1wRfHc
p0NuWIRsuXBHEr6o3iQ3KlmQLWfLeS0K+FkN60mwDTteIzuR
-----END CERTIFICATE-----
`),

		sniServerKey: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA2hh1F9DXwkCDLi20i4ehoL40DhHFPsVfHa4/nIvmcOgJE6qZ
Ch2oH+H8vDLQg4wNNpa47le5AuYqupXcXeTtrGq/AwXSnrC3LVJYqleAnVgLoL5+
Y2NEj8HxfzdWGhkCMDtA1QdZeq8HpCv3ZziRUxiZ/ddI4rZjFsCDoZUAhGGDzHCq
kKbsuBI8bNkz9V0FfXn8OprfRCUPtMJrHusNsWCHrQ4ceRYOzsa9y4IVQnmvcpMl
h7qu7kiOAbbFm41J8W/BEMxIBwJOITB2qAgkOKxF48IpsDeCWbOJ2qedisR5PNl/
te4Qv/KtD6FTqpIHv/cSkn9fz2ji6Jy574oR7wIDAQABAoIBAQDTq2UJpkGhYGdw
zB8sRIjTr4ZqGUkscPatoc5PK2COOEWG9s3tiXcA6p4WMeM5qRWx43q8qBsB+02B
Ja1o26To7/lO/7m5Fp3RuNghCyfije9LJVcZMuD5/StbYuOIFLmRAhEcMDPh5Dow
VhOZ9MbmtTvPp8AveQCWtmWKz0hfMVLGUP91yqbXD/7h7/VpN+kKEOwoOUFqBshZ
ziJs261VnI1UYeDg6jokZTDD9t1vS0EbPoe8NozPt8zF4bihXpLX7MCjJO32ISQl
4cjH+94Wl1/X2MerG0RA8BTMlMSACiQdyeE7C4d/4tfRFp2EHqSBgwsTMLAJKy1P
1NjejLmxAoGBAOxx1i22W/J1tqNvO7we3G8nuojRG5f69u1YKrW/yhk89a3hmyQ6
MB15xz55CyTEl+FqPQReMJlmBsbKqHl8viOWhRc12t4i/JsYgroCcbIHME86FXuH
boNJo0DB0Q1xoCQEuNiygyv3qLq5tKPBk0lAn2Sxhyt398MDukOcZ8ujAoGBAOwi
HuM/d6A78F6l1NHJdOoTFvLXCbM3mWSdfoU/UxQmEk9Wr3kfudOu2ZAsWfiznjw4
jgN/JS/WTY+NvVEXMIASz3QoNmLFSN0c5DCBSBVW/1B9rFP9GLCDrZMspluYg/gR
8MLxC6AAVBcbNZj7Z16mmAyUs3LCJmP9P/7GcgVFAoGANesrvVbllt/zG0gFZjvf
ZtW3evW8hibr4moFq1amHqVBHTriZxuB12bq4bs2qFbQj83rRjC4gnK6vuB+FN42
eeUcSpO0ao2t7yxiu0pNZRywjpCfT4Et2XCUcvL/2kH8E9qj0H683OzoJFSu9dzx
2nWLI6o8OdRswqL5+esT3GMCgYEApYnmDXnY60QZ5sBqygdpJw/q7qNB8Znwt1CR
+efC3kUyYNxsd4V+SKAzdZciG/AP5jfflyPzde3Oweyj481V+vM07EGknumfgyNV
9YssdYlfw5XW0aqFPHmTnbGXjm8FVUt+dat2ctzIFsrEcFMOzJQN1AQLKVBiiYZo
7rtAA+ECgYEAoR685udqJrCnvhL911cT+7/DrUyNLFmYvoIMlfG9DRXmKTIlyFH9
A+TGxO02VaWYxm/zFTNIkXsEpNrxW4CVZtWM6biktT20S0p11IA1x8SCGqOimlF8
Yg4OZRDUUYRAl0MUaslTyIxBfEVal4XgvBwhjXk0BMP6OJNDHWT3mUY=
-----END RSA PRIVATE KEY-----
`),
	}
)
