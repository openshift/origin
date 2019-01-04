package crypto

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"k8s.io/client-go/util/cert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateCertificates(t *testing.T) {
	c, err := newTestCACertificate(pkix.Name{CommonName: "test"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
	if err != nil {
		t.Fatal(err)
	}

	if len(c.Config.Certs) != 1 {
		t.Fatalf("expected 1 certificate in the chain, but got %d", len(c.Config.Certs))
	}

	validCerts := FilterExpiredCerts(c.Config.Certs...)
	if len(validCerts) != 1 {
		t.Fatalf("expected 1 valid certificate in the chain, but got %d", len(validCerts))
	}
}

func TestValidateCertificatesExpired(t *testing.T) {
	certBytes, err := ioutil.ReadFile("./testfiles/tls-expired.crt")
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	certs, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		t.Fatal(err)
	}

	newCert, err := newTestCACertificate(pkix.Name{CommonName: "etcdproxy-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	certs = append(certs, newCert.Config.Certs...)

	if len(certs) != 2 {
		t.Fatalf("expected 2 certificate in the chain, but got %d", len(certs))
	}

	validCerts := FilterExpiredCerts(certs...)
	if len(validCerts) != 1 {
		t.Fatalf("expected 1 valid certificate in the chain, but got %d", len(validCerts))
	}
}

// NewCACertificate generates and signs new CA certificate and key.
func newTestCACertificate(subject pkix.Name, serialNumber int64, validity metav1.Duration, currentTime func() time.Time) (*CA, error) {
	caPublicKey, caPrivateKey, err := NewKeyPair()
	if err != nil {
		return nil, err
	}

	caCert := &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    currentTime().Add(-1 * time.Second),
		NotAfter:     currentTime().Add(validity.Duration),
		SerialNumber: big.NewInt(serialNumber),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA: true,
	}

	cert, err := signCertificate(caCert, caPublicKey, caCert, caPrivateKey)
	if err != nil {
		return nil, err
	}

	return &CA{
		Config: &TLSCertificateConfig{
			Certs: []*x509.Certificate{cert},
			Key:   caPrivateKey,
		},
	}, nil
}
