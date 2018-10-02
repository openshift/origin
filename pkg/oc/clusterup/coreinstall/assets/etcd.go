package assets

import (
	"crypto/rsa"
	"crypto/x509"
	"net/url"

	assetslib "github.com/openshift/library-go/pkg/assets"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/assets/tlsutil"
)

const (
	AssetPathEtcdClientCA   = "tls/etcd-client-ca.crt"
	AssetPathEtcdClientKey  = "tls/etcd-client.key"
	AssetPathEtcdClientCert = "tls/etcd-client.crt"

	AssetPathEtcdPeerCA     = "tls/etcd-peer-ca.crt"
	AssetPathEtcdPeerKey    = "tls/etcd-peer.key"
	AssetPathEtcdPeerCert   = "tls/etcd-peer.crt"
	AssetPathEtcdServerKey  = "tls/etcd-server.key"
	AssetPathEtcdServerCert = "tls/etcd-server.crt"
)

func (r *TLSAssetsRenderOptions) newEtcdTLSAssets(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, etcdServers []*url.URL) ([]assetslib.Asset, error) {
	var assets []assetslib.Asset

	// Create an etcd client cert.
	etcdClientCAPrivKey, etcdClientCACert, err := r.newCACert()
	if err != nil {
		return nil, err
	}
	etcdClientKey, etcdClientCert, err := r.newEtcdKeyAndCert(etcdClientCACert, etcdClientCAPrivKey, "etcd-client", etcdServers)
	if err != nil {
		return nil, err
	}

	// Create an etcd peer cert (not consumed by self-hosted components).
	etcdPeerCAPrivKey, etcdPeerCACert, err := r.newCACert()
	if err != nil {
		return nil, err
	}
	etcdPeerKey, etcdPeerCert, err := r.newEtcdKeyAndCert(etcdPeerCACert, etcdPeerCAPrivKey, "etcd-peer", etcdServers)
	if err != nil {
		return nil, err
	}
	etcdServerKey, etcdServerCert, err := r.newEtcdKeyAndCert(caCert, caPrivKey, "etcd-server", etcdServers)
	if err != nil {
		return nil, err
	}

	assets = append(assets, []assetslib.Asset{
		{Name: AssetPathEtcdPeerCA, Data: tlsutil.EncodeCertificatePEM(etcdPeerCACert)},
		{Name: AssetPathEtcdPeerKey, Data: tlsutil.EncodePrivateKeyPEM(etcdPeerKey)},
		{Name: AssetPathEtcdPeerCert, Data: tlsutil.EncodeCertificatePEM(etcdPeerCert)},
		{Name: AssetPathEtcdServerKey, Data: tlsutil.EncodePrivateKeyPEM(etcdServerKey)},
		{Name: AssetPathEtcdServerCert, Data: tlsutil.EncodeCertificatePEM(etcdServerCert)},
	}...)

	assets = append(assets, []assetslib.Asset{
		{Name: AssetPathEtcdClientCA, Data: tlsutil.EncodeCertificatePEM(etcdClientCACert)},
		{Name: AssetPathEtcdClientKey, Data: tlsutil.EncodePrivateKeyPEM(etcdClientKey)},
		{Name: AssetPathEtcdClientCert, Data: tlsutil.EncodeCertificatePEM(etcdClientCert)},
	}...)

	return assets, nil
}

func (r *TLSAssetsRenderOptions) newEtcdKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, commonName string, etcdServers []*url.URL) (*rsa.PrivateKey, *x509.Certificate, error) {
	addrs := make([]string, len(etcdServers))
	for i := range etcdServers {
		addrs[i] = etcdServers[i].Hostname()
	}
	return r.newKeyAndCert(caCert, caPrivKey, commonName, addrs)
}
