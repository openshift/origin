package assets

import (
	"crypto/rsa"
	"crypto/x509"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/assets/tlsutil"
)

func (r *TLSAssetsRenderOptions) newAdminKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey) (*rsa.PrivateKey, *x509.Certificate, error) {
	// TLS organizations map to Kubernetes groups, and "system:masters"
	// is a well-known Kubernetes group that gives a user admin power.
	const orgSystemMasters = "system:masters"

	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	config := tlsutil.CertConfig{
		CommonName:   "admin",
		Organization: []string{orgSystemMasters},
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}
