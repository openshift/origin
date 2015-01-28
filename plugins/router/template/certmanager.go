package templaterouter

import (
	"bytes"
	"github.com/golang/glog"
	"io/ioutil"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// certManager is responsible for writing out certificates to the disk for the template router plugin.
type certManager struct{}

// writeCertificatesForConfig write certificates for edge and reencrypt termination by appending the key, cert, and ca cert
// into a single <host>.pem file.  Also write <host>_pod.pem file if it is reencrypt termination
func (cm *certManager) writeCertificatesForConfig(config *ServiceAliasConfig) error {
	if len(config.Certificates) > 0 {
		if config.TLSTermination == routeapi.TLSTerminationEdge || config.TLSTermination == routeapi.TLSTerminationReencrypt {
			certObj, ok := config.Certificates[config.Host]

			if ok {
				newLine := []byte("\n")

				//initialize with key and append the newline and cert
				buffer := bytes.NewBuffer([]byte(certObj.PrivateKey))
				buffer.Write(newLine)
				buffer.Write([]byte(certObj.Contents))

				caCertObj, caOk := config.Certificates[config.Host+caCertPostfix]

				if caOk {
					buffer.Write(newLine)
					buffer.Write([]byte(caCertObj.Contents))
				}

				cm.writeCertificate(certDir, config.Host, buffer.Bytes())
			}
		}

		if config.TLSTermination == routeapi.TLSTerminationReencrypt {
			destCertKey := config.Host + destCertPostfix
			destCert, ok := config.Certificates[destCertKey]

			if ok {
				cm.writeCertificate(caCertDir, destCertKey, []byte(destCert.Contents))
			}
		}
	}

	return nil
}

// writeCertificate creates and writes the file identified by <id> in <directory>.  The file extension
// .pem will be added to id.
func (cm *certManager) writeCertificate(directory string, id string, cert []byte) error {
	fileName := directory + id + ".pem"
	err := ioutil.WriteFile(fileName, cert, 0644)

	if err != nil {
		glog.Errorf("Error writing certificate file %v: %v", fileName, err)
		return err
	}

	return nil
}

// deleteCertificatesForConfig will delete all certificates for the ServiceAliasConfig
func (cm *certManager) deleteCertificatesForConfig(config *ServiceAliasConfig) error {
	//TODO
	return nil
}
