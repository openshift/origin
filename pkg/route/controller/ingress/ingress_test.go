package ingress

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	corelisters "k8s.io/client-go/listers/core/v1"
	extensionslisters "k8s.io/client-go/listers/extensions/v1beta1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/fake"
	routelisters "github.com/openshift/client-go/route/listers/route/v1"
)

type routeLister struct {
	Err   error
	Items []*routev1.Route
}

func (r *routeLister) List(selector labels.Selector) (ret []*routev1.Route, err error) {
	return r.Items, r.Err
}
func (r *routeLister) Routes(namespace string) routelisters.RouteNamespaceLister {
	return &nsRouteLister{r: r, ns: namespace}
}

type nsRouteLister struct {
	r  *routeLister
	ns string
}

func (r *nsRouteLister) List(selector labels.Selector) (ret []*routev1.Route, err error) {
	return r.r.Items, r.r.Err
}
func (r *nsRouteLister) Get(name string) (*routev1.Route, error) {
	for _, s := range r.r.Items {
		if s.Name == name && r.ns == s.Namespace {
			return s, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, name)
}

type ingressLister struct {
	Err   error
	Items []*extensionsv1beta1.Ingress
}

func (r *ingressLister) List(selector labels.Selector) (ret []*extensionsv1beta1.Ingress, err error) {
	return r.Items, r.Err
}
func (r *ingressLister) Ingresses(namespace string) extensionslisters.IngressNamespaceLister {
	return &nsIngressLister{r: r, ns: namespace}
}

type nsIngressLister struct {
	r  *ingressLister
	ns string
}

func (r *nsIngressLister) List(selector labels.Selector) (ret []*extensionsv1beta1.Ingress, err error) {
	return r.r.Items, r.r.Err
}
func (r *nsIngressLister) Get(name string) (*extensionsv1beta1.Ingress, error) {
	for _, s := range r.r.Items {
		if s.Name == name && r.ns == s.Namespace {
			return s, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, name)
}

type serviceLister struct {
	Err   error
	Items []*v1.Service
}

func (r *serviceLister) List(selector labels.Selector) (ret []*v1.Service, err error) {
	return r.Items, r.Err
}
func (r *serviceLister) Services(namespace string) corelisters.ServiceNamespaceLister {
	return &nsServiceLister{r: r, ns: namespace}
}

func (r *serviceLister) GetPodServices(pod *v1.Pod) ([]*v1.Service, error) {
	panic("unsupported")
}

type nsServiceLister struct {
	r  *serviceLister
	ns string
}

func (r *nsServiceLister) List(selector labels.Selector) (ret []*v1.Service, err error) {
	return r.r.Items, r.r.Err
}
func (r *nsServiceLister) Get(name string) (*v1.Service, error) {
	for _, s := range r.r.Items {
		if s.Name == name && r.ns == s.Namespace {
			return s, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, name)
}

type secretLister struct {
	Err   error
	Items []*v1.Secret
}

func (r *secretLister) List(selector labels.Selector) (ret []*v1.Secret, err error) {
	return r.Items, r.Err
}
func (r *secretLister) Secrets(namespace string) corelisters.SecretNamespaceLister {
	return &nsSecretLister{r: r, ns: namespace}
}

type nsSecretLister struct {
	r  *secretLister
	ns string
}

func (r *nsSecretLister) List(selector labels.Selector) (ret []*v1.Secret, err error) {
	return r.r.Items, r.r.Err
}
func (r *nsSecretLister) Get(name string) (*v1.Secret, error) {
	for _, s := range r.r.Items {
		if s.Name == name && r.ns == s.Namespace {
			return s, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, name)
}

const complexIngress = `
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: test-1
  namespace: test
spec:
  rules:
  - host: 1.ingress-test.com
    http:
      paths:
      - path: /test
        backend:
          serviceName: ingress-endpoint-1
          servicePort: 80
      - path: /other
        backend:
          serviceName: ingress-endpoint-2
          servicePort: 80
  - host: 2.ingress-test.com
    http:
      paths:
      - path: /
        backend:
          serviceName: ingress-endpoint-1
          servicePort: 80
  - host: 3.ingress-test.com
    http:
      paths:
      - path: /
        backend:
          serviceName: ingress-endpoint-1
          servicePort: 80
`

func TestController_stabilizeAfterCreate(t *testing.T) {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(complexIngress), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ingress := obj.(*extensionsv1beta1.Ingress)

	i := &ingressLister{
		Items: []*extensionsv1beta1.Ingress{
			ingress,
		},
	}
	r := &routeLister{}
	s := &secretLister{}
	svc := &serviceLister{Items: []*v1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-endpoint-1",
				Namespace: "test",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-endpoint-2",
				Namespace: "test",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "80-tcp",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
	}}

	var names []string
	kc := &fake.Clientset{}
	kc.AddReactor("*", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case clientgotesting.CreateAction:
			obj := a.GetObject().DeepCopyObject()
			m := obj.(metav1.Object)
			if len(m.GetName()) == 0 {
				m.SetName(m.GetGenerateName())
			}
			names = append(names, m.GetName())
			return true, obj, nil
		}
		return true, nil, nil
	})

	c := &Controller{
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingress-to-route-test"),
		client:        kc.Route(),
		ingressLister: i,
		routeLister:   r,
		secretLister:  s,
		serviceLister: svc,
		expectations:  newExpectations(),
	}
	defer c.queue.ShutDown()

	// load the ingresses for the namespace
	if err := c.sync(queueKey{namespace: "test"}); err != nil {
		t.Errorf("Controller.sync() error = %v", err)
	}
	if c.queue.Len() != 1 {
		t.Fatalf("Controller.sync() unexpected queue: %#v", c.queue.Len())
	}
	actions := kc.Actions()
	if len(actions) != 0 {
		t.Fatalf("Controller.sync() unexpected actions: %#v", actions)
	}

	// process the ingress
	key, _ := c.queue.Get()
	expectKey := queueKey{namespace: ingress.Namespace, name: ingress.Name}
	if key.(queueKey) != expectKey {
		t.Fatalf("incorrect key: %v", key)
	}
	if err := c.sync(key.(queueKey)); err != nil {
		t.Fatalf("Controller.sync() error = %v", err)
	}
	c.queue.Done(key)
	if c.queue.Len() != 0 {
		t.Fatalf("Controller.sync() unexpected queue: %#v", c.queue.Len())
	}
	actions = kc.Actions()
	if len(actions) == 0 {
		t.Fatalf("Controller.sync() unexpected actions: %#v", actions)
	}
	if !c.expectations.Expecting("test", "test-1") {
		t.Fatalf("Controller.sync() should be holding an expectation: %#v", c.expectations.expect)
	}

	for _, action := range actions {
		switch action.GetVerb() {
		case "create":
			switch o := action.(clientgotesting.CreateAction).GetObject().(type) {
			case *routev1.Route:
				r.Items = append(r.Items, o)
				c.processRoute(o)
			default:
				t.Fatalf("Unexpected create: %T", o)
			}
		default:
			t.Fatalf("Unexpected action: %#v", action)
		}
	}
	if c.queue.Len() != 1 {
		t.Fatalf("Controller.sync() unexpected queue: %#v", c.queue.Len())
	}
	if c.expectations.Expecting("test", "test-1") {
		t.Fatalf("Controller.sync() should have cleared all expectations: %#v", c.expectations.expect)
	}
	c.expectations.Expect("test", "test-1", names[0])

	// waiting for a single expected route, will do nothing
	key, _ = c.queue.Get()
	if err := c.sync(key.(queueKey)); err != nil {
		t.Errorf("Controller.sync() error = %v", err)
	}
	c.queue.Done(key)
	actions = kc.Actions()
	if len(actions) == 0 {
		t.Fatalf("Controller.sync() unexpected actions: %#v", actions)
	}
	if c.queue.Len() != 1 {
		t.Fatalf("Controller.sync() unexpected queue: %#v", c.queue.Len())
	}
	c.expectations.Satisfied("test", "test-1", names[0])

	// steady state, nothing has changed
	key, _ = c.queue.Get()
	if err := c.sync(key.(queueKey)); err != nil {
		t.Errorf("Controller.sync() error = %v", err)
	}
	c.queue.Done(key)
	actions = kc.Actions()
	if len(actions) == 0 {
		t.Fatalf("Controller.sync() unexpected actions: %#v", actions)
	}
	if c.queue.Len() != 0 {
		t.Fatalf("Controller.sync() unexpected queue: %#v", c.queue.Len())
	}
}

func newTestExpectations(fn func(*expectations)) *expectations {
	e := newExpectations()
	fn(e)
	return e
}

func TestController_sync(t *testing.T) {
	services := &serviceLister{Items: []*v1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-1",
				Namespace: "test",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-2",
				Namespace: "test",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "80-tcp",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
	}}
	secrets := &secretLister{Items: []*v1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-0",
				Namespace: "test",
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				v1.TLSCertKey:       []byte(`cert`),
				v1.TLSPrivateKeyKey: []byte(`key`),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-1",
				Namespace: "test",
			},
			Type: v1.SecretTypeTLS,
			Data: map[string][]byte{
				v1.TLSCertKey:       []byte(`cert`),
				v1.TLSPrivateKeyKey: []byte(`key`),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-1a",
				Namespace: "test",
			},
			Type: v1.SecretTypeTLS,
			Data: map[string][]byte{
				v1.TLSCertKey:       []byte(`cert`),
				v1.TLSPrivateKeyKey: []byte(`key2`),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-2",
				Namespace: "test",
			},
			Type: v1.SecretTypeTLS,
			Data: map[string][]byte{
				v1.TLSPrivateKeyKey: []byte(`key`),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-3",
				Namespace: "test",
			},
			Type: v1.SecretTypeTLS,
			Data: map[string][]byte{
				v1.TLSCertKey:       []byte(``),
				v1.TLSPrivateKeyKey: []byte(``),
			},
		},
	}}
	boolTrue := true
	type fields struct {
		i   extensionslisters.IngressLister
		r   routelisters.RouteLister
		s   corelisters.SecretLister
		svc corelisters.ServiceLister
	}
	tests := []struct {
		name            string
		fields          fields
		args            queueKey
		expects         *expectations
		wantErr         bool
		wantCreates     []*routev1.Route
		wantPatches     []clientgotesting.PatchActionImpl
		wantDeletes     []clientgotesting.DeleteActionImpl
		wantQueue       []queueKey
		wantExpectation *expectations
		wantExpects     []queueKey
	}{
		{
			name:   "no changes",
			fields: fields{i: &ingressLister{}, r: &routeLister{}},
			args:   queueKey{namespace: "test", name: "1"},
		},
		{
			name:   "sync namespace - no ingress",
			fields: fields{i: &ingressLister{}, r: &routeLister{}},
			args:   queueKey{namespace: "test"},
		},
		{
			name: "sync namespace - two ingress",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "2",
							Namespace: "test",
						},
					},
				}},
				r: &routeLister{},
			},
			args: queueKey{namespace: "test"},
			wantQueue: []queueKey{
				{namespace: "test", name: "1"},
				{namespace: "test", name: "2"},
			},
		},
		{
			name: "ignores incomplete ingress - no host",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/deep", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "ignores incomplete ingress - no service",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/deep", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "ignores incomplete ingress - no paths",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "create route",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/deep", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{},
			},
			args:        queueKey{namespace: "test", name: "1"},
			wantExpects: []queueKey{{namespace: "test", name: "1"}},
			wantCreates: []*routev1.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "<generated>",
						Namespace:       "test",
						OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
					},
					Spec: routev1.RouteSpec{
						Host: "test.com",
						Path: "/deep",
						To: routev1.RouteTargetReference{
							Name: "service-1",
						},
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "<generated>",
						Namespace:       "test",
						OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
					},
					Spec: routev1.RouteSpec{
						Host: "test.com",
						Path: "/",
						To: routev1.RouteTargetReference{
							Name: "service-1",
						},
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			},
		},
		{
			name: "create route - blocked by expectation",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/deep", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{},
			},
			expects: newTestExpectations(func(e *expectations) {
				e.Expect("test", "1", "route-test-1")
			}),
			args:      queueKey{namespace: "test", name: "1"},
			wantQueue: []queueKey{{namespace: "test", name: "1"}},
			// preserves the expectations unchanged
			wantExpectation: newTestExpectations(func(e *expectations) {
				e.Expect("test", "1", "route-test-1")
			}),
		},
		{
			name: "update route",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(80),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantPatches: []clientgotesting.PatchActionImpl{
				{
					Name:  "1-abcdef",
					Patch: []byte(`[{"op":"replace","path":"/spec","value":{"host":"test.com","path":"/","to":{"kind":"","name":"service-1","weight":null},"port":{"targetPort":8080}}}]`),
				},
			},
		},
		{
			name: "no-op",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "no-op - ignore partially owned resource",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					// this route is identical to the ingress
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
						},
					},
					// this route should be left as is because controller is not true
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-empty",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1"}},
						},
						Spec: routev1.RouteSpec{},
					},
					// this route should be ignored because it doesn't match the ingress name
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "2-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "2", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "update ingress with missing secret ref",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-4"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					Name: "1-abcdef",
				},
			},
		},
		{
			name: "update ingress to not reference secret",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com1"}, SecretName: "secret-1"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination:                   routev1.TLSTerminationEdge,
								InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
								Key:         "key",
								Certificate: "cert",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantPatches: []clientgotesting.PatchActionImpl{
				{
					Name:  "1-abcdef",
					Patch: []byte(`[{"op":"replace","path":"/spec","value":{"host":"test.com","path":"/","to":{"kind":"","name":"service-1","weight":null},"port":{"targetPort":8080}}}]`),
				},
			},
		},
		{
			name: "update route - tls config missing",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-1"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantPatches: []clientgotesting.PatchActionImpl{
				{
					Name:  "1-abcdef",
					Patch: []byte(`[{"op":"replace","path":"/spec","value":{"host":"test.com","path":"/","to":{"kind":"","name":"service-1","weight":null},"port":{"targetPort":8080},"tls":{"termination":"edge","certificate":"cert","key":"key","insecureEdgeTerminationPolicy":"Redirect"}}}]`),
				},
			},
		},
		{
			name: "update route - secret values changed",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-1a"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination: routev1.TLSTerminationEdge,
								Key:         "key",
								Certificate: "cert",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantPatches: []clientgotesting.PatchActionImpl{
				{
					Name:  "1-abcdef",
					Patch: []byte(`[{"op":"replace","path":"/spec","value":{"host":"test.com","path":"/","to":{"kind":"","name":"service-1","weight":null},"port":{"targetPort":8080},"tls":{"termination":"edge","certificate":"cert","key":"key2"}}}]`),
				},
			},
		},
		{
			name: "no-op - has TLS",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-1"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination:                   routev1.TLSTerminationEdge,
								InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
								Key:         "key",
								Certificate: "cert",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "no-op - has secret with empty keys",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-3"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination:                   routev1.TLSTerminationEdge,
								InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
								Key:         "",
								Certificate: "",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "no-op - termination policy has been changed by the user",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-1"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination: routev1.TLSTerminationEdge,
								Key:         "key",
								Certificate: "cert",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
		{
			name: "delete route when referenced secret is not TLS",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-0"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination:                   routev1.TLSTerminationEdge,
								InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
								Key:         "key",
								Certificate: "cert",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					Name: "1-abcdef",
				},
			},
		},
		{
			name: "delete route when referenced secret is not valid",
			fields: fields{
				i: &ingressLister{Items: []*extensionsv1beta1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "1",
							Namespace: "test",
						},
						Spec: extensionsv1beta1.IngressSpec{
							TLS: []extensionsv1beta1.IngressTLS{
								{Hosts: []string{"test.com"}, SecretName: "secret-2"},
							},
							Rules: []extensionsv1beta1.IngressRule{
								{
									Host: "test.com",
									IngressRuleValue: extensionsv1beta1.IngressRuleValue{
										HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
											Paths: []extensionsv1beta1.HTTPIngressPath{
												{
													Path: "/", Backend: extensionsv1beta1.IngressBackend{
														ServiceName: "service-1",
														ServicePort: intstr.FromString("http"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{
							Host: "test.com",
							Path: "/",
							To: routev1.RouteTargetReference{
								Name: "service-1",
							},
							Port: &routev1.RoutePort{
								TargetPort: intstr.FromInt(8080),
							},
							WildcardPolicy: routev1.WildcardPolicyNone,
							TLS: &routev1.TLSConfig{
								Termination:                   routev1.TLSTerminationEdge,
								InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
								Key:         "key",
								Certificate: "",
							},
						},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					Name: "1-abcdef",
				},
			},
		},
		{
			name: "ignore route when parent ingress no longer exists (gc will handle)",
			fields: fields{
				i: &ingressLister{},
				r: &routeLister{Items: []*routev1.Route{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "1-abcdef",
							Namespace:       "test",
							OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "Ingress", Name: "1", Controller: &boolTrue}},
						},
						Spec: routev1.RouteSpec{},
					},
				}},
			},
			args: queueKey{namespace: "test", name: "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var names []string
			kc := &fake.Clientset{}
			kc.AddReactor("*", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				switch a := action.(type) {
				case clientgotesting.CreateAction:
					obj := a.GetObject().DeepCopyObject()
					m := obj.(metav1.Object)
					if len(m.GetName()) == 0 {
						m.SetName(m.GetGenerateName())
					}
					names = append(names, m.GetName())
					return true, obj, nil
				}
				return true, nil, nil
			})

			c := &Controller{
				queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingress-to-route-test"),
				client:        kc.Route(),
				ingressLister: tt.fields.i,
				routeLister:   tt.fields.r,
				secretLister:  tt.fields.s,
				serviceLister: tt.fields.svc,
				expectations:  tt.expects,
			}
			// default these
			if c.expectations == nil {
				c.expectations = newExpectations()
			}
			if c.secretLister == nil {
				c.secretLister = secrets
			}
			if c.serviceLister == nil {
				c.serviceLister = services
			}

			if err := c.sync(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("Controller.sync() error = %v, wantErr %v", err, tt.wantErr)
			}

			c.queue.ShutDown()
			var hasQueue []queueKey
			for {
				key, shutdown := c.queue.Get()
				if shutdown {
					break
				}
				hasQueue = append(hasQueue, key.(queueKey))
			}
			if !reflect.DeepEqual(tt.wantQueue, hasQueue) {
				t.Errorf("unexpected queue: %s", diff.ObjectReflectDiff(tt.wantQueue, hasQueue))
			}

			wants := tt.wantExpectation
			if wants == nil {
				wants = newTestExpectations(func(e *expectations) {
					for _, key := range tt.wantExpects {
						for _, routeName := range names {
							e.Expect(key.namespace, key.name, routeName)
						}
					}
				})
			}
			if !reflect.DeepEqual(wants, c.expectations) {
				t.Errorf("unexpected expectations: %s", diff.ObjectReflectDiff(wants.expect, c.expectations.expect))
			}

			actions := kc.Actions()

			for i := range tt.wantCreates {
				if i > len(actions)-1 {
					t.Fatalf("Controller.sync() unexpected actions: %#v", kc.Actions())
				}
				if actions[i].GetVerb() != "create" {
					t.Fatalf("Controller.sync() unexpected actions: %#v", kc.Actions())
				}
				action := actions[i].(clientgotesting.CreateAction)
				if action.GetNamespace() != tt.args.namespace {
					t.Errorf("unexpected action[%d]: %#v", i, action)
				}
				obj := action.GetObject()
				if tt.wantCreates[i].Name == "<generated>" {
					tt.wantCreates[i].Name = names[0]
					names = names[1:]
				}
				if !reflect.DeepEqual(tt.wantCreates[i], obj) {
					t.Errorf("unexpected create: %s", diff.ObjectReflectDiff(tt.wantCreates[i], obj))
				}
			}
			actions = actions[len(tt.wantCreates):]

			for i := range tt.wantPatches {
				if i > len(actions)-1 {
					t.Fatalf("Controller.sync() unexpected actions: %#v", kc.Actions())
				}
				if actions[i].GetVerb() != "patch" {
					t.Fatalf("Controller.sync() unexpected actions: %#v", kc.Actions())
				}
				action := actions[i].(clientgotesting.PatchAction)
				if action.GetNamespace() != tt.args.namespace || action.GetName() != tt.wantPatches[i].Name {
					t.Errorf("unexpected action[%d]: %#v", i, action)
				}
				if !reflect.DeepEqual(string(action.GetPatch()), string(tt.wantPatches[i].Patch)) {
					t.Errorf("unexpected action[%d]: %s", i, string(action.GetPatch()))
				}
			}
			actions = actions[len(tt.wantPatches):]

			for i := range tt.wantDeletes {
				if i > len(actions)-1 {
					t.Fatalf("Controller.sync() unexpected actions: %#v", kc.Actions())
				}
				if actions[i].GetVerb() != "delete" {
					t.Fatalf("Controller.sync() unexpected actions: %#v", kc.Actions())
				}
				action := actions[i].(clientgotesting.DeleteAction)
				if action.GetName() != tt.wantDeletes[i].Name || action.GetNamespace() != tt.args.namespace {
					t.Errorf("unexpected action[%d]: %#v", i, action)
				}
			}
			actions = actions[len(tt.wantDeletes):]

			if len(actions) != 0 {
				t.Fatalf("Controller.sync() unexpected actions: %#v", actions)
			}
		})
	}
}
