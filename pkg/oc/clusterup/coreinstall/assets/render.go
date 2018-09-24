package assets

import (
	"crypto/rsa"
	"crypto/x509"
	"net/url"

	assetslib "github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/assets/tlsutil"
)

type TLSAssetsRenderOptions struct {
	AltNames tlsutil.AltNames

	config tlsAssetsRenderConfig
}

type tlsAssetsRenderConfig struct {
	EtcdServerURL string
	ServerURL     string
	AdminKey      *rsa.PrivateKey
	AdminCert     *x509.Certificate
	CACert        *x509.Certificate
}

func NewTLSAssetsRenderer(hostname string) *TLSAssetsRenderOptions {
	return &TLSAssetsRenderOptions{
		config: tlsAssetsRenderConfig{
			ServerURL:     "https://" + hostname + ":8443",
			EtcdServerURL: "https://" + hostname + ":2379",
		},
	}
}

func (r *TLSAssetsRenderOptions) Render() (*assetslib.Assets, error) {
	result := assetslib.Assets{}

	// Generate CA
	caPrivateKey, caCert, err := r.newCACert()
	if err != nil {
		return nil, err
	}
	r.config.CACert = caCert

	// Generate apiserver certs and keys
	if files, err := r.newTLSAssets(caCert, caPrivateKey, r.AltNames); err != nil {
		return nil, err
	} else {
		result = append(result, files...)
	}

	etcdURL, err := url.Parse(r.config.EtcdServerURL)
	if err != nil {
		return nil, err
	}

	// Generate etcd certs
	if files, err := r.newEtcdTLSAssets(caCert, caPrivateKey, []*url.URL{etcdURL}); err != nil {
		return nil, err
	} else {
		result = append(result, files...)
	}

	// Generate admin.kubeconfig
	result = append(result, r.newAdminKubeConfig()...)

	return &result, nil
}
