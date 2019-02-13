package certrotation

import (
	gcrypto "crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io/ioutil"
	"math/big"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/util/cert"

	"github.com/davecgh/go-spew/spew"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestEnsureConfigMapCABundle(t *testing.T) {
	tests := []struct {
		name string

		initialConfigMapFn func() *corev1.ConfigMap
		caFn               func() (*crypto.CA, error)

		verifyActions func(t *testing.T, client *kubefake.Clientset)
		expectedError string
	}{
		{
			name: "initial create",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialConfigMapFn: func() *corev1.ConfigMap { return nil },
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("get", "configmaps") {
					t.Error(actions[0])
				}
				if !actions[1].Matches("create", "configmaps") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.CreateAction).GetObject().(*corev1.ConfigMap)
				if len(actual.Data["ca-bundle.crt"]) == 0 {
					t.Error(actual.Data)
				}
			},
		},
		{
			name: "update keep both",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialConfigMapFn: func() *corev1.ConfigMap {
				caBundleConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "trust-bundle"},
					Data:       map[string]string{},
				}
				certs, err := newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
				if err != nil {
					t.Fatal(err)
				}
				caBytes, err := crypto.EncodeCertificates(certs.Config.Certs...)
				if err != nil {
					t.Fatal(err)
				}
				caBundleConfigMap.Data["ca-bundle.crt"] = string(caBytes)
				return caBundleConfigMap
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[1].Matches("update", "configmaps") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if len(actual.Data["ca-bundle.crt"]) == 0 {
					t.Error(actual.Data)
				}
				result, err := cert.ParseCertsPEM([]byte(actual.Data["ca-bundle.crt"]))
				if err != nil {
					t.Fatal(err)
				}
				if len(result) != 2 {
					t.Error(len(result))
				}
			},
		},
		{
			name: "update remove old",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialConfigMapFn: func() *corev1.ConfigMap {
				caBundleConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "trust-bundle"},
					Data:       map[string]string{},
				}
				certs, err := newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
				if err != nil {
					t.Fatal(err)
				}
				caBytes, err := crypto.EncodeCertificates(certs.Config.Certs[0], certs.Config.Certs[0])
				if err != nil {
					t.Fatal(err)
				}
				caBundleConfigMap.Data["ca-bundle.crt"] = string(caBytes)
				return caBundleConfigMap
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[1].Matches("update", "configmaps") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if len(actual.Data["ca-bundle.crt"]) == 0 {
					t.Error(actual.Data)
				}
				result, err := cert.ParseCertsPEM([]byte(actual.Data["ca-bundle.crt"]))
				if err != nil {
					t.Fatal(err)
				}
				if len(result) != 2 {
					t.Error(len(result))
				}
			},
		},
		{
			name: "update remove duplicate",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialConfigMapFn: func() *corev1.ConfigMap {
				caBundleConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "trust-bundle"},
					Data:       map[string]string{},
				}
				certBytes, err := ioutil.ReadFile("./testfiles/tls-expired.crt")
				if err != nil {
					t.Fatal(err)
				}
				certs, err := cert.ParseCertsPEM(certBytes)
				if err != nil {
					t.Fatal(err)
				}
				caBytes, err := crypto.EncodeCertificates(certs...)
				if err != nil {
					t.Fatal(err)
				}
				caBundleConfigMap.Data["ca-bundle.crt"] = string(caBytes)
				return caBundleConfigMap
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[1].Matches("update", "configmaps") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if len(actual.Data["ca-bundle.crt"]) == 0 {
					t.Error(actual.Data)
				}
				result, err := cert.ParseCertsPEM([]byte(actual.Data["ca-bundle.crt"]))
				if err != nil {
					t.Fatal(err)
				}
				if len(result) != 1 {
					t.Error(len(result))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

			client := kubefake.NewSimpleClientset()
			if startingObj := test.initialConfigMapFn(); startingObj != nil {
				indexer.Add(startingObj)
				client = kubefake.NewSimpleClientset(startingObj)
			}

			c := &CABundleRotation{
				Namespace: "ns",
				Name:      "trust-bundle",

				Client:        client.CoreV1(),
				Lister:        corev1listers.NewConfigMapLister(indexer),
				EventRecorder: events.NewInMemoryRecorder("test"),
			}

			newCA, err := test.caFn()
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.ensureConfigMapCABundle(newCA)
			switch {
			case err != nil && len(test.expectedError) == 0:
				t.Error(err)
			case err != nil && !strings.Contains(err.Error(), test.expectedError):
				t.Error(err)
			case err == nil && len(test.expectedError) != 0:
				t.Errorf("missing %q", test.expectedError)
			}

			test.verifyActions(t, client)
		})
	}
}

// NewCACertificate generates and signs new CA certificate and key.
func newTestCACertificate(subject pkix.Name, serialNumber int64, validity metav1.Duration, currentTime func() time.Time) (*crypto.CA, error) {
	caPublicKey, caPrivateKey, err := crypto.NewKeyPair()
	if err != nil {
		return nil, err
	}

	caCert := &x509.Certificate{
		Subject: subject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    currentTime().Add(-1 * time.Second),
		NotAfter:     currentTime().Add(validity.Duration),
		SerialNumber: big.NewInt(serialNumber),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA: true,
	}

	cert, err := signCertificate(caCert, caPublicKey, caCert, caPrivateKey)
	if err != nil {
		return nil, err
	}

	return &crypto.CA{
		Config: &crypto.TLSCertificateConfig{
			Certs: []*x509.Certificate{cert},
			Key:   caPrivateKey,
		},
		SerialGenerator: &crypto.RandomSerialGenerator{},
	}, nil
}

func signCertificate(template *x509.Certificate, requestKey gcrypto.PublicKey, issuer *x509.Certificate, issuerKey gcrypto.PrivateKey) (*x509.Certificate, error) {
	derBytes, err := x509.CreateCertificate(rand.Reader, template, issuer, requestKey, issuerKey)
	if err != nil {
		return nil, err
	}
	certs, err := x509.ParseCertificates(derBytes)
	if err != nil {
		return nil, err
	}
	if len(certs) != 1 {
		return nil, errors.New("Expected a single certificate")
	}
	return certs[0], nil
}
