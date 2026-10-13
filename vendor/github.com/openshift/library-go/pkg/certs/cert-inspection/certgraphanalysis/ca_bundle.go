package certgraphanalysis

import (
	"crypto/x509"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func toCABundle(certificates []*x509.Certificate) (*certgraphapi.CertificateAuthorityBundle, error) {
	ret := &certgraphapi.CertificateAuthorityBundle{
		Spec: certgraphapi.CertificateAuthorityBundleSpec{},
	}

	certNames := []string{}
	for _, cert := range certificates {
		metadata := toCertKeyMetadata(cert)
		ret.Spec.CertificateMetadata = append(ret.Spec.CertificateMetadata, metadata)
		certNames = append(certNames, metadata.CertIdentifier.CommonName)
	}
	ret.Name = strings.Join(certNames, "|")

	return ret, nil
}

func addConfigMapLocation(in *certgraphapi.CertificateAuthorityBundle, namespace, name string) *certgraphapi.CertificateAuthorityBundle {
	secretLocation := certgraphapi.InClusterConfigMapLocation{
		Namespace: namespace,
		Name:      name,
	}
	out := in.DeepCopy()
	for _, curr := range in.Spec.ConfigMapLocations {
		if curr == secretLocation {
			return out
		}
	}

	out.Spec.ConfigMapLocations = append(out.Spec.ConfigMapLocations, secretLocation)
	return out
}
