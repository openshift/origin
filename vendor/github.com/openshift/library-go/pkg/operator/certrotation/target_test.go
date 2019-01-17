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

				if !actions[0].Matches("update", "secrets") {
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
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}

				if !actions[0].Matches("update", "secrets") {
					t.Error(actions[0])
				}

				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*corev1.Secret)
				if len(actual.Data["tls.crt"]) == 0 || len(actual.Data["tls.key"]) == 0 {
					t.Error(actual.Data)
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
				Namespace:         "ns",
				Validity:          24 * time.Hour,
				RefreshPercentage: .50,
				Name:              "target-secret",
				ServingRotation: &ServingRotation{
					Hostnames: []string{"foo"},
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
