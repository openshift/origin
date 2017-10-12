package importer

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	restclient "k8s.io/client-go/rest"
)

const (
	// dockerCertsDir is the directory where Docker stores certificates.
	dockerCertsDir = "/etc/docker/certs.d"
)

func hasFile(files []os.FileInfo, name string) bool {
	for _, f := range files {
		if f.Name() == name {
			return true
		}
	}
	return false
}

// readCertsDirectory reads the directory for TLS certificates including roots
// and certificate pairs and updates the provided TLS configuration.
func readCertsDirectory(tlsConfig *tls.Config, directory string) error {
	fs, err := ioutil.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".crt") {
			if tlsConfig.RootCAs == nil {
				systemPool, err := x509.SystemCertPool()
				if err != nil {
					return fmt.Errorf("unable to get system cert pool: %v", err)
				}
				tlsConfig.RootCAs = systemPool
			}
			glog.V(5).Infof("crt: %s", filepath.Join(directory, f.Name()))
			data, err := ioutil.ReadFile(filepath.Join(directory, f.Name()))
			if err != nil {
				return err
			}
			tlsConfig.RootCAs.AppendCertsFromPEM(data)
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			certName := f.Name()
			keyName := certName[:len(certName)-5] + ".key"
			glog.V(5).Infof("cert: %s", filepath.Join(directory, f.Name()))
			if !hasFile(fs, keyName) {
				return fmt.Errorf("missing key %s for client certificate %s. Note that CA certificates should use the extension .crt", keyName, certName)
			}
			cert, err := tls.LoadX509KeyPair(filepath.Join(directory, certName), filepath.Join(directory, keyName))
			if err != nil {
				return err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		}
		if strings.HasSuffix(f.Name(), ".key") {
			keyName := f.Name()
			certName := keyName[:len(keyName)-4] + ".cert"
			glog.V(5).Infof("key: %s", filepath.Join(directory, f.Name()))
			if !hasFile(fs, certName) {
				return fmt.Errorf("Missing client certificate %s for key %s", certName, keyName)
			}
		}
	}

	return nil
}

type TransportRetriever interface {
	TransportFor(host string, insecure bool) (http.RoundTripper, error)
}

type transportRetriever struct {
	certsDir string
}

func NewTransportRetriever() TransportRetriever {
	return &transportRetriever{
		certsDir: dockerCertsDir,
	}
}

func (tr *transportRetriever) TransportFor(host string, insecure bool) (http.RoundTripper, error) {
	tlsConfig := &tls.Config{
		MinVersion:               tls.VersionTLS10,
		PreferServerCipherSuites: true,
		InsecureSkipVerify:       insecure,
	}

	if err := readCertsDirectory(tlsConfig, filepath.Join(tr.certsDir, host)); err != nil {
		return nil, fmt.Errorf("unable to read certificates: %v", err)
	}

	return restclient.TransportFor(&restclient.Config{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	})
}
