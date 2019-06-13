/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/server"
	. "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/certs"
	servingcerttesting "k8s.io/apiserver/pkg/server/options/testing"
	"k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
)

func setUp(t *testing.T) Config {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	config := NewConfig(codecs)

	return *config
}

type NamedTestCertSpec struct {
	servingcerttesting.TestCertSpec
	explicitNames []string // as --tls-sni-cert-key explicit names
}

func TestGetNamedCertificateMap(t *testing.T) {
	tests := []struct {
		certs         []NamedTestCertSpec
		explicitNames []string
		expected      map[string]int // name to certs[*] index
		errorString   string
	}{
		{
			// empty certs
			expected: map[string]int{},
		},
		{
			// only one cert
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
					},
				},
			},
			expected: map[string]int{
				"test.com": 0,
			},
		},
		{
			// ips are ignored
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
						IPs:  []string{"1.2.3.4"},
					},
				},
			},
			expected: map[string]int{
				"test.com": 0,
			},
		},
		{
			// two certs with the same name
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
					},
				},
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
					},
				},
			},
			expected: map[string]int{
				"test.com": 0,
			},
		},
		{
			// two certs with different names
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test2.com",
					},
				},
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test1.com",
					},
				},
			},
			expected: map[string]int{
				"test1.com": 1,
				"test2.com": 0,
			},
		},
		{
			// two certs with the same name, explicit trumps
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
					},
				},
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
					},
					explicitNames: []string{"test.com"},
				},
			},
			expected: map[string]int{
				"test.com": 1,
			},
		},
		{
			// certs with partial overlap; ips are ignored
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host:  "a",
						Names: []string{"a.test.com", "test.com"},
					},
				},
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host:  "b",
						Names: []string{"b.test.com", "test.com"},
					},
				},
			},
			expected: map[string]int{
				"a": 0, "b": 1,
				"a.test.com": 0, "b.test.com": 1,
				"test.com": 0,
			},
		},
		{
			// wildcards
			certs: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host:  "a",
						Names: []string{"a.test.com", "test.com"},
					},
					explicitNames: []string{"*.test.com", "test.com"},
				},
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host:  "b",
						Names: []string{"b.test.com", "test.com"},
					},
					explicitNames: []string{"dev.test.com", "test.com"},
				}},
			expected: map[string]int{
				"test.com":     0,
				"*.test.com":   0,
				"dev.test.com": 1,
			},
		},
	}

NextTest:
	for i, test := range tests {
		var namedTLSCerts []NamedTLSCert
		bySignature := map[string]int{} // index in test.certs by cert signature
		for j, c := range test.certs {
			cert, err := servingcerttesting.CreateTestTLSCerts(c.TestCertSpec)
			if err != nil {
				t.Errorf("%d - failed to create cert %d: %v", i, j, err)
				continue NextTest
			}

			namedTLSCerts = append(namedTLSCerts, NamedTLSCert{
				TLSCert: cert,
				Names:   c.explicitNames,
			})

			sig, err := servingcerttesting.CertSignature(cert)
			if err != nil {
				t.Errorf("%d - failed to get signature for %d: %v", i, j, err)
				continue NextTest
			}
			bySignature[sig] = j
		}

		certMap, _, err := GetNamedCertificateMap(namedTLSCerts)
		if err == nil && len(test.errorString) != 0 {
			t.Errorf("%d - expected no error, got: %v", i, err)
		} else if err != nil && err.Error() != test.errorString {
			t.Errorf("%d - expected error %q, got: %v", i, test.errorString, err)
		} else {
			got := map[string]int{}
			for name, cert := range certMap {
				x509Certs, err := x509.ParseCertificates(cert.Certificate[0])
				assert.NoError(t, err, "%d - invalid certificate for %q", i, name)
				assert.True(t, len(x509Certs) > 0, "%d - expected at least one x509 cert in tls cert for %q", i, name)
				got[name] = bySignature[servingcerttesting.X509CertSignature(x509Certs[0])]
			}

			assert.EqualValues(t, test.expected, got, "%d - wrong certificate map", i)
		}
	}
}

func TestServerRunWithSNI(t *testing.T) {
	tests := map[string]struct {
		Cert              servingcerttesting.TestCertSpec
		SNICerts          []NamedTestCertSpec
		ExpectedCertIndex int

		// passed in the client hello info, "localhost" if unset
		ServerName string

		// optional ip or hostname to pass to NewLoopbackClientConfig
		LoopbackClientBindAddressOverride string
		ExpectLoopbackClientError         bool
	}{
		"only one cert": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
				IPs:  []string{"127.0.0.1"},
			},
			ExpectedCertIndex: -1,
		},
		"cert with multiple alternate names": {
			Cert: servingcerttesting.TestCertSpec{
				Host:  "localhost",
				Names: []string{"test.com"},
				IPs:   []string{"127.0.0.1"},
			},
			ExpectedCertIndex: -1,
			ServerName:        "test.com",
		},
		"one SNI and the default cert with the same name": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
				IPs:  []string{"127.0.0.1"},
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "localhost",
					},
				},
			},
			ExpectedCertIndex: 0,
		},
		"matching SNI cert": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
				IPs:  []string{"127.0.0.1"},
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
					},
				},
			},
			ExpectedCertIndex: 0,
			ServerName:        "test.com",
		},
		"matching IP in SNI cert and the server cert": {
			// IPs must not be passed via SNI. Hence, the ServerName in the
			// HELLO packet is empty and the server should select the non-SNI cert.
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
				IPs:  []string{"10.0.0.1", "127.0.0.1"},
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "test.com",
						IPs:  []string{"10.0.0.1"},
					},
				},
			},
			ExpectedCertIndex: -1,
			ServerName:        "10.0.0.1",
		},
		"wildcards": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
				IPs:  []string{"127.0.0.1"},
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host:  "test.com",
						Names: []string{"*.test.com"},
					},
				},
			},
			ExpectedCertIndex: 0,
			ServerName:        "www.test.com",
		},

		"loopback: LoopbackClientServerNameOverride not on any cert": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "test.com",
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "localhost",
					},
				},
			},
			ExpectedCertIndex: 0,
		},
		"loopback: LoopbackClientServerNameOverride on server cert": {
			Cert: servingcerttesting.TestCertSpec{
				Host: certs.LoopbackClientServerNameOverride,
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: "localhost",
					},
				},
			},
			ExpectedCertIndex: 0,
		},
		"loopback: LoopbackClientServerNameOverride on SNI cert": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
			},
			SNICerts: []NamedTestCertSpec{
				{
					TestCertSpec: servingcerttesting.TestCertSpec{
						Host: certs.LoopbackClientServerNameOverride,
					},
				},
			},
			ExpectedCertIndex: -1,
		},
		"loopback: bind to 0.0.0.0 => loopback uses localhost": {
			Cert: servingcerttesting.TestCertSpec{
				Host: "localhost",
			},
			ExpectedCertIndex:                 -1,
			LoopbackClientBindAddressOverride: "0.0.0.0",
		},
	}

	specToName := func(spec servingcerttesting.TestCertSpec) string {
		name := spec.Host + "_" + strings.Join(spec.Names, ",") + "_" + strings.Join(spec.IPs, ",")
		return strings.Replace(name, "*", "star", -1)
	}

	for title := range tests {
		test := tests[title]
		t.Run(title, func(t *testing.T) {
			t.Parallel()
			// create server cert
			certDir := "testdata/" + specToName(test.Cert)
			serverCertBundleFile := filepath.Join(certDir, "cert")
			serverKeyFile := filepath.Join(certDir, "key")
			err := servingcerttesting.GetOrCreateTestCertFiles(serverCertBundleFile, serverKeyFile, test.Cert)
			if err != nil {
				t.Fatalf("failed to create server cert: %v", err)
			}
			ca, err := servingcerttesting.CACertFromBundle(serverCertBundleFile)
			if err != nil {
				t.Fatalf("failed to extract ca cert from server cert bundle: %v", err)
			}
			caCerts := []*x509.Certificate{ca}

			// create SNI certs
			var namedCertKeys []cliflag.NamedCertKey
			serverSig, err := servingcerttesting.CertFileSignature(serverCertBundleFile, serverKeyFile)
			if err != nil {
				t.Fatalf("failed to get server cert signature: %v", err)
			}
			signatures := map[string]int{
				serverSig: -1,
			}
			for j, c := range test.SNICerts {
				sniDir := filepath.Join(certDir, specToName(c.TestCertSpec))
				certBundleFile := filepath.Join(sniDir, "cert")
				keyFile := filepath.Join(sniDir, "key")
				err := servingcerttesting.GetOrCreateTestCertFiles(certBundleFile, keyFile, c.TestCertSpec)
				if err != nil {
					t.Fatalf("failed to create SNI cert %d: %v", j, err)
				}

				namedCertKeys = append(namedCertKeys, cliflag.NamedCertKey{
					KeyFile:  keyFile,
					CertFile: certBundleFile,
					Names:    c.explicitNames,
				})

				ca, err := servingcerttesting.CACertFromBundle(certBundleFile)
				if err != nil {
					t.Fatalf("failed to extract ca cert from SNI cert %d: %v", j, err)
				}
				caCerts = append(caCerts, ca)

				// store index in namedCertKeys with the signature as the key
				sig, err := servingcerttesting.CertFileSignature(certBundleFile, keyFile)
				if err != nil {
					t.Fatalf("failed get SNI cert %d signature: %v", j, err)
				}
				signatures[sig] = j
			}

			stopCh := make(chan struct{})
			defer close(stopCh)

			// launch server
			config := setUp(t)

			v := fakeVersion()
			config.Version = &v

			config.EnableIndex = true
			secureOptions := (&SecureServingOptions{
				BindAddress: net.ParseIP("127.0.0.1"),
				BindPort:    6443,
				ServerCert: GeneratableKeyCert{
					CertKey: CertKey{
						CertFile: serverCertBundleFile,
						KeyFile:  serverKeyFile,
					},
				},
				SNICertKeys: namedCertKeys,
			}).WithLoopback()
			// use a random free port
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("failed to listen on 127.0.0.1:0")
			}

			secureOptions.Listener = ln
			// get port
			secureOptions.BindPort = ln.Addr().(*net.TCPAddr).Port
			config.LoopbackClientConfig = &restclient.Config{}
			if err := secureOptions.ApplyTo(&config.SecureServing, &config.LoopbackClientConfig); err != nil {
				t.Fatalf("failed applying the SecureServingOptions: %v", err)
			}

			s, err := config.Complete(nil).New("test", server.NewEmptyDelegate())
			if err != nil {
				t.Fatalf("failed creating the server: %v", err)
			}

			// add poststart hook to know when the server is up.
			startedCh := make(chan struct{})
			s.AddPostStartHookOrDie("test-notifier", func(context PostStartHookContext) error {
				close(startedCh)
				return nil
			})
			preparedServer := s.PrepareRun()
			go func() {
				if err := preparedServer.Run(stopCh); err != nil {
					t.Fatal(err)
				}
			}()

			// load ca certificates into a pool
			roots := x509.NewCertPool()
			for _, caCert := range caCerts {
				roots.AddCert(caCert)
			}

			<-startedCh

			// try to dial
			addr := fmt.Sprintf("localhost:%d", secureOptions.BindPort)
			t.Logf("Dialing %s as %q", addr, test.ServerName)
			conn, err := tls.Dial("tcp", addr, &tls.Config{
				RootCAs:    roots,
				ServerName: test.ServerName, // used for SNI in the client HELLO packet
			})
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			// check returned server certificate
			sig := servingcerttesting.X509CertSignature(conn.ConnectionState().PeerCertificates[0])
			gotCertIndex, found := signatures[sig]
			if !found {
				t.Errorf("unknown signature returned from server: %s", sig)
			}
			if gotCertIndex != test.ExpectedCertIndex {
				t.Errorf("expected cert index %d, got cert index %d", test.ExpectedCertIndex, gotCertIndex)
			}

			// check that the loopback client can connect
			host := "127.0.0.1"
			if len(test.LoopbackClientBindAddressOverride) != 0 {
				host = test.LoopbackClientBindAddressOverride
			}
			s.LoopbackClientConfig.Host = net.JoinHostPort(host, strconv.Itoa(secureOptions.BindPort))
			if test.ExpectLoopbackClientError {
				if err == nil {
					t.Fatalf("expected error creating loopback client config")
				}
				return
			}
			if err != nil {
				t.Fatalf("failed creating loopback client config: %v", err)
			}
			client, err := discovery.NewDiscoveryClientForConfig(s.LoopbackClientConfig)
			if err != nil {
				t.Fatalf("failed to create loopback client: %v", err)
			}
			got, err := client.ServerVersion()
			if err != nil {
				t.Fatalf("failed to connect with loopback client: %v", err)
			}
			if expected := &v; !reflect.DeepEqual(got, expected) {
				t.Errorf("loopback client didn't get correct version info: expected=%v got=%v", expected, got)
			}

		})
	}
}

func fakeVersion() version.Info {
	return version.Info{
		Major:        "42",
		Minor:        "42",
		GitVersion:   "42",
		GitCommit:    "34973274ccef6ab4dfaaf86599792fa9c3fe4689",
		GitTreeState: "Dirty",
	}
}
