package assets

import (
	"crypto/rsa"
	"crypto/x509"

	assetslib "github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/assets/tlsutil"
)

const (
	AssetPathCAKey                 = "tls/ca.key"
	AssetPathCACert                = "tls/ca.crt"
	AssetPathAPIServerKey          = "tls/apiserver.key"
	AssetPathAPIServerCert         = "tls/apiserver.crt"
	AssetPathServiceAccountPrivKey = "tls/service-account.key"
	AssetPathServiceAccountPubKey  = "tls/service-account.pub"
	AssetPathAdminKey              = "tls/admin.key"
	AssetPathAdminCert             = "tls/admin.crt"
)

func (r *TLSAssetsRenderOptions) newAPIKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	altNames.DNSNames = append(altNames.DNSNames, []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
	}...)

	config := tlsutil.CertConfig{
		CommonName:   "kube-apiserver",
		Organization: []string{"kube-master"},
		AltNames:     altNames,
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

func (r *TLSAssetsRenderOptions) newTLSAssets(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) ([]assetslib.Asset, error) {
	var (
		assets []assetslib.Asset
		err    error
	)

	apiKey, apiCert, err := r.newAPIKeyAndCert(caCert, caPrivKey, altNames)
	if err != nil {
		return assets, err
	}

	saPrivKey, err := tlsutil.NewPrivateKey()
	if err != nil {
		return assets, err
	}

	saPubKey, err := tlsutil.EncodePublicKeyPEM(&saPrivKey.PublicKey)
	if err != nil {
		return assets, err
	}

	adminKey, adminCert, err := r.newAdminKeyAndCert(caCert, caPrivKey)
	if err != nil {
		return assets, err
	}

	r.adminKey = adminKey
	r.adminCert = adminCert

	assets = append(assets, []assetslib.Asset{
		{Name: AssetPathCAKey, Data: tlsutil.EncodePrivateKeyPEM(caPrivKey)},
		{Name: AssetPathCACert, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathAPIServerKey, Data: tlsutil.EncodePrivateKeyPEM(apiKey)},
		{Name: AssetPathAPIServerCert, Data: tlsutil.EncodeCertificatePEM(apiCert)},
		{Name: AssetPathServiceAccountPrivKey, Data: tlsutil.EncodePrivateKeyPEM(saPrivKey)},
		{Name: AssetPathServiceAccountPubKey, Data: saPubKey},
		{Name: AssetPathAdminKey, Data: tlsutil.EncodePrivateKeyPEM(adminKey)},
		{Name: AssetPathAdminCert, Data: tlsutil.EncodeCertificatePEM(adminCert)},
	}...)
	return assets, nil
}
