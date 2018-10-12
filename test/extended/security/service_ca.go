package security

import (
	"crypto/rsa"
	"crypto/x509"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	serviceName               = "test-service"
	secretName                = "test-secret"
	configMapName             = "test-config"
	servingCertAnnotation     = "service.beta.openshift.io/serving-cert-secret-name"
	configMapBundleAnnotation = "service.beta.openshift.io/inject-cabundle"
	configMapBundleDataKey    = "service-ca.crt"
	signerCertNamespace       = "openshift-service-ca"
	signerCertSecretName      = "service-serving-cert-signer-signing-key"
	testDataKey               = "test"
)

var _ = g.Describe("[Feature:ServiceCA] Services that request serving certificates", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("service-ca", exutil.KubeConfigPath())
		f  = oc.KubeFramework()
	)

	g.It("see a created secret and configmap", func() {
		g.By("an annotated service resulting in a generated serving cert")
		_, err := f.ClientSet.CoreV1().Services(f.Namespace.Name).Create(annotatedService())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer f.ClientSet.CoreV1().Services(f.Namespace.Name).Delete(serviceName, nil)

		generatedSecret, err := pollForServiceServingSecret(f.ClientSet, f.Namespace.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Delete(secretName, nil)

		g.By("an annotated configmap injected with a signer CA bundle data item")
		_, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(annotatedConfigMap(configMapName, testDataKey, testDataKey))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Delete(configMapName, nil)

		injectedCABundle, err := pollForConfigMapCABundle(f.ClientSet, f.Namespace.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check that the injected CA bundle matches the signer CA
		signer, err := f.ClientSet.CoreV1().Secrets(signerCertNamespace).Get(signerCertSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		caCert, ok := signer.Data[kapiv1.TLSCertKey]
		o.Expect(ok).NotTo(o.BeFalse())
		o.Expect(caCert).To(o.BeEquivalentTo(injectedCABundle))

		serverCert, ok := generatedSecret.Data[kapiv1.TLSCertKey]
		o.Expect(ok).NotTo(o.BeFalse())
		serverKey, ok := generatedSecret.Data[kapiv1.TLSPrivateKeyKey]
		o.Expect(ok).NotTo(o.BeFalse())

		// Check that the serving cert may be trusted via the injected CA bundle
		verifyServerCertCreds(serverCert, serverKey, injectedCABundle)
		confMap, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Get(configMapName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// The configMap should stomp on the test data
		o.Expect(confMap.Data[testDataKey]).To(o.BeEmpty())
	})
})

func verifyServerCertCreds(certPem, keyPem []byte, caBundle string) {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caBundle))
	certs, err := certutil.ParseCertsPEM(certPem)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(certs).NotTo(o.BeEmpty())
	serverCert := certs[0]
	_, err = serverCert.Verify(x509.VerifyOptions{
		DNSName:   "",
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	key, err := certutil.ParsePrivateKeyPEM(keyPem)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, ok := key.(*rsa.PrivateKey)
	o.Expect(ok).NotTo(o.BeFalse())
}

func annotatedService() *kapiv1.Service {
	return &kapiv1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Annotations: map[string]string{
				servingCertAnnotation: secretName,
			},
		},
		Spec: kapiv1.ServiceSpec{
			Ports: []kapiv1.ServicePort{
				{
					Name: "tests",
					Port: 8443,
				},
			},
		},
	}
}

func annotatedConfigMap(name, testKey, testData string) *kapiv1.ConfigMap {
	return &kapiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				configMapBundleAnnotation: "true",
			},
		},
		Data: map[string]string{
			testKey: testData,
		},
	}
}

func pollForServiceServingSecret(client kubernetes.Interface, namespace string) (*kapiv1.Secret, error) {
	var secret *kapiv1.Secret
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		s, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		secret = s
		return true, nil
	})
	return secret, err
}

func pollForConfigMapCABundle(client kubernetes.Interface, namespace string) (string, error) {
	caBundle := ""
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		cm, err := client.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ca, ok := cm.Data[configMapBundleDataKey]
		if !ok || len(ca) == 0 {
			return false, nil
		}

		caBundle = ca
		return true, nil
	})
	return caBundle, err
}
