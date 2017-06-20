package servingcert

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/v1"
)

func TestRequiresRegenerationServiceUIDMismatch(t *testing.T) {
	tests := []struct {
		name          string
		primeServices func(cache.Store)
		secret        *v1.Secret
		expected      bool
	}{
		{
			name:          "no service annotation",
			primeServices: func(serviceCache cache.Store) {},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name:          "missing service",
			primeServices: func(serviceCache cache.Store) {},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation: "foo",
					},
				},
			},
			expected: false,
		},
		{
			name: "service-uid-mismatch",
			primeServices: func(serviceCache cache.Store) {
				serviceCache.Add(&v1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo", UID: types.UID("uid-2"), Annotations: map[string]string{ServingCertSecretAnnotation: "mysecret"}},
				})
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation: "foo",
						ServiceUIDAnnotation:  "uid-1",
					},
				},
			},
			expected: false,
		},
		{
			name: "service secret name mismatch",
			primeServices: func(serviceCache cache.Store) {
				serviceCache.Add(&v1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo", UID: types.UID("uid-1"), Annotations: map[string]string{ServingCertSecretAnnotation: "mysecret2"}},
				})
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation: "foo",
						ServiceUIDAnnotation:  "uid-1",
					},
				},
			},
			expected: false,
		},
		{
			name: "no expiry",
			primeServices: func(serviceCache cache.Store) {
				serviceCache.Add(&v1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo", UID: types.UID("uid-1"), Annotations: map[string]string{ServingCertSecretAnnotation: "mysecret"}},
				})
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation: "foo",
						ServiceUIDAnnotation:  "uid-1",
					},
				},
			},
			expected: true,
		},
		{
			name: "bad expiry",
			primeServices: func(serviceCache cache.Store) {
				serviceCache.Add(&v1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo", UID: types.UID("uid-1"), Annotations: map[string]string{ServingCertSecretAnnotation: "mysecret"}},
				})
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation:       "foo",
						ServiceUIDAnnotation:        "uid-1",
						ServingCertExpiryAnnotation: "bad-format",
					},
				},
			},
			expected: true,
		},
		{
			name: "expired expiry",
			primeServices: func(serviceCache cache.Store) {
				serviceCache.Add(&v1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo", UID: types.UID("uid-1"), Annotations: map[string]string{ServingCertSecretAnnotation: "mysecret"}},
				})
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation:       "foo",
						ServiceUIDAnnotation:        "uid-1",
						ServingCertExpiryAnnotation: time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
					},
				},
			},
			expected: true,
		},
		{
			name: "distant expiry",
			primeServices: func(serviceCache cache.Store) {
				serviceCache.Add(&v1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo", UID: types.UID("uid-1"), Annotations: map[string]string{ServingCertSecretAnnotation: "mysecret"}},
				})
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1", Name: "mysecret",
					Annotations: map[string]string{
						ServiceNameAnnotation:       "foo",
						ServiceUIDAnnotation:        "uid-1",
						ServingCertExpiryAnnotation: time.Now().Add(10 * time.Minute).Format(time.RFC3339),
					},
				},
			},
			expected: false,
		},
	}
	for _, tc := range tests {
		c := &ServiceServingCertUpdateController{
			serviceCache: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		}
		tc.primeServices(c.serviceCache)
		actual, service := c.requiresRegeneration(tc.secret)
		if tc.expected != actual {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expected, actual)
		}
		if service == nil && tc.expected {
			t.Errorf("%s: should have returned service", tc.name)
		}
	}
}
