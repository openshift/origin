package registryhostname

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServiceResolverCacheEmpty(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	})
	cache := newServiceResolverCache(fakeClient.Core().Services("default").Get)
	if v, ok := cache.resolve("FOO_SERVICE_HOST"); v != "" || !ok {
		t.Errorf("unexpected cache item")
	}
	if len(fakeClient.Actions()) != 1 {
		t.Errorf("unexpected client actions: %#v", fakeClient.Actions())
	}
	cache.resolve("FOO_SERVICE_HOST")
	if len(fakeClient.Actions()) != 1 {
		t.Errorf("unexpected cache miss: %#v", fakeClient.Actions())
	}
	cache.resolve("FOO_SERVICE_PORT")
	if len(fakeClient.Actions()) != 1 {
		t.Errorf("unexpected cache miss: %#v", fakeClient.Actions())
	}
}

type fakeRetriever struct {
	service *corev1.Service
	err     error
}

func (r fakeRetriever) Get(name string, options metav1.GetOptions) (*corev1.Service, error) {
	return r.service, r.err
}

func TestServiceResolverCache(t *testing.T) {
	c := fakeRetriever{
		err: errors.NewNotFound(corev1.Resource("Service"), "bar"),
	}
	cache := newServiceResolverCache(c.Get)
	if v, ok := cache.resolve("FOO_SERVICE_HOST"); v != "" || ok {
		t.Errorf("unexpected cache item")
	}

	c = fakeRetriever{
		service: &corev1.Service{
			Spec: corev1.ServiceSpec{
				ClusterIP: "127.0.0.1",
				Ports:     []corev1.ServicePort{{Port: 80}},
			},
		},
	}
	cache = newServiceResolverCache(c.Get)
	if v, ok := cache.resolve("FOO_SERVICE_HOST"); v != "127.0.0.1" || !ok {
		t.Errorf("unexpected cache item")
	}
	if v, ok := cache.resolve("FOO_SERVICE_PORT"); v != "80" || !ok {
		t.Errorf("unexpected cache item")
	}
	if _, err := cache.Defer("${UNKNOWN}"); err == nil {
		t.Errorf("unexpected non-error")
	}
	fn, err := cache.Defer("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := fn(); v != "test" || !ok {
		t.Errorf("unexpected cache item")
	}
	fn, err = cache.Defer("${FOO_SERVICE_HOST}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := fn(); v != "127.0.0.1" || !ok {
		t.Errorf("unexpected cache item")
	}
	if v, ok := fn(); v != "127.0.0.1" || !ok {
		t.Errorf("unexpected cache item")
	}
}
