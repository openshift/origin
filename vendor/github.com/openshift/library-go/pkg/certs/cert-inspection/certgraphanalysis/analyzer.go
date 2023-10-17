package certgraphanalysis

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/cert"
)

func InspectSecret(obj *corev1.Secret) (*certgraphapi.CertKeyPair, error) {
	resourceString := fmt.Sprintf("secrets/%s[%s]", obj.Name, obj.Namespace)
	tlsCrt, isTLS := obj.Data["tls.crt"]
	if !isTLS {
		return nil, nil
	}
	//fmt.Printf("%s - tls (%v)\n", resourceString, obj.CreationTimestamp.UTC())
	if len(tlsCrt) == 0 {
		return nil, fmt.Errorf("%s MISSING tls.crt content\n", resourceString)
	}

	certificates, err := cert.ParseCertsPEM([]byte(tlsCrt))
	if err != nil {
		return nil, err
	}
	for _, certificate := range certificates {
		detail, err := toCertKeyPair(certificate)
		if err != nil {
			return nil, err
		}
		detail = addSecretLocation(detail, obj.Namespace, obj.Name)
		return detail, nil
	}
	return nil, fmt.Errorf("didn't see that coming")
}

func inspectCSR(resourceString, objName string, certificate []byte) (*certgraphapi.CertKeyPair, error) {
	if len(certificate) == 0 {
		return nil, fmt.Errorf("%s MISSING issued certificate\n", resourceString)
	}

	certificates, err := cert.ParseCertsPEM([]byte(certificate))
	if err != nil {
		return nil, err
	}
	for _, certificate := range certificates {
		detail, err := toCertKeyPair(certificate)
		if err != nil {
			return nil, err
		}
		return detail, nil
	}
	return nil, fmt.Errorf("didn't see that coming")
}

func InspectCSR(obj *certificatesv1.CertificateSigningRequest) (*certgraphapi.CertKeyPair, error) {
	resourceString := fmt.Sprintf("csr/%s[%s]", obj.Name, obj.Namespace)
	return inspectCSR(resourceString, obj.Name, obj.Status.Certificate)
}

func InspectConfigMap(obj *corev1.ConfigMap) (*certgraphapi.CertificateAuthorityBundle, error) {
	caBundle, ok := obj.Data["ca-bundle.crt"]
	if !ok {
		return nil, nil
	}
	if len(caBundle) == 0 {
		return nil, nil
	}

	certificates, err := cert.ParseCertsPEM([]byte(caBundle))
	if err != nil {
		return nil, err
	}
	caBundleDetail, err := toCABundle(certificates)
	if err != nil {
		return nil, err
	}
	caBundleDetail = addConfigMapLocation(caBundleDetail, obj.Namespace, obj.Name)

	return caBundleDetail, nil
}
