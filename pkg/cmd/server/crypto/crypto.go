package crypto

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// SecureTLSConfig enforces the default minimum security settings for the
// cluster.
// TODO: allow override
func SecureTLSConfig(config *tls.Config) *tls.Config {
	// Recommendations from https://wiki.mozilla.org/Security/Server_Side_TLS
	// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
	config.MinVersion = tls.VersionTLS10
	// In a legacy environment, allow cipher control to be disabled.
	if len(os.Getenv("OPENSHIFT_ALLOW_DANGEROUS_TLS_CIPHER_SUITES")) == 0 {
		config.PreferServerCipherSuites = true
		config.CipherSuites = []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		}
	} else {
		glog.Warningf("Potentially insecure TLS cipher suites are allowed in client connections because environment variable OPENSHIFT_ALLOW_DANGEROUS_TLS_CIPHER_SUITES is set")
	}
	return config
}

type TLSCertificateConfig struct {
	Certs []*x509.Certificate
	Key   crypto.PrivateKey
}

type TLSCARoots struct {
	Roots []*x509.Certificate
}

func (c *TLSCertificateConfig) writeCertConfig(certFile, keyFile string) error {
	if err := writeCertificates(certFile, c.Certs...); err != nil {
		return err
	}
	if err := writeKeyFile(keyFile, c.Key); err != nil {
		return err
	}
	return nil
}
func (c *TLSCARoots) writeCARoots(rootFile string) error {
	if err := writeCertificates(rootFile, c.Roots...); err != nil {
		return err
	}
	return nil
}

func GetTLSCARoots(caFile string) (*TLSCARoots, error) {
	if len(caFile) == 0 {
		return nil, errors.New("caFile missing")
	}

	caPEMBlock, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	roots, err := cmdutil.CertificatesFromPEM(caPEMBlock)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s", caFile, err)
	}

	return &TLSCARoots{roots}, nil
}

func GetTLSCertificateConfig(certFile, keyFile string) (*TLSCertificateConfig, error) {
	if len(certFile) == 0 {
		return nil, errors.New("certFile missing")
	}
	if len(keyFile) == 0 {
		return nil, errors.New("keyFile missing")
	}

	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	certs, err := cmdutil.CertificatesFromPEM(certPEMBlock)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s", certFile, err)
	}

	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	keyPairCert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}
	key := keyPairCert.PrivateKey

	return &TLSCertificateConfig{certs, key}, nil
}

var (
	// Default ca certs to be long-lived
	caLifetime = time.Hour * 24 * 365 * 5

	// Default templates to last for two years
	lifetime = time.Hour * 24 * 365 * 2

	// Default keys are 2048 bits
	keyBits = 2048
)

type CA struct {
	SerialFile string
	Config     *TLSCertificateConfig

	// lock guards access to the Serial field
	lock   sync.Mutex
	Serial int64
}

// EnsureCA returns a CA, whether it was created (as opposed to pre-existing), and any error
func EnsureCA(certFile, keyFile, serialFile, name string) (*CA, bool, error) {
	if ca, err := GetCA(certFile, keyFile, serialFile); err == nil {
		return ca, false, err
	}
	ca, err := MakeCA(certFile, keyFile, serialFile, name)
	return ca, true, err
}

func GetCA(certFile, keyFile, serialFile string) (*CA, error) {
	caConfig, err := GetTLSCertificateConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	// read serial file
	var serial int64
	if serialData, err := ioutil.ReadFile(serialFile); err == nil {
		serial, _ = strconv.ParseInt(string(serialData), 10, 64)
	} else {
		return nil, err
	}
	if serial < 1 {
		serial = 1
	}

	return &CA{
		Serial:     serial,
		SerialFile: serialFile,
		Config:     caConfig,
	}, nil
}

func MakeCA(certFile, keyFile, serialFile, name string) (*CA, error) {
	glog.V(2).Infof("Generating new CA for %s cert, and key in %s, %s", name, certFile, keyFile)
	// Create CA cert
	rootcaPublicKey, rootcaPrivateKey, err := NewKeyPair()
	if err != nil {
		return nil, err
	}
	rootcaTemplate, err := newSigningCertificateTemplate(pkix.Name{CommonName: name})
	if err != nil {
		return nil, err
	}
	rootcaCert, err := signCertificate(rootcaTemplate, rootcaPublicKey, rootcaTemplate, rootcaPrivateKey)
	if err != nil {
		return nil, err
	}
	caConfig := &TLSCertificateConfig{
		Certs: []*x509.Certificate{rootcaCert},
		Key:   rootcaPrivateKey,
	}
	if err := caConfig.writeCertConfig(certFile, keyFile); err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(serialFile, []byte("0"), 0644); err != nil {
		return nil, err
	}

	return &CA{
		Serial:     0,
		SerialFile: serialFile,
		Config:     caConfig,
	}, nil
}

func (ca *CA) EnsureServerCert(certFile, keyFile string, hostnames sets.String) (*TLSCertificateConfig, bool, error) {
	certConfig, err := GetServerCert(certFile, keyFile, hostnames)
	if err != nil {
		certConfig, err = ca.MakeServerCert(certFile, keyFile, hostnames)
		return certConfig, true, err
	}

	return certConfig, false, nil
}

func GetServerCert(certFile, keyFile string, hostnames sets.String) (*TLSCertificateConfig, error) {
	server, err := GetTLSCertificateConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	cert := server.Certs[0]
	ips, dns := IPAddressesDNSNames(hostnames.List())
	missingIps := ipsNotInSlice(ips, cert.IPAddresses)
	missingDns := stringsNotInSlice(dns, cert.DNSNames)
	if len(missingIps) == 0 && len(missingDns) == 0 {
		glog.V(4).Infof("Found existing server certificate in %s", certFile)
		return server, nil
	}

	return nil, fmt.Errorf("Existing server certificate in %s was missing some hostnames (%v) or IP addresses (%v).", certFile, missingDns, missingIps)
}

func (ca *CA) MakeServerCert(certFile, keyFile string, hostnames sets.String) (*TLSCertificateConfig, error) {
	glog.V(4).Infof("Generating server certificate in %s, key in %s", certFile, keyFile)

	serverPublicKey, serverPrivateKey, _ := NewKeyPair()
	serverTemplate, _ := newServerCertificateTemplate(pkix.Name{CommonName: hostnames.List()[0]}, hostnames.List())
	serverCrt, _ := ca.signCertificate(serverTemplate, serverPublicKey)
	server := &TLSCertificateConfig{
		Certs: append([]*x509.Certificate{serverCrt}, ca.Config.Certs...),
		Key:   serverPrivateKey,
	}
	if err := server.writeCertConfig(certFile, keyFile); err != nil {
		return server, err
	}
	return server, nil
}

func (ca *CA) EnsureClientCertificate(certFile, keyFile string, u user.Info) (*TLSCertificateConfig, bool, error) {
	certConfig, err := GetTLSCertificateConfig(certFile, keyFile)
	if err != nil {
		certConfig, err = ca.MakeClientCertificate(certFile, keyFile, u)
		return certConfig, true, err // true indicates we wrote the files.
	}

	return certConfig, false, nil
}

func (ca *CA) MakeClientCertificate(certFile, keyFile string, u user.Info) (*TLSCertificateConfig, error) {
	glog.V(4).Infof("Generating client cert in %s and key in %s", certFile, keyFile)
	// ensure parent dirs
	if err := os.MkdirAll(filepath.Dir(certFile), os.FileMode(0755)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), os.FileMode(0755)); err != nil {
		return nil, err
	}

	clientPublicKey, clientPrivateKey, _ := NewKeyPair()
	clientTemplate, _ := newClientCertificateTemplate(x509request.UserToSubject(u))
	clientCrt, _ := ca.signCertificate(clientTemplate, clientPublicKey)

	certData, err := encodeCertificates(clientCrt)
	if err != nil {
		return nil, err
	}
	keyData, err := encodeKey(clientPrivateKey)
	if err != nil {
		return nil, err
	}

	if err = ioutil.WriteFile(certFile, certData, os.FileMode(0644)); err != nil {
		return nil, err
	}
	if err = ioutil.WriteFile(keyFile, keyData, os.FileMode(0600)); err != nil {
		return nil, err
	}

	return GetTLSCertificateConfig(certFile, keyFile)
}

// nextSerial returns a unique, monotonically increasing serial number and ensures the CA on
// disk records that value.
func (ca *CA) nextSerial() (int64, error) {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	next := ca.Serial + 1
	ca.Serial = next
	if err := ioutil.WriteFile(ca.SerialFile, []byte(fmt.Sprintf("%d", next)), os.FileMode(0640)); err != nil {
		return 0, err
	}
	return next, nil
}

func (ca *CA) signCertificate(template *x509.Certificate, requestKey crypto.PublicKey) (*x509.Certificate, error) {
	// Increment and persist serial
	serial, err := ca.nextSerial()
	if err != nil {
		return nil, err
	}
	template.SerialNumber = big.NewInt(serial)
	return signCertificate(template, requestKey, ca.Config.Certs[0], ca.Config.Key)
}

func NewKeyPair() (crypto.PublicKey, crypto.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, nil, err
	}
	return &privateKey.PublicKey, privateKey, nil
}

// Can be used for CA or intermediate signing certs
func newSigningCertificateTemplate(subject pkix.Name) (*x509.Certificate, error) {
	return &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    time.Now().Add(-1 * time.Second),
		NotAfter:     time.Now().Add(caLifetime),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA: true,
	}, nil
}

// Can be used for ListenAndServeTLS
func newServerCertificateTemplate(subject pkix.Name, hosts []string) (*x509.Certificate, error) {
	template := &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    time.Now().Add(-1 * time.Second),
		NotAfter:     time.Now().Add(lifetime),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IPAddresses, template.DNSNames = IPAddressesDNSNames(hosts)

	return template, nil
}

func IPAddressesDNSNames(hosts []string) ([]net.IP, []string) {
	ips := []net.IP{}
	dns := []string{}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
		} else {
			dns = append(dns, host)
		}
	}

	// Include IP addresses as DNS subjectAltNames in the cert as well, for the sake of Python, Windows (< 10), and unnamed other libraries
	// Ensure these technically invalid DNS subjectAltNames occur after the valid ones, to avoid triggering cert errors in Firefox
	// See https://bugzilla.mozilla.org/show_bug.cgi?id=1148766
	for _, ip := range ips {
		dns = append(dns, ip.String())
	}

	return ips, dns
}

func CertsFromPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	ok := false
	certs := []*x509.Certificate{}
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
		ok = true
	}

	if !ok {
		return certs, errors.New("Could not read any certificates")
	}
	return certs, nil
}

// Can be used as a certificate in http.Transport TLSClientConfig
func newClientCertificateTemplate(subject pkix.Name) (*x509.Certificate, error) {
	return &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    time.Now().Add(-1 * time.Second),
		NotAfter:     time.Now().Add(lifetime),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}, nil
}

func signCertificate(template *x509.Certificate, requestKey crypto.PublicKey, issuer *x509.Certificate, issuerKey crypto.PrivateKey) (*x509.Certificate, error) {
	derBytes, err := x509.CreateCertificate(rand.Reader, template, issuer, requestKey, issuerKey)
	if err != nil {
		return nil, err
	}
	certs, err := x509.ParseCertificates(derBytes)
	if err != nil {
		return nil, err
	}
	if len(certs) != 1 {
		return nil, errors.New("Expected a single certificate")
	}
	return certs[0], nil
}

func encodeCertificates(certs ...*x509.Certificate) ([]byte, error) {
	b := bytes.Buffer{}
	for _, cert := range certs {
		if err := pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
			return []byte{}, err
		}
	}
	return b.Bytes(), nil
}
func encodeKey(key crypto.PrivateKey) ([]byte, error) {
	b := bytes.Buffer{}
	switch key := key.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return []byte{}, err
		}
		if err := pem.Encode(&b, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
			return b.Bytes(), err
		}
	case *rsa.PrivateKey:
		if err := pem.Encode(&b, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
			return []byte{}, err
		}
	default:
		return []byte{}, errors.New("Unrecognized key type")

	}
	return b.Bytes(), nil
}

func writeCertificates(path string, certs ...*x509.Certificate) error {
	// ensure parent dir
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}

	bytes, err := encodeCertificates(certs...)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, bytes, os.FileMode(0644))
}
func writeKeyFile(path string, key crypto.PrivateKey) error {
	// ensure parent dir
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}

	b, err := encodeKey(key)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, os.FileMode(0600))
}

func stringsNotInSlice(needles []string, haystack []string) []string {
	missing := []string{}
	for _, needle := range needles {
		if !stringInSlice(needle, haystack) {
			missing = append(missing, needle)
		}
	}
	return missing
}

func stringInSlice(needle string, haystack []string) bool {
	for _, straw := range haystack {
		if needle == straw {
			return true
		}
	}
	return false
}

func ipsNotInSlice(needles []net.IP, haystack []net.IP) []net.IP {
	missing := []net.IP{}
	for _, needle := range needles {
		if !ipInSlice(needle, haystack) {
			missing = append(missing, needle)
		}
	}
	return missing
}

func ipInSlice(needle net.IP, haystack []net.IP) bool {
	for _, straw := range haystack {
		if needle.Equal(straw) {
			return true
		}
	}
	return false
}
