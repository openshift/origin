package bootstrapauthenticator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"strings"
	"testing"
	"time"
)

func TestIsEnabled(t *testing.T) {
	deletionTime := metav1.NewTime(time.Now())
	namespaceSystem := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metav1.NamespaceSystem,
		},
	}
	testCases := []struct {
		name        string
		kubeClient  *fake.Clientset
		expectedErr string
		expectedVal bool
	}{
		{
			name:        "bootstrap user secret not present",
			kubeClient:  fake.NewSimpleClientset(),
			expectedVal: false,
		},
		{
			name: "bootstrap user secret present",
			kubeClient: fake.NewSimpleClientset(
				namespaceSystem,
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceSystem,
						Name:      bootstrapUserBasicAuth,
					},
					Data: map[string][]byte{bootstrapUserBasicAuth: []byte("foo")},
				}),
			expectedVal: true,
		},
		{
			name: "bootstrap user secret being deleted",
			kubeClient: fake.NewSimpleClientset(
				namespaceSystem,
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         metav1.NamespaceSystem,
						Name:              bootstrapUserBasicAuth,
						DeletionTimestamp: &deletionTime,
					},
					Data: map[string][]byte{bootstrapUserBasicAuth: []byte("foo")},
				}),
			expectedVal: false,
		},
		{
			name: "bootstrap user secret recreated",
			kubeClient: fake.NewSimpleClientset(
				namespaceSystem,
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         metav1.NamespaceSystem,
						Name:              bootstrapUserBasicAuth,
						CreationTimestamp: metav1.NewTime(time.Now().Add(2 * time.Hour)),
					},
					Data: map[string][]byte{bootstrapUserBasicAuth: []byte("foo")},
				}),
			expectedVal: false,
		},
		{
			name: "namespace not found error",
			kubeClient: fake.NewSimpleClientset(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceSystem,
						Name:      bootstrapUserBasicAuth,
					},
					Data: map[string][]byte{bootstrapUserBasicAuth: []byte("foo")},
				}),
			expectedErr: "not found",
			expectedVal: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			b := NewBootstrapUserDataGetter(test.kubeClient.CoreV1(), test.kubeClient.CoreV1())
			enabled, err := b.IsEnabled()
			if err != nil {
				if len(test.expectedErr) == 0 {
					t.Errorf("expected %#v, got %#v", test.expectedErr, err)
				} else if !strings.Contains(err.Error(), test.expectedErr) {
					t.Errorf("expected %#v, got %#v", test.expectedErr, err)
				}
			} else if len(test.expectedErr) > 0 {
				t.Errorf("expected %#v, got %#v", test.expectedErr, err)
			}
			if test.expectedVal != enabled {
				t.Errorf("%s: expected %v, got %v", test.name, test.expectedVal, enabled)
			}
		})
	}
}
