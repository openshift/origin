package admin

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/cert"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type VerifyClientCertOptions struct {
	// CAFile is the bundle of CA certs to verify against. Optional.
	CAFile         string
	CertFile       string
	PrivateKeyFile string
	CSRFile        string
	User           string
	Groups         []string
}

func (o *VerifyClientCertOptions) VerifyClientCert() error {
	certs, err := getVerifiedCerts(o.CertFile, o.PrivateKeyFile, o.CSRFile, o.CAFile)
	if err != nil {
		return err
	}

	if !hasUsage(certs[0], x509.ExtKeyUsageClientAuth) {
		return fmt.Errorf("certificate %s does not have ExtKeyUsage of TLS Client Authentication", o.CertFile)
	}

	if certs[0].Subject.CommonName != o.User {
		return fmt.Errorf("certificate %s does not have CommonName=%q", o.CertFile, o.User)
	}
	missingGroups := sets.NewString(o.Groups...).Difference(sets.NewString(certs[0].Subject.Organization...)).List()
	if len(missingGroups) > 0 {
		return fmt.Errorf("certificate %s does not have all the expected groups in its Organization subject component (expected %q, missing %q)", o.CertFile, o.Groups, missingGroups)
	}

	return nil
}

type VerifyServerCertOptions struct {
	// CAFile is the bundle of CA certs to verify against. Optional.
	CAFile         string
	CertFile       string
	PrivateKeyFile string
	CSRFile        string
	Hostnames      []string
}

func (o *VerifyServerCertOptions) VerifyServerCert() error {
	certs, err := getVerifiedCerts(o.CertFile, o.PrivateKeyFile, o.CSRFile, o.CAFile)
	if err != nil {
		return err
	}

	if !hasUsage(certs[0], x509.ExtKeyUsageServerAuth) {
		return fmt.Errorf("certificate %s does not have ExtKeyUsage of TLS Server Authentication", o.CertFile)
	}

	// SANs (tolerate extra)
	ips, dns := crypto.IPAddressesDNSNames(o.Hostnames)
	missingSANs := sets.NewString()
	for _, needIP := range ips {
		found := false
		for _, hasIP := range certs[0].IPAddresses {
			if hasIP.Equal(needIP) {
				found = true
				break
			}
		}
		if !found {
			missingSANs.Insert("IP:" + needIP.String())
		}
	}
	for _, missingDNS := range sets.NewString(dns...).Difference(sets.NewString(certs[0].DNSNames...)).List() {
		missingSANs.Insert("DNS:" + missingDNS)
	}
	if len(missingSANs) > 0 {
		return fmt.Errorf("certificate %s is missing required subjectAltNames: %v", o.CertFile, missingSANs.List())
	}

	return nil
}

func getVerifiedCerts(certFile, keyFile, csrFile, caFile string) ([]*x509.Certificate, error) {
	certs, err := cert.CertsFromFile(certFile)
	if err != nil {
		return nil, err
	}

	if (certs[0].KeyUsage & x509.KeyUsageDigitalSignature) == 0 {
		return nil, fmt.Errorf("certificate %s does not allow digital signature key usage", certFile)
	}
	if (certs[0].KeyUsage & x509.KeyUsageKeyEncipherment) == 0 {
		return nil, fmt.Errorf("certificate %s does not allow key encipherment key usage", certFile)
	}
	if certs[0].NotBefore.After(time.Now()) || certs[0].NotAfter.Before(time.Now()) {
		return nil, fmt.Errorf("certificate %s is only valid between %v and %v", certFile, certs[0].NotBefore, certs[0].NotAfter)
	}

	// Signature validates with CA bundle, if CAFile is set
	if len(caFile) > 0 {
		rootPool, err := cert.NewPool(caFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load CA bundle from %s: %v", caFile, err)
		}

		intermediatePool := x509.NewCertPool()
		for _, intermediateCert := range certs[1:] {
			intermediatePool.AddCert(intermediateCert)
		}

		_, err = certs[0].Verify(x509.VerifyOptions{
			Intermediates: intermediatePool,
			Roots:         rootPool,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		})
		if err != nil {
			return nil, fmt.Errorf("certificate %s could not be verified against CA bundle %s: %v", certFile, caFile, err)
		}
	}

	// Verify with key file if keyfile exists
	if _, err := os.Stat(keyFile); err == nil {
		if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
			return nil, err
		}
	}

	// Verify cert matches key in CSR if CSR exists
	if req, err := CSRFromFile(csrFile); err == nil {
		if !reflect.DeepEqual(req.PublicKey, certs[0].PublicKey) || !reflect.DeepEqual(req.PublicKeyAlgorithm, certs[0].PublicKeyAlgorithm) {
			return nil, fmt.Errorf("certificate %s does not match the public key or public key algorithm of csr %s", certFile, csrFile)
		}
	}

	return certs, nil
}

func hasUsage(cert *x509.Certificate, usage x509.ExtKeyUsage) bool {
	for _, u := range cert.ExtKeyUsage {
		if usage == u {
			return true
		}
	}
	return false
}
