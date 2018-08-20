package controller

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/api/route"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/fake"
	routelisters "github.com/openshift/client-go/route/listers/route/v1"
	"github.com/openshift/origin/pkg/util/writerlease"
)

type noopLease struct{}

func (_ noopLease) Wait() bool {
	panic("not implemented")
}

func (_ noopLease) WaitUntil(t time.Duration) (leader bool, ok bool) {
	panic("not implemented")
}

func (_ noopLease) Try(key string, fn writerlease.WorkFunc) {
	fn()
}

func (_ noopLease) Extend(key string) {
}

func (_ noopLease) Remove(key string) {
	panic("not implemented")
}

type fakePlugin struct {
	t     watch.EventType
	route *routev1.Route
	err   error
}

func (p *fakePlugin) HandleRoute(t watch.EventType, route *routev1.Route) error {
	p.t, p.route = t, route
	return p.err
}

func (p *fakePlugin) HandleNode(t watch.EventType, node *kapi.Node) error {
	return fmt.Errorf("not expected")
}

func (p *fakePlugin) HandleEndpoints(watch.EventType, *kapi.Endpoints) error {
	return fmt.Errorf("not expected")
}
func (p *fakePlugin) HandleNamespaces(namespaces sets.String) error {
	return fmt.Errorf("not expected")
}
func (p *fakePlugin) Commit() error {
	return fmt.Errorf("not expected")
}

type routeLister struct {
	items []*routev1.Route
	err   error
}

func (l *routeLister) List(selector labels.Selector) (ret []*routev1.Route, err error) {
	return l.items, l.err
}

func (l *routeLister) Routes(namespace string) routelisters.RouteNamespaceLister {
	return routeNamespaceLister{namespace: namespace, l: l}
}

type routeNamespaceLister struct {
	l         *routeLister
	namespace string
}

func (l routeNamespaceLister) List(selector labels.Selector) (ret []*routev1.Route, err error) {
	var items []*routev1.Route
	for _, item := range l.l.items {
		if item.Namespace == l.namespace {
			items = append(items, item)
		}
	}
	return items, l.l.err
}

func (l routeNamespaceLister) Get(name string) (*routev1.Route, error) {
	for _, item := range l.l.items {
		if item.Namespace == l.namespace && item.Name == name {
			return item, nil
		}
	}
	return nil, errors.NewNotFound(route.Resource("route"), name)
}

type recorded struct {
	at      time.Time
	ingress *routev1.RouteIngress
}

type fakeTracker struct {
	contended map[string]recorded
	cleared   map[string]recorded
	results   map[string]bool
}

func (t *fakeTracker) IsChangeContended(id string, now time.Time, ingress *routev1.RouteIngress) bool {
	if t.contended == nil {
		t.contended = make(map[string]recorded)
	}
	t.contended[id] = recorded{
		at:      now,
		ingress: ingress,
	}
	return t.results[id]
}

func (t *fakeTracker) Clear(id string, ingress *routev1.RouteIngress) {
	if t.cleared == nil {
		t.cleared = make(map[string]recorded)
	}
	t.cleared[id] = recorded{
		ingress: ingress,
		at:      ingressConditionTouched(ingress).Time,
	}
}

func TestStatusNoOp(t *testing.T) {
	now := nowFn()
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset()
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:                    "route1.test.local",
					RouterName:              "test",
					RouterCanonicalHostname: "a.b.c.d",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "a.b.c.d", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, route)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Actions()) > 0 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
}

func checkResult(t *testing.T, err error, c *fake.Clientset, admitter *StatusAdmitter, targetHost string, targetObjTime metav1.Time, targetCachedTime *time.Time, ingressInd int, actionInd int) *routev1.Route {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Actions()) != actionInd+1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[actionInd]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[actionInd].(clientgotesting.UpdateAction).GetObject().(*routev1.Route)
	if len(obj.Status.Ingress) != ingressInd+1 || obj.Status.Ingress[ingressInd].Host != targetHost {
		t.Fatalf("expected route reset: expected %q / actual %q -- %#v", targetHost, obj.Status.Ingress[ingressInd].Host, obj)
	}
	condition := obj.Status.Ingress[ingressInd].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != targetObjTime || condition.Status != corev1.ConditionTrue || condition.Reason != "" {
		t.Fatalf("%s: unexpected condition: %#v %s/%s", targetHost, condition, condition.LastTransitionTime, targetObjTime)
	}
	if targetCachedTime != nil {
		switch tracker := admitter.tracker.(type) {
		case *SimpleContentionTracker:
			if tracker.ids["uid1"].at != *targetCachedTime {
				t.Fatalf("unexpected status time")
			}
		}
	}

	return obj
}

func TestStatusResetsHost(t *testing.T) {
	now := metav1.Now()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, route)

	route = checkResult(t, err, c, admitter, "route1.test.local", now, &now.Time, 0, 0)
	ingress := findIngressForRoute(route, "test")
	if ingress == nil {
		t.Fatalf("no ingress found: %#v", route)
	}
	if ingress.Host != "route1.test.local" {
		t.Fatalf("incorrect ingress: %#v", ingress)
	}
}

func findIngressForRoute(route *routev1.Route, routerName string) *routev1.RouteIngress {
	for i := range route.Status.Ingress {
		if route.Status.Ingress[i].RouterName == routerName {
			return &route.Status.Ingress[i]
		}
	}
	return nil
}

func TestStatusAdmitsRouteOnForbidden(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		return true, nil, errors.NewForbidden(corev1.Resource("Route"), "route1", nil)
	})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, route)
	route = checkResult(t, err, c, admitter, "route1.test.local", now, &touched.Time, 0, 0)
	ingress := findIngressForRoute(route, "test")
	if ingress == nil {
		t.Fatalf("no ingress found: %#v", route)
	}
	if ingress.Host != "route1.test.local" {
		t.Fatalf("incorrect ingress: %#v", ingress)
	}
}

func TestStatusBackoffOnConflict(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		return true, nil, errors.NewConflict(corev1.Resource("Route"), "route1", nil)
	})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, route)
	checkResult(t, err, c, admitter, "route1.test.local", now, nil, 0, 0)
}

func TestStatusRecordRejection(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(route, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routev1.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != corev1.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
}

func TestStatusRecordRejectionNoChange(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route1.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							Reason:             "Failed",
							Message:            "generic error",
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(route, "Failed", "generic error")

	if len(c.Actions()) != 0 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
}

func TestStatusRecordRejectionWithStatus(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(route, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routev1.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != corev1.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
}

func TestStatusRecordRejectionOnHostUpdateOnly(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched,
							Reason:             "Failed",
							Message:            "generic error",
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(route, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routev1.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != corev1.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
	if tracker.contended["uid1"].at != now.Time || tracker.cleared["uid1"].at.IsZero() {
		t.Fatal(tracker)
	}
}

func TestStatusRecordRejectionConflict(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		return true, nil, errors.NewConflict(corev1.Resource("Route"), "route1", nil)
	})
	tracker := &fakeTracker{}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}
	lister := &routeLister{items: []*routev1.Route{route}}
	admitter := NewStatusAdmitter(p, c.Route(), lister, "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(route, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routev1.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != corev1.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
}

func TestStatusFightBetweenReplicas(t *testing.T) {
	p := &fakePlugin{}
	stopCh := make(chan struct{})
	defer close(stopCh)

	// the initial pre-population
	now1 := metav1.Now()
	nowFn = func() metav1.Time { return now1 }
	c1 := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker1 := &fakeTracker{}
	route1 := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.test.local"},
		Status:     routev1.RouteStatus{},
	}
	lister1 := &routeLister{items: []*routev1.Route{route1}}
	admitter1 := NewStatusAdmitter(p, c1.Route(), lister1, "test", "", noopLease{}, tracker1)
	err := admitter1.HandleRoute(watch.Added, route1)

	outObj1 := checkResult(t, err, c1, admitter1, "route1.test.local", now1, &now1.Time, 0, 0)
	if tracker1.cleared["uid1"].at != now1.Time {
		t.Fatal(tracker1)
	}
	outObj1 = outObj1.DeepCopy()

	// the new deployment's replica
	now2 := metav1.Time{Time: now1.Time.Add(2 * time.Minute)}
	nowFn = func() metav1.Time { return now2 }
	c2 := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker2 := &fakeTracker{}
	lister2 := &routeLister{items: []*routev1.Route{outObj1}}
	admitter2 := NewStatusAdmitter(p, c2.Route(), lister2, "test", "", noopLease{}, tracker2)
	outObj1.Spec.Host = "route1.test-new.local"
	err = admitter2.HandleRoute(watch.Added, outObj1)

	outObj2 := checkResult(t, err, c2, admitter2, "route1.test-new.local", now2, &now2.Time, 0, 0)
	if tracker2.cleared["uid1"].at != now2.Time {
		t.Fatal(tracker2)
	}
	outObj2 = outObj2.DeepCopy()

	lister1.items[0] = outObj2

	tracker1.results = map[string]bool{"uid1": true}
	now3 := metav1.Time{Time: now1.Time.Add(time.Minute)}
	nowFn = func() metav1.Time { return now3 }
	outObj2.Spec.Host = "route1.test.local"
	err = admitter1.HandleRoute(watch.Modified, outObj2)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// expect the last HandleRoute not to have performed any actions
	if len(c1.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c1.Actions())
	}
}

func TestStatusFightBetweenRouters(t *testing.T) {
	p := &fakePlugin{}

	// initial try, results in conflict
	now1 := metav1.Now()
	nowFn = func() metav1.Time { return now1 }
	touched1 := metav1.Time{Time: now1.Add(-time.Minute)}
	c1 := fake.NewSimpleClientset(&routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	returnConflict := true
	c1.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if returnConflict {
			returnConflict = false
			return true, nil, errors.NewConflict(corev1.Resource("Route"), "route1", nil)
		}
		return false, nil, nil
	})
	tracker := &fakeTracker{}
	route1 := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route2.test-new.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route1.test.local",
					RouterName: "test1",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched1,
						},
					},
				},
				{
					Host:       "route1.test-new.local",
					RouterName: "test2",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched1,
						},
					},
				},
			},
		},
	}
	lister1 := &routeLister{items: []*routev1.Route{route1}}
	admitter1 := NewStatusAdmitter(p, c1.Route(), lister1, "test2", "", noopLease{}, tracker)
	err := admitter1.HandleRoute(watch.Added, route1)

	checkResult(t, err, c1, admitter1, "route2.test-new.local", now1, nil, 1, 0)
	if tracker.contended["uid1"].at != now1.Time || !tracker.cleared["uid1"].at.IsZero() {
		t.Fatalf("should have recorded uid1 into tracker: %#v", tracker)
	}

	// second try, should not send status because the tracker reports a conflict
	now2 := metav1.Now()
	nowFn = func() metav1.Time { return now2 }
	touched2 := metav1.Time{Time: now2.Add(-time.Minute)}
	tracker.cleared = nil
	tracker.results = map[string]bool{"uid1": true}
	route2 := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route2.test-new.local"},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test1",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched1,
						},
					},
				},
				{
					Host:       "route1.test-new.local",
					RouterName: "test2",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: &touched2,
						},
					},
				},
			},
		},
	}
	lister1.items[0] = route2
	err = admitter1.HandleRoute(watch.Modified, route2)

	checkResult(t, err, c1, admitter1, "route2.test-new.local", now1, &now2.Time, 1, 0)
	if tracker.contended["uid1"].at != now2.Time {
		t.Fatalf("should have recorded uid1 into tracker: %#v", tracker)
	}
}

func makePass(t *testing.T, host string, admitter *StatusAdmitter, srcObj *routev1.Route, expectUpdate bool, conflict bool) *routev1.Route {
	t.Helper()
	// initialize a new client
	c := fake.NewSimpleClientset(srcObj)
	if conflict {
		c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() != "status" {
				return false, nil, nil
			}
			return true, nil, errors.NewConflict(corev1.Resource("Route"), "route1", nil)
		})
	}

	admitter.client = c.Route()

	inputObj := srcObj.DeepCopy()
	inputObj.Spec.Host = host

	admitter.lister.(*routeLister).items = []*routev1.Route{inputObj}

	err := admitter.HandleRoute(watch.Modified, inputObj)

	if expectUpdate {
		now := nowFn()
		return checkResult(t, err, c, admitter, inputObj.Spec.Host, now, nil, 0, 0)
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// expect the last HandleRoute not to have performed any actions
	if len(c.Actions()) != 0 {
		t.Fatalf("expected no actions: %#v", c)
	}

	return nil
}

func TestRouterContention(t *testing.T) {
	p := &fakePlugin{}
	stopCh := make(chan struct{})
	defer close(stopCh)

	now := metav1.Now()
	nowFn = func() metav1.Time { return now }

	initObj := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routev1.RouteSpec{Host: "route1.new.local"},
		Status:     routev1.RouteStatus{},
	}

	// NB: contention period is 1 minute
	i1 := &fakeInformer{}
	t1 := NewSimpleContentionTracker(i1, "test", time.Minute)
	lister1 := &routeLister{}

	r1 := NewStatusAdmitter(p, nil, lister1, "test", "", noopLease{}, t1)

	// update
	currObj := makePass(t, "route1.test.local", r1, initObj, true, false)
	// no-op
	makePass(t, "route1.test.local", r1, currObj, false, false)

	// another caller changes the status, we should change it back
	findIngressForRoute(currObj, "test").Host = "route1.other.local"
	currObj = makePass(t, "route1.test.local", r1, currObj, true, false)

	// if we observe a single change to our ingress, record it but still update
	otherObj := currObj.DeepCopy()
	ingress := findIngressForRoute(otherObj, "test")
	ingress.Host = "route1.other1.local"
	t1.Changed(string(otherObj.UID), ingress)
	if t1.IsChangeContended(string(otherObj.UID), nowFn().Time, ingress) {
		t.Fatal("change shouldn't be contended yet")
	}
	currObj = makePass(t, "route1.test.local", r1, otherObj, true, false)

	// updating the route sets us back to candidate, but if we observe our own write
	// we stay in candidate
	ingress = findIngressForRoute(currObj, "test").DeepCopy()
	t1.Changed(string(currObj.UID), ingress)
	if t1.IsChangeContended(string(currObj.UID), nowFn().Time, ingress) {
		t.Fatal("change should not be contended")
	}
	makePass(t, "route1.test.local", r1, currObj, false, false)

	// updating the route sets us back to candidate, and if we detect another change to
	// ingress we will go into conflict, even with our original write
	ingress = ingressChangeWithNewHost(currObj, "test", "route1.other2.local")
	t1.Changed(string(currObj.UID), ingress)
	if !t1.IsChangeContended(string(currObj.UID), nowFn().Time, ingress) {
		t.Fatal("change should be contended")
	}
	makePass(t, "route1.test.local", r1, currObj, false, false)

	// another contending write occurs, but the tracker isn't flushed so
	// we stay contended
	ingress = ingressChangeWithNewHost(currObj, "test", "route1.other3.local")
	t1.Changed(string(currObj.UID), ingress)
	t1.flush()
	if !t1.IsChangeContended(string(currObj.UID), nowFn().Time, ingress) {
		t.Fatal("change should be contended")
	}
	makePass(t, "route1.test.local", r1, currObj, false, false)

	// after the interval expires, we no longer contend
	now = metav1.Time{Time: now.Add(3 * time.Minute)}
	nowFn = func() metav1.Time { return now }
	t1.flush()
	findIngressForRoute(currObj, "test").Host = "route1.other.local"
	currObj = makePass(t, "route1.test.local", r1, currObj, true, false)

	// multiple changes to host name don't cause contention
	currObj = makePass(t, "route2.test.local", r1, currObj, true, false)
	currObj = makePass(t, "route3.test.local", r1, currObj, true, false)
	t1.Changed(string(currObj.UID), findIngressForRoute(currObj, "test"))
	currObj = makePass(t, "route4.test.local", r1, currObj, true, false)
	t1.Changed(string(currObj.UID), findIngressForRoute(currObj, "test"))
	currObj = makePass(t, "route5.test.local", r1, currObj, true, false)
	t1.Changed(string(currObj.UID), findIngressForRoute(currObj, "test"))
	t1.Changed(string(currObj.UID), findIngressForRoute(currObj, "test"))
	currObj = makePass(t, "route6.test.local", r1, currObj, true, false)
}

func ingressChangeWithNewHost(route *routev1.Route, routerName, newHost string) *routev1.RouteIngress {
	ingress := findIngressForRoute(route, routerName).DeepCopy()
	ingress.Host = newHost
	return ingress
}

type fakeInformer struct {
	handlers []cache.ResourceEventHandler
}

func (i *fakeInformer) Update(old, obj interface{}) {
	for _, h := range i.handlers {
		h.OnUpdate(old, obj)
	}
}

func (i *fakeInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	i.handlers = append(i.handlers, handler)
}

func (i *fakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	panic("not implemented")
}

func (i *fakeInformer) GetStore() cache.Store {
	panic("not implemented")
}

func (i *fakeInformer) GetController() cache.Controller {
	panic("not implemented")
}

func (i *fakeInformer) Run(stopCh <-chan struct{}) {
	panic("not implemented")
}

func (i *fakeInformer) HasSynced() bool {
	panic("not implemented")
}

func (i *fakeInformer) LastSyncResourceVersion() string {
	panic("not implemented")
}
