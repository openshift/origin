package crypto

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"testing"
)

func TestCrypto(t *testing.T) {
	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()

	// Test CA
	fmt.Println("Building CA...")
	caKey, caCrt := buildCA(t)
	roots.AddCert(caCrt)

	// Test intermediate
	fmt.Println("Building intermediate 1...")
	intKey, intCrt := buildIntermediate(t, caKey, caCrt)
	verify(t, intCrt, x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
	}, true, 2)
	intermediates.AddCert(intCrt)

	// Test intermediate 2
	fmt.Println("Building intermediate 2...")
	intKey2, intCrt2 := buildIntermediate(t, intKey, intCrt)
	verify(t, intCrt2, x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
	}, true, 3)
	intermediates.AddCert(intCrt2)

	// Test server cert
	fmt.Println("Building server...")
	_, serverCrt := buildServer(t, intKey2, intCrt2)
	verify(t, serverCrt, x509.VerifyOptions{
		DNSName:       "localhost",
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, true, 4)
	verify(t, serverCrt, x509.VerifyOptions{
		DNSName:       "www.example.com",
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, true, 4)
	verify(t, serverCrt, x509.VerifyOptions{
		DNSName:       "127.0.0.1",
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, true, 4)
	verify(t, serverCrt, x509.VerifyOptions{
		DNSName:       "www.foo.com",
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, false, 4)

	// Test client cert
	fmt.Println("Building client...")
	_, clientCrt := buildClient(t, intKey2, intCrt2)
	verify(t, clientCrt, x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, true, 4)
}

func buildCA(t *testing.T) (crypto.PrivateKey, *x509.Certificate) {
	caPublicKey, caPrivateKey, err := NewKeyPair()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	caTemplate, err := newSigningCertificateTemplate(pkix.Name{CommonName: "CA"})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	caCrt, err := signCertificate(caTemplate, caPublicKey, caTemplate, caPrivateKey)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	return caPrivateKey, caCrt
}

func buildIntermediate(t *testing.T, signingKey crypto.PrivateKey, signingCrt *x509.Certificate) (crypto.PrivateKey, *x509.Certificate) {
	intermediatePublicKey, intermediatePrivateKey, err := NewKeyPair()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	intermediateTemplate, err := newSigningCertificateTemplate(pkix.Name{CommonName: "Intermediate"})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	intermediateCrt, err := signCertificate(intermediateTemplate, intermediatePublicKey, signingCrt, signingKey)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if err := intermediateCrt.CheckSignatureFrom(signingCrt); err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	return intermediatePrivateKey, intermediateCrt
}

func buildServer(t *testing.T, signingKey crypto.PrivateKey, signingCrt *x509.Certificate) (crypto.PrivateKey, *x509.Certificate) {
	serverPublicKey, serverPrivateKey, err := NewKeyPair()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	serverTemplate, err := newServerCertificateTemplate(pkix.Name{CommonName: "Server"}, []string{"127.0.0.1", "localhost", "www.example.com"})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	serverCrt, err := signCertificate(serverTemplate, serverPublicKey, signingCrt, signingKey)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if err := serverCrt.CheckSignatureFrom(signingCrt); err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	return serverPrivateKey, serverCrt
}

func buildClient(t *testing.T, signingKey crypto.PrivateKey, signingCrt *x509.Certificate) (crypto.PrivateKey, *x509.Certificate) {
	clientPublicKey, clientPrivateKey, err := NewKeyPair()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	clientTemplate, err := newClientCertificateTemplate(pkix.Name{CommonName: "Client"})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	clientCrt, err := signCertificate(clientTemplate, clientPublicKey, signingCrt, signingKey)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if err := clientCrt.CheckSignatureFrom(signingCrt); err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	return clientPrivateKey, clientCrt
}

func verify(t *testing.T, cert *x509.Certificate, opts x509.VerifyOptions, success bool, chainLength int) {
	validChains, err := cert.Verify(opts)
	if success {
		if err != nil {
			t.Fatalf("Unexpected error: %#v", err)
		}
		if len(validChains) != 1 {
			t.Fatalf("Expected a valid chain")
		}
		if len(validChains[0]) != chainLength {
			t.Fatalf("Expected a valid chain of length %d, got %d", chainLength, len(validChains[0]))
		}
	} else if err == nil && len(validChains) > 0 {
		t.Fatalf("Expected failure, got success")
	}
}

func TestRandomSerialGenerator(t *testing.T) {
	generator := &RandomSerialGenerator{}

	hostnames := []string{"foo", "bar"}
	template, err := newServerCertificateTemplate(pkix.Name{CommonName: hostnames[0]}, hostnames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := generator.Next(template); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
