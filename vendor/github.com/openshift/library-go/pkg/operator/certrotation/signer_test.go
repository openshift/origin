package certrotation

import (
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestEnsureSigningCertKeyPair(t *testing.T) {
	tests := []struct {
		name string

		initialSecret *corev1.Secret

		verifyActions func(t *testing.T, client *kubefake.Clientset)
		expectedError string
	}{
		{
			name: "initial create",
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				t.Helper()
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
			name: "update no annotations",
			initialSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "signer"},
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				t.Helper()
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
			},
		},
		{
			name: "update no work",
			initialSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "signer",
					Annotations: map[string]string{
						"auth.openshift.io/certificate-not-after":  "2108-09-08T22:47:31-07:00",
						"auth.openshift.io/certificate-not-before": "2108-09-08T20:47:31-07:00",
					}},
			},
			verifyActions: func(t *testing.T, client *kubefake.Clientset) {
				t.Helper()
				actions := client.Actions()
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
			expectedError: "certFile missing", // this means we tried to read the cert from the existing secret.  If we created one, we fail in the client check
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

			client := kubefake.NewSimpleClientset()
			if test.initialSecret != nil {
				indexer.Add(test.initialSecret)
				client = kubefake.NewSimpleClientset(test.initialSecret)
			}

			c := &SigningRotation{
				Namespace:     "ns",
				Name:          "signer",
				Validity:      24 * time.Hour,
				Refresh:       12 * time.Hour,
				Client:        client.CoreV1(),
				Lister:        corev1listers.NewSecretLister(indexer),
				EventRecorder: events.NewInMemoryRecorder("test"),
			}

			_, err := c.ensureSigningCertKeyPair()
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
