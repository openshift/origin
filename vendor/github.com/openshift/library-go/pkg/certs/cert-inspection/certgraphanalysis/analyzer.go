package certgraphanalysis

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

func InspectSecret(obj *corev1.Secret) (*certgraphapi.CertKeyPair, error) {
	resourceString := fmt.Sprintf("secrets/%s[%s]", obj.Name, obj.Namespace)
	tlsCrt, isTLS := obj.Data["tls.crt"]
	if !isTLS {
		if detail, err := InspectSecretAsKubeConfig(obj); err == nil {
			return detail, nil
		}
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
		if detail, err := InspectConfigMapAsKubeConfig(obj); err == nil {
			return detail, nil
		}
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

func extractKubeConfigFromConfigMap(obj *corev1.ConfigMap) (*rest.Config, error) {
	if obj == nil {
		return nil, fmt.Errorf("empty object")
	}
	for _, v := range obj.Data {
		kubeConfig, err := clientcmd.NewClientConfigFromBytes([]byte(v))
		if err != nil {
			continue
		}
		if clientConfig, err := kubeConfig.ClientConfig(); err == nil {
			return clientConfig, nil
		}
	}
	return nil, nil
}

func extractKubeConfigFromSecret(obj *corev1.Secret) (*rest.Config, error) {
	if obj == nil {
		return nil, fmt.Errorf("empty object")
	}
	for _, v := range obj.Data {
		kubeConfig, err := clientcmd.NewClientConfigFromBytes(v)
		if err != nil {
			continue
		}
		if clientConfig, err := kubeConfig.ClientConfig(); err == nil {
			return clientConfig, nil
		}
	}
	return nil, nil
}

func GetCAFromKubeConfig(kubeConfig *rest.Config, namespace, name string) (*certgraphapi.CertificateAuthorityBundle, error) {
	if kubeConfig == nil {
		return nil, fmt.Errorf("empty kubeconfig")
	}
	certificates, err := cert.ParseCertsPEM(kubeConfig.CAData)
	if err != nil {
		return nil, err
	}
	caBundleDetail, err := toCABundle(certificates)
	if err != nil {
		return nil, err
	}
	if len(namespace) > 0 && len(name) > 0 {
		caBundleDetail = addConfigMapLocation(caBundleDetail, namespace, name)
	}
	return caBundleDetail, nil
}

func GetCertKeyPairFromKubeConfig(kubeConfig *rest.Config, namespace, name string) (*certgraphapi.CertKeyPair, error) {
	if kubeConfig == nil {
		return nil, fmt.Errorf("empty kubeconfig")
	}
	certificates, err := cert.ParseCertsPEM(kubeConfig.CertData)
	if err != nil {
		return nil, err
	}
	for _, certificate := range certificates {
		detail, err := toCertKeyPair(certificate)
		if err != nil {
			return nil, err
		}
		if len(namespace) > 0 && len(name) > 0 {
			detail = addSecretLocation(detail, namespace, name)
		}
		return detail, nil
	}
	return nil, fmt.Errorf("didn't see that coming")
}

func InspectConfigMapAsKubeConfig(obj *corev1.ConfigMap) (*certgraphapi.CertificateAuthorityBundle, error) {
	kubeConfig, err := extractKubeConfigFromConfigMap(obj)
	if err != nil {
		return nil, err
	}
	return GetCAFromKubeConfig(kubeConfig, obj.Namespace, obj.Name)
}

func InspectSecretAsKubeConfig(obj *corev1.Secret) (*certgraphapi.CertKeyPair, error) {
	kubeConfig, err := extractKubeConfigFromSecret(obj)
	if err != nil {
		return nil, err
	}
	return GetCertKeyPairFromKubeConfig(kubeConfig, obj.Namespace, obj.Name)
}
