package certgraphanalysis

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type caBundleRewriteFunc func(caBundle *certgraphapi.CertificateAuthorityBundle)

type certKeyPairRewriteFunc func(certKeyPair *certgraphapi.CertKeyPair)

type metadataOptions struct {
	rewriteCABundle    caBundleRewriteFunc
	rewriteCertKeyPair certKeyPairRewriteFunc
}

func (metadataOptions) approved() {}

var (
	ElideProxyCADetails = &metadataOptions{
		rewriteCABundle: func(caBundle *certgraphapi.CertificateAuthorityBundle) {
			isProxyCA := false
			for _, location := range caBundle.Spec.ConfigMapLocations {
				if location.Namespace == "openshift-config-managed" && location.Name == "trusted-ca-bundle" {
					isProxyCA = true
				}
			}
			if !isProxyCA {
				return
			}
			if len(caBundle.Spec.CertificateMetadata) < 10 {
				return
			}
			caBundle.Name = "proxy-ca"
			caBundle.LogicalName = "proxy-ca"
			caBundle.Spec.CertificateMetadata = []certgraphapi.CertKeyMetadata{
				{
					CertIdentifier: certgraphapi.CertIdentifier{
						CommonName:   "synthetic-proxy-ca",
						SerialNumber: "0",
						Issuer:       nil,
					},
				},
			}
		},
	}
)
