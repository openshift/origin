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
	mathrand "math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"

	"sort"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

var versions = map[string]uint16{
	"VersionTLS10": tls.VersionTLS10,
	"VersionTLS11": tls.VersionTLS11,
	"VersionTLS12": tls.VersionTLS12,
}

func TLSVersion(versionName string) (uint16, error) {
	if len(versionName) == 0 {
		return DefaultTLSVersion(), nil
	}
	if version, ok := versions[versionName]; ok {
		return version, nil
	}
	return 0, fmt.Errorf("unknown tls version %q", versionName)
}
func TLSVersionOrDie(versionName string) uint16 {
	version, err := TLSVersion(versionName)
	if err != nil {
		panic(err)
	}
	return version
}
func ValidTLSVersions() []string {
	validVersions := []string{}
	for k := range versions {
		validVersions = append(validVersions, k)
	}
	sort.Strings(validVersions)
	return validVersions
}
func DefaultTLSVersion() uint16 {
	// Can't use SSLv3 because of POODLE and BEAST
	// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
	// Can't use TLSv1.1 because of RC4 cipher usage
	return tls.VersionTLS12
}

var ciphers = map[string]uint16{
	"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
	"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"TLS_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

func CipherSuite(cipherName string) (uint16, error) {
	if cipher, ok := ciphers[cipherName]; ok {
		return cipher, nil
	}
	return 0, fmt.Errorf("unknown cipher name %q", cipherName)
}

func CipherSuitesOrDie(cipherNames []string) []uint16 {
	if len(cipherNames) == 0 {
		return DefaultCiphers()
	}
	cipherValues := []uint16{}
	for _, cipherName := range cipherNames {
		cipher, err := CipherSuite(cipherName)
		if err != nil {
			panic(err)
		}
		cipherValues = append(cipherValues, cipher)
	}
	return cipherValues
}
func ValidCipherSuites() []string {
	validCipherSuites := []string{}
	for k := range ciphers {
		validCipherSuites = append(validCipherSuites, k)
	}
	sort.Strings(validCipherSuites)
	return validCipherSuites
}
func DefaultCiphers() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, // required by http/2
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, // forbidden by http/2
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, // forbidden by http/2
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,   // forbidden by http/2
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,   // forbidden by http/2
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,      // forbidden by http/2
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,      // forbidden by http/2
		// the next one is in the intermediate suite, but go1.8 http2isBadCipher() complains when it is included at the recommended index
		// because it comes after ciphers forbidden by the http/2 spec
		// tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,        // forbidden by http/2
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,        // forbidden by http/2
		tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, // forbidden by http/2
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,       // forbidden by http/2
	}
}

// SecureTLSConfig enforces the default minimum security settings for the cluster.
func SecureTLSConfig(config *tls.Config) *tls.Config {
	if config.MinVersion == 0 {
		config.MinVersion = DefaultTLSVersion()
	}

	config.PreferServerCipherSuites = true
	if len(config.CipherSuites) == 0 {
		config.CipherSuites = DefaultCiphers()
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
func (c *TLSCertificateConfig) GetPEMBytes() ([]byte, []byte, error) {
	certBytes, err := encodeCertificates(c.Certs...)
	if err != nil {
		return nil, nil, err
	}
	keyBytes, err := encodeKey(c.Key)
	if err != nil {
		return nil, nil, err
	}

	return certBytes, keyBytes, nil
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

const (
	DefaultCertificateLifetimeInDays   = 365 * 2 // 2 years
	DefaultCACertificateLifetimeInDays = 365 * 5 // 5 years

	// Default keys are 2048 bits
	keyBits = 2048
)

type CA struct {
	Config *TLSCertificateConfig

	SerialGenerator SerialGenerator
}

// SerialGenerator is an interface for getting a serial number for the cert.  It MUST be thread-safe.
type SerialGenerator interface {
	Next(template *x509.Certificate) (int64, error)
}

// SerialFileGenerator returns a unique, monotonically increasing serial number and ensures the CA on disk records that value.
type SerialFileGenerator struct {
	SerialFile string

	// lock guards access to the Serial field
	lock   sync.Mutex
	Serial int64
}

func NewSerialFileGenerator(serialFile string, createIfNeeded bool) (*SerialFileGenerator, error) {
	// read serial file
	var serial int64
	serialData, err := ioutil.ReadFile(serialFile)
	if err == nil {
		serial, _ = strconv.ParseInt(string(serialData), 16, 64)
	}
	if os.IsNotExist(err) && createIfNeeded {
		if err := ioutil.WriteFile(serialFile, []byte("00"), 0644); err != nil {
			return nil, err
		}
		serial = 1

	} else if err != nil {
		return nil, err
	}

	if serial < 1 {
		serial = 1
	}

	return &SerialFileGenerator{
		Serial:     serial,
		SerialFile: serialFile,
	}, nil
}

// Next returns a unique, monotonically increasing serial number and ensures the CA on disk records that value.
func (s *SerialFileGenerator) Next(template *x509.Certificate) (int64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	next := s.Serial + 1
	s.Serial = next

	// Output in hex, padded to multiples of two characters for OpenSSL's sake
	serialText := fmt.Sprintf("%X", next)
	if len(serialText)%2 == 1 {
		serialText = "0" + serialText
	}

	if err := ioutil.WriteFile(s.SerialFile, []byte(serialText), os.FileMode(0640)); err != nil {
		return 0, err
	}
	return next, nil
}

// RandomSerialGenerator returns a serial based on time.Now and the subject
type RandomSerialGenerator struct {
}

func (s *RandomSerialGenerator) Next(template *x509.Certificate) (int64, error) {
	r := mathrand.New(mathrand.NewSource(time.Now().UTC().UnixNano()))
	return r.Int63(), nil
}

// EnsureCA returns a CA, whether it was created (as opposed to pre-existing), and any error
// if serialFile is empty, a RandomSerialGenerator will be used
func EnsureCA(certFile, keyFile, serialFile, name string, expireDays int) (*CA, bool, error) {
	if ca, err := GetCA(certFile, keyFile, serialFile); err == nil {
		return ca, false, err
	}
	ca, err := MakeCA(certFile, keyFile, serialFile, name, expireDays)
	return ca, true, err
}

// if serialFile is empty, a RandomSerialGenerator will be used
func GetCA(certFile, keyFile, serialFile string) (*CA, error) {
	caConfig, err := GetTLSCertificateConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	var serialGenerator SerialGenerator
	if len(serialFile) > 0 {
		serialGenerator, err = NewSerialFileGenerator(serialFile, false)
		if err != nil {
			return nil, err
		}
	} else {
		serialGenerator = &RandomSerialGenerator{}
	}

	return &CA{
		SerialGenerator: serialGenerator,
		Config:          caConfig,
	}, nil
}

// if serialFile is empty, a RandomSerialGenerator will be used
func MakeCA(certFile, keyFile, serialFile, name string, expireDays int) (*CA, error) {
	glog.V(2).Infof("Generating new CA for %s cert, and key in %s, %s", name, certFile, keyFile)
	// Create CA cert
	rootcaPublicKey, rootcaPrivateKey, err := NewKeyPair()
	if err != nil {
		return nil, err
	}
	rootcaTemplate := newSigningCertificateTemplate(pkix.Name{CommonName: name}, expireDays, time.Now)
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

	var serialGenerator SerialGenerator
	if len(serialFile) > 0 {
		if err := ioutil.WriteFile(serialFile, []byte("00"), 0644); err != nil {
			return nil, err
		}
		serialGenerator, err = NewSerialFileGenerator(serialFile, false)
		if err != nil {
			return nil, err
		}
	} else {
		serialGenerator = &RandomSerialGenerator{}
	}

	return &CA{
		SerialGenerator: serialGenerator,
		Config:          caConfig,
	}, nil
}

func (ca *CA) EnsureServerCert(certFile, keyFile string, hostnames sets.String, expireDays int) (*TLSCertificateConfig, bool, error) {
	certConfig, err := GetServerCert(certFile, keyFile, hostnames)
	if err != nil {
		certConfig, err = ca.MakeAndWriteServerCert(certFile, keyFile, hostnames, expireDays)
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

func (ca *CA) MakeAndWriteServerCert(certFile, keyFile string, hostnames sets.String, expireDays int) (*TLSCertificateConfig, error) {
	glog.V(4).Infof("Generating server certificate in %s, key in %s", certFile, keyFile)

	server, err := ca.MakeServerCert(hostnames, expireDays)
	if err != nil {
		return nil, err
	}
	if err := server.writeCertConfig(certFile, keyFile); err != nil {
		return server, err
	}
	return server, nil
}

// CertificateExtensionFunc is passed a certificate that it may extend, or return an error
// if the extension attempt failed.
type CertificateExtensionFunc func(*x509.Certificate) error

func (ca *CA) MakeServerCert(hostnames sets.String, expireDays int, fns ...CertificateExtensionFunc) (*TLSCertificateConfig, error) {
	serverPublicKey, serverPrivateKey, _ := NewKeyPair()
	serverTemplate := newServerCertificateTemplate(pkix.Name{CommonName: hostnames.List()[0]}, hostnames.List(), expireDays, time.Now)
	for _, fn := range fns {
		if err := fn(serverTemplate); err != nil {
			return nil, err
		}
	}
	serverCrt, err := ca.signCertificate(serverTemplate, serverPublicKey)
	if err != nil {
		return nil, err
	}
	server := &TLSCertificateConfig{
		Certs: append([]*x509.Certificate{serverCrt}, ca.Config.Certs...),
		Key:   serverPrivateKey,
	}
	return server, nil
}

func (ca *CA) EnsureClientCertificate(certFile, keyFile string, u user.Info, expireDays int) (*TLSCertificateConfig, bool, error) {
	certConfig, err := GetTLSCertificateConfig(certFile, keyFile)
	if err != nil {
		certConfig, err = ca.MakeClientCertificate(certFile, keyFile, u, expireDays)
		return certConfig, true, err // true indicates we wrote the files.
	}

	return certConfig, false, nil
}

func (ca *CA) MakeClientCertificate(certFile, keyFile string, u user.Info, expireDays int) (*TLSCertificateConfig, error) {
	glog.V(4).Infof("Generating client cert in %s and key in %s", certFile, keyFile)
	// ensure parent dirs
	if err := os.MkdirAll(filepath.Dir(certFile), os.FileMode(0755)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), os.FileMode(0755)); err != nil {
		return nil, err
	}

	clientPublicKey, clientPrivateKey, _ := NewKeyPair()
	clientTemplate := newClientCertificateTemplate(userToSubject(u), expireDays, time.Now)
	clientCrt, err := ca.signCertificate(clientTemplate, clientPublicKey)
	if err != nil {
		return nil, err
	}

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

func userToSubject(u user.Info) pkix.Name {
	return pkix.Name{
		CommonName:   u.GetName(),
		SerialNumber: u.GetUID(),
		Organization: u.GetGroups(),
	}
}

func (ca *CA) signCertificate(template *x509.Certificate, requestKey crypto.PublicKey) (*x509.Certificate, error) {
	// Increment and persist serial
	serial, err := ca.SerialGenerator.Next(template)
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
func newSigningCertificateTemplate(subject pkix.Name, expireDays int, currentTime func() time.Time) *x509.Certificate {
	var caLifetimeInDays = DefaultCACertificateLifetimeInDays
	if expireDays > 0 {
		caLifetimeInDays = expireDays
	}

	if caLifetimeInDays > DefaultCACertificateLifetimeInDays {
		warnAboutCertificateLifeTime(subject.CommonName, DefaultCACertificateLifetimeInDays)
	}

	caLifetime := time.Duration(caLifetimeInDays) * 24 * time.Hour

	return &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    currentTime().Add(-1 * time.Second),
		NotAfter:     currentTime().Add(caLifetime),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA: true,
	}
}

// Can be used for ListenAndServeTLS
func newServerCertificateTemplate(subject pkix.Name, hosts []string, expireDays int, currentTime func() time.Time) *x509.Certificate {
	var lifetimeInDays = DefaultCertificateLifetimeInDays
	if expireDays > 0 {
		lifetimeInDays = expireDays
	}

	if lifetimeInDays > DefaultCertificateLifetimeInDays {
		warnAboutCertificateLifeTime(subject.CommonName, DefaultCertificateLifetimeInDays)
	}

	lifetime := time.Duration(lifetimeInDays) * 24 * time.Hour

	template := &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    currentTime().Add(-1 * time.Second),
		NotAfter:     currentTime().Add(lifetime),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IPAddresses, template.DNSNames = IPAddressesDNSNames(hosts)

	return template
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
func newClientCertificateTemplate(subject pkix.Name, expireDays int, currentTime func() time.Time) *x509.Certificate {
	var lifetimeInDays = DefaultCertificateLifetimeInDays
	if expireDays > 0 {
		lifetimeInDays = expireDays
	}

	if lifetimeInDays > DefaultCertificateLifetimeInDays {
		warnAboutCertificateLifeTime(subject.CommonName, DefaultCertificateLifetimeInDays)
	}

	lifetime := time.Duration(lifetimeInDays) * 24 * time.Hour

	return &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    currentTime().Add(-1 * time.Second),
		NotAfter:     currentTime().Add(lifetime),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
}

func warnAboutCertificateLifeTime(name string, defaultLifetimeInDays int) {
	defaultLifetimeInYears := defaultLifetimeInDays / 365
	fmt.Fprintf(os.Stderr, "WARNING: Validity period of the certificate for %q is greater than %d years!\n", name, defaultLifetimeInYears)
	fmt.Fprintln(os.Stderr, "WARNING: By security reasons it is strongly recommended to change this period and make it smaller!")
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
