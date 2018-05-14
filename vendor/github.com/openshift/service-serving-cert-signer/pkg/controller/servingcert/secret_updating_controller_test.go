package servingcert

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestRequiresRegenerationServiceUIDMismatch(t *testing.T) {
	tests := []struct {
		name          string
		primeServices func(cache.Indexer)
		secret        *v1.Secret
		expected      bool
	}{
		{
			name:          "no service annotation",
			primeServices: func(serviceCache cache.Indexer) {},
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
			primeServices: func(serviceCache cache.Indexer) {},
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
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-2")}})},
				},
			},
			expected: false,
		},
		{
			name: "service secret name mismatch",
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-1")}})},
				},
			},
			expected: false,
		},
		{
			name: "no expiry",
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-1")}})},
				},
			},
			expected: true,
		},
		{
			name: "bad expiry",
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-1")}})},
				},
			},
			expected: true,
		},
		{
			name: "expired expiry",
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-1")}})},
				},
			},
			expected: true,
		},
		{
			name: "distant expiry",
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-1")}})},
				},
			},
			expected: false,
		},
		{
			name: "missing ownerref",
			primeServices: func(serviceCache cache.Indexer) {
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
					OwnerReferences: []metav1.OwnerReference{ownerRef(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("uid-2")}})},
				},
			},
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			index := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			c := &ServiceServingCertUpdateController{
				serviceLister: listers.NewServiceLister(index),
			}
			tc.primeServices(index)
			actual, service := c.requiresRegeneration(tc.secret)
			if tc.expected != actual {
				t.Errorf("%s: expected %v, got %v", tc.name, tc.expected, actual)
			}
			if service == nil && tc.expected {
				t.Errorf("%s: should have returned service", tc.name)
			}
		})
	}
}
