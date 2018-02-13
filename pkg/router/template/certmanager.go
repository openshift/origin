package templaterouter

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// certificateFile represents a certificate file.
type certificateFile struct {
	certDir string
	id      string
}

// Tag generates a certificate file tag/name. This is used to index into the
// the map of deleted certificates.
func (cf certificateFile) Tag() string {
	return filepath.Join(cf.certDir, cf.id+".pem")
}

// simpleCertificateManager is the default implementation of a certificateManager
type simpleCertificateManager struct {
	cfg *certificateManagerConfig
	w   certificateWriter

	deletedCertificates map[string]certificateFile
}

// newSimpleCertificateManager should be used to create a new cert manager.  It will return an error
// if the config is not valid
func newSimpleCertificateManager(cfg *certificateManagerConfig, w certificateWriter) (certificateManager, error) {
	if err := validateCertManagerConfig(cfg); err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("certificate manager requires a certificate writer")
	}
	return &simpleCertificateManager{cfg, w, make(map[string]certificateFile, 0)}, nil
}

// validateCertManagerConfig ensures that the key functions and directories are set as well as
// ensuring that the two configured directories are set to different values
func validateCertManagerConfig(cfg *certificateManagerConfig) error {
	if cfg.certKeyFunc == nil || cfg.caCertKeyFunc == nil ||
		cfg.destCertKeyFunc == nil || len(cfg.certDir) == 0 ||
		len(cfg.caCertDir) == 0 {
		return fmt.Errorf("certificate manager requires all config items to be set")
	}
	if cfg.certDir == cfg.caCertDir {
		return fmt.Errorf("certificate manager requires different directories for certDir and caCertDir")
	}
	return nil
}

// CertificateWriter provides direct access to the underlying writer if required
func (cm *simpleCertificateManager) CertificateWriter() certificateWriter {
	return cm.w
}

// WriteCertificatesForConfig write certificates for edge and reencrypt termination by appending the
// key, cert, and ca cert into a single <host>.pem file.  Also write <host>_pod.pem file if it is
// reencrypt termination
func (cm *simpleCertificateManager) WriteCertificatesForConfig(config *ServiceAliasConfig) error {
	if config == nil {
		return nil
	}
	if config.Status == ServiceAliasConfigStatusSaved {
		glog.V(4).Infof("skipping certificate write for %s%s since its status is already %s", config.Host, config.Path, ServiceAliasConfigStatusSaved)
		return nil
	}
	if len(config.Certificates) > 0 {
		if config.TLSTermination == routeapi.TLSTerminationEdge || config.TLSTermination == routeapi.TLSTerminationReencrypt {
			certKey := cm.cfg.certKeyFunc(config)
			certObj, ok := config.Certificates[certKey]

			if ok {
				newLine := []byte("\n")

				//initialize with key and append the newline and cert
				buffer := bytes.NewBuffer([]byte(certObj.PrivateKey))
				buffer.Write(newLine)
				buffer.Write([]byte(certObj.Contents))

				caCertKey := cm.cfg.caCertKeyFunc(config)
				caCertObj, caOk := config.Certificates[caCertKey]

				if caOk {
					buffer.Write(newLine)
					buffer.Write([]byte(caCertObj.Contents))
				}

				certFile := certificateFile{certDir: cm.cfg.certDir, id: certObj.ID}
				delete(cm.deletedCertificates, certFile.Tag())
				if err := cm.w.WriteCertificate(cm.cfg.certDir, certObj.ID, buffer.Bytes()); err != nil {
					return err
				}
			}
		}

		if config.TLSTermination == routeapi.TLSTerminationReencrypt {
			destCertKey := cm.cfg.destCertKeyFunc(config)
			destCert, ok := config.Certificates[destCertKey]

			if ok {
				destCertFile := certificateFile{certDir: cm.cfg.caCertDir, id: destCert.ID}
				delete(cm.deletedCertificates, destCertFile.Tag())
				if err := cm.w.WriteCertificate(cm.cfg.caCertDir, destCert.ID, []byte(destCert.Contents)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// DeleteCertificatesForConfig will delete all certificates for the ServiceAliasConfig
func (cm *simpleCertificateManager) DeleteCertificatesForConfig(config *ServiceAliasConfig) error {
	if config == nil {
		return nil
	}
	if len(config.Certificates) > 0 {
		if config.TLSTermination == routeapi.TLSTerminationEdge || config.TLSTermination == routeapi.TLSTerminationReencrypt {
			certKey := cm.cfg.certKeyFunc(config)
			certObj, ok := config.Certificates[certKey]

			if ok {
				certFile := certificateFile{certDir: cm.cfg.certDir, id: certObj.ID}
				cm.deletedCertificates[certFile.Tag()] = certFile
			}
		}

		if config.TLSTermination == routeapi.TLSTerminationReencrypt {
			destCertKey := cm.cfg.destCertKeyFunc(config)
			destCert, ok := config.Certificates[destCertKey]

			if ok {
				destCertFile := certificateFile{certDir: cm.cfg.caCertDir, id: destCert.ID}
				cm.deletedCertificates[destCertFile.Tag()] = destCertFile
			}
		}
	}
	return nil
}

// Commit applies any pending changes made to the certificateManager.
func (cm *simpleCertificateManager) Commit() error {
	// Deletion of certificates that are being referenced in backends or
	// config is problematic in that the template router will not
	// reload because the config is invalid, so we _do_ need to "stage"
	// or commit the removals. Remove all the deleted certificates.
	for _, certFile := range cm.deletedCertificates {
		err := cm.w.DeleteCertificate(certFile.certDir, certFile.id)
		if err != nil {
			// Log a warning if the delete fails but proceed on.
			glog.Warningf("Ignoring error deleting certificate file %v: %v", certFile.Tag(), err)
		}
	}

	cm.deletedCertificates = make(map[string]certificateFile, 0)

	// If we decide to stage the certificate writes, we can flush the
	// write to the disk here. Today, the certificate writes are done
	// just before this function is called. The tradeoff is storing a
	// copy in memory until we commit.

	return nil
}

// simpleCertificateWriter is the default implementation of a certificateWriter
type simpleCertificateWriter struct{}

// NewSimpleCertificateWriter provides a new instance of simpleCertificateWriter
func newSimpleCertificateWriter() certificateWriter {
	return &simpleCertificateWriter{}
}

// WriteCertificate creates and writes the file identified by <id> in <directory>.  The file extension
// .pem will be added to id.
func (cm *simpleCertificateWriter) WriteCertificate(directory string, id string, cert []byte) error {
	fileName := filepath.Join(directory, id+".pem")
	err := ioutil.WriteFile(fileName, cert, 0644)

	if err != nil {
		glog.Errorf("Error writing certificate file %v: %v", fileName, err)
		return err
	}
	return nil
}

// DeleteCertificate deletes certificates identified by <id> in <directory> with the .pem extension added.
// this will not return an error if the file does not exist
func (cm *simpleCertificateWriter) DeleteCertificate(directory, id string) error {
	fileName := filepath.Join(directory, id+".pem")
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		glog.V(4).Infof("attempted to delete file %s but it does not exist", fileName)
		return nil
	}

	err := os.Remove(fileName)
	if os.IsNotExist(err) {
		glog.V(4).Infof("%s passed the existence check but it was gone when os.Remove was called", fileName)
		return nil
	}
	return err
}
