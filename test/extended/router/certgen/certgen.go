package certgen

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// MarshalPrivateKeyToDERFormat converts the key to a string
// representation (SEC 1, ASN.1 DER form) suitable for dropping into a
// route's TLS key stanza.
func MarshalPrivateKeyToDERFormat(key *ecdsa.PrivateKey) (string, error) {
	data, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %v", err)
	}

	buf := &bytes.Buffer{}

	if err := pem.Encode(buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: data}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// MarshalCertToPEMString encodes derBytes to a PEM format suitable
// for dropping into a route's TLS certificate stanza.
func MarshalCertToPEMString(derBytes []byte) (string, error) {
	buf := &bytes.Buffer{}

	if err := pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", fmt.Errorf("failed to encode cert data: %v", err)
	}

	return buf.String(), nil
}

// GenerateKeyPair creates root CA, certificate and key with optional
// hosts. Certificate is valid from notBefore and expires after
// notAfter.
func GenerateKeyPair(rootCertCN string, notBefore, notAfter time.Time, hosts ...string) ([]byte, []byte, *ecdsa.PrivateKey, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate serial number: %v", err)
	}

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ECDSA key: %v", err)
	}

	rootTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Red Hat"},
			CommonName:   rootCertCN,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	rootDerBytes, err := x509.CreateCertificate(rand.Reader, &rootTemplate, &rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create root certificate: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ECDSA key: %v", err)
	}

	serialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate serial number: %v", err)
	}

	leafCertTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Red Hat"},
			CommonName:   "test_cert",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			leafCertTemplate.IPAddresses = append(leafCertTemplate.IPAddresses, ip)
		} else {
			leafCertTemplate.DNSNames = append(leafCertTemplate.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &leafCertTemplate, &rootTemplate, &leafKey.PublicKey, rootKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create leaf certificate: %v", err)
	}

	return rootDerBytes, derBytes, leafKey, nil
}
