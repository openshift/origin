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
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
)

func filenamesFromDir(dir string) (string, string, string) {
	return filepath.Join(dir, "root.crt"), filepath.Join(dir, "cert.crt"), filepath.Join(dir, "key.key")
}

type TLSCertificateConfig struct {
	CAFile   string
	CertFile string
	KeyFile  string

	Roots []*x509.Certificate
	Certs []*x509.Certificate
	Key   crypto.PrivateKey
}

func (c *TLSCertificateConfig) writeDir(dir string) error {
	c.CAFile, c.CertFile, c.KeyFile = filenamesFromDir(dir)

	// mkdir
	if err := os.MkdirAll(dir, os.FileMode(0755)); err != nil {
		return err
	}

	// write certs and keys
	if err := writeCertificates(c.CAFile, c.Roots...); err != nil {
		return err
	}
	if err := writeCertificates(c.CertFile, c.Certs...); err != nil {
		return err
	}
	if err := writeKeyFile(c.KeyFile, c.Key); err != nil {
		return err
	}
	return nil
}

func newTLSCertificateConfig(dir string) (*TLSCertificateConfig, error) {
	caFile, certFile, keyFile := filenamesFromDir(dir)
	config := &TLSCertificateConfig{
		CAFile:   caFile,
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	if caFile != "" {
		caPEMBlock, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		config.Roots, err = certsFromPEM(caPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("Error reading %s: %s", caFile, err)
		}
	}

	if certFile != "" {
		certPEMBlock, err := ioutil.ReadFile(certFile)
		if err != nil {
			return nil, err
		}
		config.Certs, err = certsFromPEM(certPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("Error reading %s: %s", caFile, err)
		}

		if keyFile != "" {
			keyPEMBlock, err := ioutil.ReadFile(keyFile)
			if err != nil {
				return nil, err
			}
			keyPairCert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
			if err != nil {
				return nil, err
			}
			config.Key = keyPairCert.PrivateKey
		}
	}

	return config, nil
}

func readClientConfigFromDir(dir string, defaults kclient.Config) (kclient.Config, error) {
	// Make a copy of the default config
	client := defaults

	var err error

	// read files
	client.CAFile, client.CertFile, client.KeyFile = filenamesFromDir(dir)
	if client.CAData, err = ioutil.ReadFile(client.CAFile); err != nil {
		return client, err
	}
	if client.CertData, err = ioutil.ReadFile(client.CertFile); err != nil {
		return client, err
	}
	if client.KeyData, err = ioutil.ReadFile(client.KeyFile); err != nil {
		return client, err
	}

	return client, nil
}

func writeClientCertsToDir(client *kclient.Config, dir string) error {
	// mkdir
	if err := os.MkdirAll(dir, os.FileMode(0755)); err != nil {
		return err
	}

	// write files
	client.CAFile, client.CertFile, client.KeyFile = filenamesFromDir(dir)
	if err := ioutil.WriteFile(client.CAFile, client.CAData, os.FileMode(0644)); err != nil {
		return err
	}
	if err := ioutil.WriteFile(client.CertFile, client.CertData, os.FileMode(0644)); err != nil {
		return err
	}
	if err := ioutil.WriteFile(client.KeyFile, client.KeyData, os.FileMode(0600)); err != nil {
		return err
	}
	return nil
}

func writeKubeConfigToDir(client *kclient.Config, username string, dir string) error {
	// mkdir
	if err := os.MkdirAll(dir, os.FileMode(0755)); err != nil {
		return err
	}

	caFile, err := filepath.Rel(dir, client.CAFile)
	if err != nil {
		return err
	}
	certFile, err := filepath.Rel(dir, client.CertFile)
	if err != nil {
		return err
	}
	keyFile, err := filepath.Rel(dir, client.KeyFile)
	if err != nil {
		return err
	}

	clusterName := "master"
	contextName := clusterName + "-" + username
	config := &clientcmdapi.Config{
		Clusters: map[string]clientcmdapi.Cluster{
			clusterName: {
				Server:                client.Host,
				CertificateAuthority:  caFile,
				InsecureSkipTLSVerify: client.Insecure,
			},
		},
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			username: {
				Token:             client.BearerToken,
				ClientCertificate: certFile,
				ClientKey:         keyFile,
			},
		},
		Contexts: map[string]clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: username,
			},
		},
		CurrentContext: contextName,
	}

	if err := clientcmd.WriteToFile(*config, filepath.Join(dir, ".kubeconfig")); err != nil {
		return err
	}

	return nil
}

func certsFromPEM(pemCerts []byte) ([]*x509.Certificate, error) {
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

var (
	// Default templates to last for a year
	lifetime = time.Hour * 24 * 365

	// Default keys are 2048 bits
	keyBits = 2048
)

type CA struct {
	Dir        string
	SerialFile string
	Serial     int64
	Config     *TLSCertificateConfig
}

// InitCA ensures a certificate authority structure exists in the given directory, creating it if necessary:
//	<dir>/
//	  ca/
//	root.crt	- Root certificate bundle.
//	cert.crt	- Signing certificate
//	key.key	 - Private key
//	serial.txt  - Stores the highest serial number generated by this CA
func InitCA(dir string, name string) (*CA, error) {
	caDir := filepath.Join(dir, "ca")

	caConfig, err := newTLSCertificateConfig(caDir)
	if err != nil {
		glog.Infof("Generating new CA in %s", caDir)
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
		caConfig = &TLSCertificateConfig{
			Roots: []*x509.Certificate{rootcaCert},
			Certs: []*x509.Certificate{rootcaCert},
			Key:   rootcaPrivateKey,
		}
		if err := caConfig.writeDir(caDir); err != nil {
			return nil, err
		}
	} else {
		glog.Infof("Using existing CA certificate in %s", caDir)
	}

	// read serial file
	var serial int64
	serialFile := filepath.Join(caDir, "serial.txt")
	if serialData, err := ioutil.ReadFile(serialFile); err == nil {
		serial, _ = strconv.ParseInt(string(serialData), 10, 64)
	}
	if serial < 1 {
		serial = 1
	}

	return &CA{
		Serial:     serial,
		SerialFile: serialFile,
		Dir:        dir,
		Config:     caConfig,
	}, nil
}

// MakeServerCert creates a folder containing certificates for the given server:
//	<CA.dir>/
//	 <name>/
//	root.crt	- Root certificate bundle.
//	cert.crt	- Server certificate
//	key.key	 - Private key
// The generated certificate has the following attributes:
//	CommonName: hostnames[0]
//	DNSNames subjectAltNames containing all specified hostnames
//	IPAddresses subjectAltNames containing all specified hostnames which are IP addresses
//	ExtKeyUsage: ExtKeyUsageServerAuth
func (ca *CA) MakeServerCert(name string, hostnames []string) (*TLSCertificateConfig, error) {
	serverDir := filepath.Join(ca.Dir, name)

	server, err := newTLSCertificateConfig(serverDir)
	if err == nil {
		cert := server.Certs[0]
		ips, dns := IPAddressesDNSNames(hostnames)
		missingIps := ipsNotInSlice(ips, cert.IPAddresses)
		missingDns := stringsNotInSlice(dns, cert.DNSNames)
		if len(missingIps) == 0 && len(missingDns) == 0 {
			glog.Infof("Using existing server certificate in %s", serverDir)
			return server, nil
		}

		glog.Infof("Existing server certificate in %s was missing some hostnames (%v) or IP addresses (%v)", serverDir, missingDns, missingIps)
	}

	glog.Infof("Generating server certificate in %s", serverDir)
	serverPublicKey, serverPrivateKey, _ := NewKeyPair()
	serverTemplate, _ := newServerCertificateTemplate(pkix.Name{CommonName: hostnames[0]}, hostnames)
	serverCrt, _ := ca.signCertificate(serverTemplate, serverPublicKey)
	server = &TLSCertificateConfig{
		Roots: ca.Config.Roots,
		Certs: append([]*x509.Certificate{serverCrt}, ca.Config.Certs...),
		Key:   serverPrivateKey,
	}
	if err := server.writeDir(serverDir); err != nil {
		return server, err
	}
	return server, nil
}

// MakeClientConfig creates a folder containing certificates for the given client:
//   <CA.dir>/
//     <id>/
//       root.crt - Root certificate bundle.
//       cert.crt - Client certificate
//       key.key  - Private key
// The generated certificate has the following attributes:
//   Subject:
//     SerialNumber: user.GetUID()
//     CommonName:   user.GetName()
//     Organization: user.GetGroups()
//   ExtKeyUsage: ExtKeyUsageClientAuth
func (ca *CA) MakeClientConfig(clientId string, u user.Info, defaults kclient.Config) (kclient.Config, error) {
	clientDir := filepath.Join(ca.Dir, clientId)
	kubeConfig := filepath.Join(clientDir, ".kubeconfig")

	client, err := readClientConfigFromDir(clientDir, defaults)
	if err == nil {
		// Always write .kubeconfig to pick up hostname changes
		if err := writeKubeConfigToDir(&client, clientId, clientDir); err != nil {
			return client, err
		}
		glog.Infof("Using existing client config in %s", kubeConfig)
		return client, nil
	}

	glog.Infof("Generating client config in %s", kubeConfig)

	// Make a copy of the default config
	client = defaults

	// Create cert for system components to use to talk to the API
	clientPublicKey, clientPrivateKey, _ := NewKeyPair()
	clientTemplate, _ := newClientCertificateTemplate(x509request.UserToSubject(u))
	clientCrt, _ := ca.signCertificate(clientTemplate, clientPublicKey)

	caData, err := encodeCertificates(ca.Config.Roots...)
	if err != nil {
		return client, err
	}
	certData, err := encodeCertificates(clientCrt)
	if err != nil {
		return client, err
	}
	keyData, err := encodeKey(clientPrivateKey)
	if err != nil {
		return client, err
	}

	client.CAData = caData
	client.CertData = certData
	client.KeyData = keyData

	if err := writeClientCertsToDir(&client, clientDir); err != nil {
		return client, err
	}
	if err := writeKubeConfigToDir(&client, clientId, clientDir); err != nil {
		return client, err
	}

	return client, nil
}

func (ca *CA) signCertificate(template *x509.Certificate, requestKey crypto.PublicKey) (*x509.Certificate, error) {
	// Increment and persist serial
	ca.Serial = ca.Serial + 1
	if err := ioutil.WriteFile(ca.SerialFile, []byte(fmt.Sprintf("%d", ca.Serial)), os.FileMode(0640)); err != nil {
		return nil, err
	}
	template.SerialNumber = big.NewInt(ca.Serial)
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
		NotAfter:     time.Now().Add(lifetime),
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
		}
		// Include IP addresses as DNS names in the cert, for Python's sake
		dns = append(dns, host)
	}
	return ips, dns
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
	bytes, err := encodeCertificates(certs...)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, bytes, os.FileMode(0644))
}
func writeKeyFile(path string, key crypto.PrivateKey) error {
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
