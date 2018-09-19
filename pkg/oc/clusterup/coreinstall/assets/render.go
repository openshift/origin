package assets

import (
	"crypto/rsa"
	"crypto/x509"
	"net/url"

	assetslib "github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/assets/tlsutil"
)

type TLSAssetsRenderOptions struct {
	AltNames *tlsutil.AltNames

	etcdServerURL string
	serverURL     string
	adminKey      *rsa.PrivateKey
	adminCert     *x509.Certificate
	caCert        *x509.Certificate
}

func NewTLSAssetsRenderer(hostname string) *TLSAssetsRenderOptions {
	return &TLSAssetsRenderOptions{
		serverURL:     "https://" + hostname + ":8443",
		etcdServerURL: "https://" + hostname + ":2379",
	}
}

func (r *TLSAssetsRenderOptions) Render() (*assetslib.Assets, error) {
	result := assetslib.Assets{}

	// Generate CA
	caPrivateKey, caCert, err := r.newCACert()
	if err != nil {
		return nil, err
	}
	r.caCert = caCert

	// Generate apiserver certs and keys
	if files, err := r.newTLSAssets(caCert, caPrivateKey, *r.AltNames); err != nil {
		return nil, err
	} else {
		result = append(result, files...)
	}

	etcdURL, err := url.Parse(r.etcdServerURL)
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
	result = append(result, r.newAdminKubeConfig())

	return &result, nil
}
