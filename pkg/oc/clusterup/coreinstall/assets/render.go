package assets

import (
	"net"
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
	AdminKey      []byte
	AdminCert     []byte
	CACert        []byte
}

func NewTLSAssetsRenderer(hostname string) *TLSAssetsRenderOptions {
	var altNames tlsutil.AltNames
	if len(hostname) > 0 {
		if ip := net.ParseIP(hostname); ip == nil {
			altNames.DNSNames = append(altNames.DNSNames, hostname)
		} else {
			altNames.IPs = append(altNames.IPs, ip)
		}
	}
	return &TLSAssetsRenderOptions{
		AltNames: altNames,
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
	r.config.CACert = tlsutil.EncodeCertificatePEM(caCert)

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

	localEtcdURL, _ := url.Parse("https://127.0.0.1:2379")

	// Generate etcd certs
	if files, err := r.newEtcdTLSAssets(caCert, caPrivateKey, []*url.URL{
		etcdURL,
		localEtcdURL,
	}); err != nil {
		return nil, err
	} else {
		result = append(result, files...)
	}

	// Generate admin.kubeconfig
	result = append(result, r.newAdminKubeConfig()...)

	return &result, nil
}
