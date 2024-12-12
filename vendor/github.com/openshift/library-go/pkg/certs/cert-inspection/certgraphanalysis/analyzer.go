package certgraphanalysis

import (
	"fmt"
	"slices"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/cert"
)

var caBundleKeys = []string{
	"ca-bundle.crt",
	"client-ca-file",
	"client-ca.crt",
	"metrics-ca-bundle.crt",
	"requestheader-client-ca-file",
	"image-registry.openshift-image-registry.svc..5000",
	"image-registry.openshift-image-registry.svc.cluster.local..5000",
}

func InspectSecret(obj *corev1.Secret) ([]*certgraphapi.CertKeyPair, error) {
	tlsCrt, isTLS := obj.Data["tls.crt"]
	if !isTLS || len(tlsCrt) == 0 {
		if detail, err := InspectSecretAsKubeConfig(obj); err == nil {
			return detail, nil
		}
		return nil, nil
	}
	return extractCertKeyPairsFromBytes("secret", &obj.ObjectMeta, tlsCrt)
}

func extractCertKeyPairsFromBytes(resourceType string, obj *metav1.ObjectMeta, certificate []byte) ([]*certgraphapi.CertKeyPair, error) {
	resourceString := ""
	if obj != nil {
		resourceString = fmt.Sprintf("%s/%s[%s]", resourceType, obj.Name, obj.Namespace)
	}
	if len(certificate) == 0 {
		return nil, fmt.Errorf("%s MISSING issued certificate\n", resourceString)
	}

	certKeyPairDetails := []*certgraphapi.CertKeyPair{}
	certificates, err := cert.ParseCertsPEM([]byte(certificate))
	if err != nil {
		return nil, err
	}
	for _, certificate := range certificates {
		detail, err := toCertKeyPair(certificate)
		if err != nil {
			return nil, err
		}
		if resourceType == "secret" && obj != nil {
			detail = addSecretLocation(detail, obj.Namespace, obj.Name)
		}
		certKeyPairDetails = append(certKeyPairDetails, detail)
	}
	return certKeyPairDetails, nil
}

func InspectCSR(obj *certificatesv1.CertificateSigningRequest) ([]*certgraphapi.CertKeyPair, error) {
	return extractCertKeyPairsFromBytes("csr", &obj.ObjectMeta, obj.Status.Certificate)
}

func InspectConfigMap(obj *corev1.ConfigMap) (*certgraphapi.CertificateAuthorityBundle, error) {
	if details, err := InspectConfigMapAsKubeConfig(obj); err == nil {
		return details, nil
	}

	var caBundle string
	for key := range obj.Data {
		if !slices.Contains(caBundleKeys, key) {
			continue
		}
		if value := obj.Data[key]; len(value) > 0 {
			caBundle = value
			break
		}
	}

	if len(caBundle) == 0 {
		return nil, nil
	}

	certificates, err := cert.ParseCertsPEM([]byte(caBundle))
	if err != nil {
		return nil, nil
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

func extractKubeConfigFromSecret(obj *corev1.Secret) (*clientcmdapi.Config, error) {
	if obj == nil {
		return nil, fmt.Errorf("empty object")
	}
	for _, v := range obj.Data {
		clientConfig, err := clientcmd.NewClientConfigFromBytes(v)
		if err != nil {
			continue
		}
		if kubeConfig, err := clientConfig.RawConfig(); err == nil && kubeConfig.CurrentContext != "" {
			return &kubeConfig, nil
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
		return nil, nil
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

func GetCertKeyPairsFromKubeConfig(authInfo *clientcmdapi.AuthInfo, obj *metav1.ObjectMeta) ([]*certgraphapi.CertKeyPair, error) {
	if authInfo == nil {
		return nil, fmt.Errorf("empty authinfo")
	}
	return extractCertKeyPairsFromBytes("secret", obj, authInfo.ClientCertificateData)
}

func InspectConfigMapAsKubeConfig(obj *corev1.ConfigMap) (*certgraphapi.CertificateAuthorityBundle, error) {
	kubeConfig, err := extractKubeConfigFromConfigMap(obj)
	if err != nil {
		return nil, err
	}
	if kubeConfig == nil {
		return nil, fmt.Errorf("empty kubeconfig")
	}

	return GetCAFromKubeConfig(kubeConfig, obj.Namespace, obj.Name)
}

func InspectSecretAsKubeConfig(obj *corev1.Secret) ([]*certgraphapi.CertKeyPair, error) {
	kubeConfig, err := extractKubeConfigFromSecret(obj)
	if err != nil {
		return nil, err
	}
	if kubeConfig == nil {
		return nil, fmt.Errorf("empty kubeconfig")
	}
	certKeyPairInfos := []*certgraphapi.CertKeyPair{}
	for _, v := range kubeConfig.AuthInfos {
		certKeyPairInfo, err := GetCertKeyPairsFromKubeConfig(v, &obj.ObjectMeta)
		if err != nil {
			continue
		}
		certKeyPairInfos = append(certKeyPairInfos, certKeyPairInfo...)
	}
	return certKeyPairInfos, nil
}
