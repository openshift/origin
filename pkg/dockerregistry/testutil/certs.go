package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

type KeyPair struct {
	Certificate *x509.Certificate
	PrivateKey  interface{}
}

type CertificateOptions struct {
	Subject     pkix.Name
	ValidFor    time.Duration
	IsCA        bool
	DNSNames    []string
	IPAddresses []net.IP
}

func GenerateRSAKeyPair(rsaBits int, parent *KeyPair, opts CertificateOptions) (*KeyPair, error) {
	fail := func(format string, args ...interface{}) (*KeyPair, error) {
		return nil, fmt.Errorf(format, args)
	}

	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return fail("failed to generate private key: %s", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(opts.ValidFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fail("failed to generate serial number: %s", err)
	}

	keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if opts.IsCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      opts.Subject,
		NotBefore:    notBefore,
		NotAfter:     notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA: opts.IsCA,

		DNSNames:    opts.DNSNames,
		IPAddresses: opts.IPAddresses,
	}

	var parentCert *x509.Certificate
	var parentPriv interface{}
	if parent != nil {
		parentCert = parent.Certificate
		parentPriv = parent.PrivateKey
	} else {
		parentCert = &template
		parentPriv = priv
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, parentCert, &priv.PublicKey, parentPriv)
	if err != nil {
		return fail("failed to create certificate: %s", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return fail("failed to parse generated certificate: %s", err)
	}

	return &KeyPair{
		Certificate: cert,
		PrivateKey:  priv,
	}, nil
}

/*
func main() {
	rootKeyPair, err := GenerateRSAKeyPair(1024, nil, CertificateOptions{
		Subject: pkix.Name{
			Organization: []string{"OpenShift Test Suite"},
		},
		ValidFor: 24 * time.Hour,
		IsCA:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	keyPair, err := GenerateRSAKeyPair(1024, rootKeyPair, CertificateOptions{
		Subject: pkix.Name{
			Organization: []string{"Server Certificate"},
			CommonName:   "localhost",
		},
		ValidFor:    12 * time.Hour,
		IPAddresses: []net.IP{net.ParseIP("10.34.129.210")},
	})
	if err != nil {
		log.Fatal(err)
	}

	writePEM := func(filename string, typ string, bytes []byte) {
		f, err := os.Create(filename)
		if err != nil {
			log.Fatalf("failed to open %s for writing: %s", filename, err)
		}
		defer f.Close()
		pem.Encode(f, &pem.Block{Type: typ, Bytes: bytes})
	}
	writePEM("./ca.crt", "CERTIFICATE", rootKeyPair.Certificate.Raw)
	writePEM("./client.cert", "CERTIFICATE", keyPair.Certificate.Raw)
	writePEM("./client.key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(keyPair.PrivateKey.(*rsa.PrivateKey)))

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(rootKeyPair.Certificate)

	config := &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{keyPair.Certificate.Raw},
			PrivateKey:  keyPair.PrivateKey,
		}},
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: config,
	}

	log.Fatal(server.ListenAndServeTLS("", ""))
}
*/
