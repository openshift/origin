package certrotation

import (
	"crypto/x509/pkix"
	"strings"
	"testing"
	"time"

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

func TestNeedNewTargetCertKeyPairForTime(t *testing.T) {
	now := time.Now()
	nowFn := func() time.Time { return now }
	elevenMinutesBeforeNow := time.Now().Add(-11 * time.Minute)
	elevenMinutesBeforeNowFn := func() time.Time { return elevenMinutesBeforeNow }
	nowCert, err := newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: 200 * time.Minute}, nowFn)
	if err != nil {
		t.Fatal(err)
	}
	elevenMinutesBeforeNowCert, err := newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: 200 * time.Minute}, elevenMinutesBeforeNowFn)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string

		annotations map[string]string
		signerFn    func() (*crypto.CA, error)
		refresh     time.Duration

		expected string
	}{
		{
			name: "from nothing",
			signerFn: func() (*crypto.CA, error) {
				return nowCert, nil
			},
			refresh:  50 * time.Minute,
			expected: "missing notAfter",
		},
		{
			name: "malformed",
			annotations: map[string]string{
				CertificateNotAfterAnnotation:  "malformed",
				CertificateNotBeforeAnnotation: now.Add(-45 * time.Minute).Format(time.RFC3339),
			},
			signerFn: func() (*crypto.CA, error) {
				return nowCert, nil
			},
			refresh:  50 * time.Minute,
			expected: `bad expiry: "malformed"`,
		},
		{
			name: "past midpoint and cert is ready",
			annotations: map[string]string{
				CertificateNotAfterAnnotation:  now.Add(45 * time.Minute).Format(time.RFC3339),
				CertificateNotBeforeAnnotation: now.Add(-45 * time.Minute).Format(time.RFC3339),
			},
			signerFn: func() (*crypto.CA, error) {
				return elevenMinutesBeforeNowCert, nil
			},
			refresh:  40 * time.Minute,
			expected: "past its refresh time",
		},
		{
			name: "past midpoint and cert is new",
			annotations: map[string]string{
				CertificateNotAfterAnnotation:  now.Add(45 * time.Minute).Format(time.RFC3339),
				CertificateNotBeforeAnnotation: now.Add(-45 * time.Minute).Format(time.RFC3339),
			},
			signerFn: func() (*crypto.CA, error) {
				return nowCert, nil
			},
			refresh:  40 * time.Minute,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signer, err := test.signerFn()
			if err != nil {
				t.Fatal(err)
			}

			actual := needNewTargetCertKeyPairForTime(test.annotations, signer, test.refresh)
			if !strings.HasPrefix(actual, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestEnsureTargetCertKeyPair(t *testing.T) {
	tests := []struct {
		name string

		initialSecretFn func() *corev1.Secret
		caFn            func() (*crypto.CA, error)

		verifyActions func(t *testing.T, client *kubefake.Clientset)
		expectedError string
	}{
		{
			name: "initial create",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialSecretFn: func() *corev1.Secret { return nil },
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("get", "secrets") {
					t.Error(actions[0])
				}
				if !actions[1].Matches("create", "secrets") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.CreateAction).GetObject().(*corev1.Secret)
				if len(actual.Data["tls.crt"]) == 0 || len(actual.Data["tls.key"]) == 0 {
					t.Error(actual.Data)
				}
			},
		},
		{
			name: "update write",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialSecretFn: func() *corev1.Secret {
				caBundleSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "target-secret"},
					Data:       map[string][]byte{},
				}
				return caBundleSecret
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[1].Matches("update", "secrets") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.Secret)
				if len(actual.Data["tls.crt"]) == 0 || len(actual.Data["tls.key"]) == 0 {
					t.Error(actual.Data)
				}
				if actual.Annotations[CertificateHostnames] != "bar,foo" {
					t.Error(actual.Annotations[CertificateHostnames])
				}

			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

			client := kubefake.NewSimpleClientset()
			if startingObj := test.initialSecretFn(); startingObj != nil {
				indexer.Add(startingObj)
				client = kubefake.NewSimpleClientset(startingObj)
			}

			c := &TargetRotation{
				Namespace: "ns",
				Validity:  24 * time.Hour,
				Refresh:   12 * time.Hour,
				Name:      "target-secret",
				CertCreator: &ServingRotation{
					Hostnames: func() []string { return []string{"foo", "bar"} },
				},

				Client:        client.CoreV1(),
				Lister:        corev1listers.NewSecretLister(indexer),
				EventRecorder: events.NewInMemoryRecorder("test"),
			}

			newCA, err := test.caFn()
			if err != nil {
				t.Fatal(err)
			}
			err = c.ensureTargetCertKeyPair(newCA, newCA.Config.Certs)
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

func TestServerHostnameCheck(t *testing.T) {
	tests := []struct {
		name string

		existingHostnames string
		requiredHostnames []string

		expected string
	}{
		{
			name:              "nothing",
			existingHostnames: "",
			requiredHostnames: []string{"foo"},
			expected:          `"" are existing and not required, "foo" are required and not existing`,
		},
		{
			name:              "exists",
			existingHostnames: "foo",
			requiredHostnames: []string{"foo"},
			expected:          "",
		},
		{
			name:              "hasExtra",
			existingHostnames: "foo,bar",
			requiredHostnames: []string{"foo"},
			expected:          `"bar" are existing and not required, "" are required and not existing`,
		},
		{
			name:              "needsAnother",
			existingHostnames: "foo",
			requiredHostnames: []string{"foo", "bar"},
			expected:          `"" are existing and not required, "bar" are required and not existing`,
		},
		{
			name:              "both",
			existingHostnames: "foo,baz",
			requiredHostnames: []string{"foo", "bar"},
			expected:          `"baz" are existing and not required, "bar" are required and not existing`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &ServingRotation{
				Hostnames: func() []string { return test.requiredHostnames },
			}
			actual := r.missingHostnames(map[string]string{CertificateHostnames: test.existingHostnames})
			if actual != test.expected {
				t.Fatal(actual)
			}
		})
	}
}

func TestEnsureTargetSignerCertKeyPair(t *testing.T) {
	tests := []struct {
		name string

		initialSecretFn func() *corev1.Secret
		caFn            func() (*crypto.CA, error)

		verifyActions func(t *testing.T, client *kubefake.Clientset)
		expectedError string
	}{
		{
			name: "initial create",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialSecretFn: func() *corev1.Secret { return nil },
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("get", "secrets") {
					t.Error(actions[0])
				}
				if !actions[1].Matches("create", "secrets") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.CreateAction).GetObject().(*corev1.Secret)
				if len(actual.Data["tls.crt"]) == 0 || len(actual.Data["tls.key"]) == 0 {
					t.Error(actual.Data)
				}

				signingCertKeyPair, err := crypto.GetCAFromBytes(actual.Data["tls.crt"], actual.Data["tls.key"])
				if err != nil {
					t.Error(actual.Data)
				}
				if signingCertKeyPair.Config.Certs[0].Issuer.CommonName != "signer-tests" {
					t.Error(signingCertKeyPair.Config.Certs[0].Issuer.CommonName)

				}
				if signingCertKeyPair.Config.Certs[1].Subject.CommonName != "signer-tests" {
					t.Error(signingCertKeyPair.Config.Certs[0].Issuer.CommonName)
				}
			},
		},
		{
			name: "update write",
			caFn: func() (*crypto.CA, error) {
				return newTestCACertificate(pkix.Name{CommonName: "signer-tests"}, int64(1), metav1.Duration{Duration: time.Hour * 24 * 60}, time.Now)
			},
			initialSecretFn: func() *corev1.Secret {
				caBundleSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "target-secret"},
					Data:       map[string][]byte{},
				}
				return caBundleSecret
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				actions := client.Actions()
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[1].Matches("update", "secrets") {
					t.Error(actions[1])
				}

				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.Secret)
				if len(actual.Data["tls.crt"]) == 0 || len(actual.Data["tls.key"]) == 0 {
					t.Error(actual.Data)
				}

				signingCertKeyPair, err := crypto.GetCAFromBytes(actual.Data["tls.crt"], actual.Data["tls.key"])
				if err != nil {
					t.Error(actual.Data)
				}
				if signingCertKeyPair.Config.Certs[0].Issuer.CommonName != "signer-tests" {
					t.Error(signingCertKeyPair.Config.Certs[0].Issuer.CommonName)

				}
				if signingCertKeyPair.Config.Certs[1].Subject.CommonName != "signer-tests" {
					t.Error(signingCertKeyPair.Config.Certs[0].Issuer.CommonName)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

			client := kubefake.NewSimpleClientset()
			if startingObj := test.initialSecretFn(); startingObj != nil {
				indexer.Add(startingObj)
				client = kubefake.NewSimpleClientset(startingObj)
			}

			c := &TargetRotation{
				Namespace: "ns",
				Validity:  24 * time.Hour,
				Refresh:   12 * time.Hour,
				Name:      "target-secret",
				CertCreator: &SignerRotation{
					SignerName: "lower-signer",
				},

				Client:        client.CoreV1(),
				Lister:        corev1listers.NewSecretLister(indexer),
				EventRecorder: events.NewInMemoryRecorder("test"),
			}

			newCA, err := test.caFn()
			if err != nil {
				t.Fatal(err)
			}
			err = c.ensureTargetCertKeyPair(newCA, newCA.Config.Certs)
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
