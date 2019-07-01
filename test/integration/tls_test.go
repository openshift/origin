package integration

import (
	"crypto/tls"
	"testing"

	"github.com/openshift/library-go/pkg/crypto"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestTLSOverrides(t *testing.T) {
	master, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, master)

	// Pick these ciphers because the first is http2 compatible, and the second works with TLS10
	master.ServingInfo.MinTLSVersion = "VersionTLS10"
	master.ServingInfo.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_RSA_WITH_AES_128_CBC_SHA"}

	_, err = testserver.StartConfiguredMaster(master)
	if err != nil {
		t.Fatal(err)
	}

	// Verify we work with all TLS versions
	for _, tlsVersionName := range crypto.ValidTLSVersions() {
		tlsVersion := crypto.TLSVersionOrDie(tlsVersionName)
		expectSuccess := true
		config := &tls.Config{MinVersion: tlsVersion, MaxVersion: tlsVersion, InsecureSkipVerify: true}

		{
			conn, err := tls.Dial(master.ServingInfo.BindNetwork, master.ServingInfo.BindAddress, config)
			if err == nil {
				conn.Close()
			}
			if success := err == nil; success != expectSuccess {
				t.Errorf("Expected success %v, got %v with TLS version %s dialing master", expectSuccess, success, tlsVersionName)
			}
		}
	}

	// Verify the only ciphers we work with are the ones we chose
	defaultCiphers := map[uint16]bool{}
	for _, defaultCipher := range crypto.DefaultCiphers() {
		defaultCiphers[defaultCipher] = true
	}
	for _, cipherName := range crypto.ValidCipherSuites() {
		cipher, err := crypto.CipherSuite(cipherName)
		if err != nil {
			t.Fatal(err)
		}
		expectFailure := true
		switch cipher {
		case tls.TLS_RSA_WITH_AES_128_CBC_SHA:
			expectFailure = false
		case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
			expectFailure = false
		default:
			expectFailure = true
		}
		config := &tls.Config{CipherSuites: []uint16{cipher}, InsecureSkipVerify: true}

		{
			conn, err := tls.Dial(master.ServingInfo.BindNetwork, master.ServingInfo.BindAddress, config)
			if err == nil {
				conn.Close()
				if expectFailure {
					t.Errorf("Expected failure on cipher %s, got success dialing master", cipherName)
				}
			}
		}
	}
}
