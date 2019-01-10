package crypto

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"go/importer"
	"strings"
	"testing"
	"time"
)

const certificateLifetime = 365 * 2

func TestConstantMaps(t *testing.T) {
	pkg, err := importer.Default().Import("crypto/tls")
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		return
	}
	discoveredVersions := map[string]bool{}
	discoveredCiphers := map[string]bool{}
	for _, declName := range pkg.Scope().Names() {
		if strings.HasPrefix(declName, "VersionTLS") {
			discoveredVersions[declName] = true
		}
		if strings.HasPrefix(declName, "TLS_RSA_") || strings.HasPrefix(declName, "TLS_ECDHE_") {
			discoveredCiphers[declName] = true
		}
	}

	for k := range discoveredCiphers {
		if _, ok := ciphers[k]; !ok {
			t.Errorf("discovered cipher tls.%s not in ciphers map", k)
		}
	}
	for k := range ciphers {
		if _, ok := discoveredCiphers[k]; !ok {
			t.Errorf("ciphers map has %s not in tls package", k)
		}
	}

	for k := range discoveredVersions {
		if _, ok := versions[k]; !ok {
			t.Errorf("discovered version tls.%s not in version map", k)
		}
	}
	for k := range versions {
		if _, ok := discoveredVersions[k]; !ok {
			t.Errorf("versions map has %s not in tls package", k)
		}
	}
}

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

// Can be used for CA or intermediate signing certs
func newSigningCertificateTemplate(subject pkix.Name, expireDays int, currentTime func() time.Time) *x509.Certificate {
	var caLifetimeInDays = DefaultCACertificateLifetimeInDays
	if expireDays > 0 {
		caLifetimeInDays = expireDays
	}

	if caLifetimeInDays > DefaultCACertificateLifetimeInDays {
		warnAboutCertificateLifeTime(subject.CommonName, DefaultCACertificateLifetimeInDays)
	}

	caLifetime := time.Duration(caLifetimeInDays) * 24 * time.Hour

	return newSigningCertificateTemplateForDuration(subject, caLifetime, currentTime)
}

func buildCA(t *testing.T) (crypto.PrivateKey, *x509.Certificate) {
	caPublicKey, caPrivateKey, err := NewKeyPair()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	caTemplate := newSigningCertificateTemplate(pkix.Name{CommonName: "CA"}, certificateLifetime, time.Now)
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
	intermediateTemplate := newSigningCertificateTemplate(pkix.Name{CommonName: "Intermediate"}, certificateLifetime, time.Now)
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
	hosts := []string{"127.0.0.1", "localhost", "www.example.com"}
	serverTemplate := newServerCertificateTemplate(pkix.Name{CommonName: "Server"}, hosts, certificateLifetime, time.Now)
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
	clientTemplate := newClientCertificateTemplate(pkix.Name{CommonName: "Client"}, certificateLifetime, time.Now)
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
	template := newServerCertificateTemplate(pkix.Name{CommonName: hostnames[0]}, hostnames, certificateLifetime, time.Now)
	if _, err := generator.Next(template); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidityPeriodOfClientCertificate(t *testing.T) {
	currentTime := time.Now()

	currentFakeTime := func() time.Time {
		return currentTime
	}

	tests := []struct {
		passedExpireDays int
		realExpireDays   int
	}{
		{
			passedExpireDays: 100,
			realExpireDays:   100,
		},
		{
			passedExpireDays: 0,
			realExpireDays:   DefaultCertificateLifetimeInDays,
		},
		{
			passedExpireDays: -1,
			realExpireDays:   DefaultCertificateLifetimeInDays,
		},
	}

	for _, test := range tests {
		cert := newClientCertificateTemplate(pkix.Name{CommonName: "client"}, test.passedExpireDays, currentFakeTime)
		expirationDate := cert.NotAfter
		expectedExpirationDate := currentTime.Add(time.Duration(test.realExpireDays) * 24 * time.Hour)
		if expectedExpirationDate != expirationDate {
			t.Errorf("expected that client certificate will expire at %v but found %v", expectedExpirationDate, expirationDate)
		}
	}
}

func TestValidityPeriodOfServerCertificate(t *testing.T) {
	currentTime := time.Now()

	currentFakeTime := func() time.Time {
		return currentTime
	}

	tests := []struct {
		passedExpireDays int
		realExpireDays   int
	}{
		{
			passedExpireDays: 100,
			realExpireDays:   100,
		},
		{
			passedExpireDays: 0,
			realExpireDays:   DefaultCertificateLifetimeInDays,
		},
		{
			passedExpireDays: -1,
			realExpireDays:   DefaultCertificateLifetimeInDays,
		},
	}

	for _, test := range tests {
		cert := newServerCertificateTemplate(
			pkix.Name{CommonName: "server"},
			[]string{"www.example.com"},
			test.passedExpireDays,
			currentFakeTime,
		)
		expirationDate := cert.NotAfter
		expectedExpirationDate := currentTime.Add(time.Duration(test.realExpireDays) * 24 * time.Hour)
		if expectedExpirationDate != expirationDate {
			t.Errorf("expected that server certificate will expire at %v but found %v", expectedExpirationDate, expirationDate)
		}
	}
}

func TestValidityPeriodOfSigningCertificate(t *testing.T) {
	currentTime := time.Now()

	currentFakeTime := func() time.Time {
		return currentTime
	}

	tests := []struct {
		passedExpireDays int
		realExpireDays   int
	}{
		{
			passedExpireDays: 100,
			realExpireDays:   100,
		},
		{
			passedExpireDays: 0,
			realExpireDays:   DefaultCACertificateLifetimeInDays,
		},
		{
			passedExpireDays: -1,
			realExpireDays:   DefaultCACertificateLifetimeInDays,
		},
	}

	for _, test := range tests {
		cert := newSigningCertificateTemplate(pkix.Name{CommonName: "CA"}, test.passedExpireDays, currentFakeTime)
		expirationDate := cert.NotAfter
		expectedExpirationDate := currentTime.Add(time.Duration(test.realExpireDays) * 24 * time.Hour)
		if expectedExpirationDate != expirationDate {
			t.Errorf("expected that CA certificate will expire at %v but found %v", expectedExpirationDate, expirationDate)
		}
	}
}
