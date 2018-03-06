package controller

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/route/generated/internalclientset/fake"
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
	route *routeapi.Route
	err   error
}

func (p *fakePlugin) HandleRoute(t watch.EventType, route *routeapi.Route) error {
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

type recorded struct {
	at      time.Time
	ingress *routeapi.RouteIngress
}

type fakeTracker struct {
	contended map[string]recorded
	cleared   map[string]recorded
	results   map[string]bool
}

func (t *fakeTracker) IsContended(id string, now time.Time, ingress *routeapi.RouteIngress) bool {
	if t.contended == nil {
		t.contended = make(map[string]recorded)
	}
	t.contended[id] = recorded{
		at:      now,
		ingress: ingress,
	}
	return t.results[id]
}

func (t *fakeTracker) Clear(id string, ingress *routeapi.RouteIngress) {
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
	admitter := NewStatusAdmitter(p, c.Route(), "test", "a.b.c.d", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:                    "route1.test.local",
					RouterName:              "test",
					RouterCanonicalHostname: "a.b.c.d",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Actions()) > 0 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
}

func checkResult(t *testing.T, err error, c *fake.Clientset, admitter *StatusAdmitter, targetHost string, targetObjTime metav1.Time, targetCachedTime *time.Time, ingressInd int, actionInd int) *routeapi.Route {
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
	obj := c.Actions()[actionInd].(clientgotesting.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != ingressInd+1 || obj.Status.Ingress[ingressInd].Host != targetHost {
		t.Fatalf("expected route reset: expected %q / actual %q -- %#v", targetHost, obj.Status.Ingress[ingressInd].Host, obj)
	}
	condition := obj.Status.Ingress[ingressInd].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != targetObjTime || condition.Status != kapi.ConditionTrue || condition.Reason != "" {
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
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	})

	checkResult(t, err, c, admitter, "route1.test.local", now, &now.Time, 0, 0)
}

func TestStatusAdmitsRouteOnForbidden(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		return true, nil, errors.NewForbidden(kapi.Resource("Route"), "route1", nil)
	})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	})
	checkResult(t, err, c, admitter, "route1.test.local", now, &touched.Time, 0, 0)
}

func TestStatusBackoffOnConflict(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		return true, nil, errors.NewConflict(kapi.Resource("Route"), "route1", nil)
	})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	})
	checkResult(t, err, c, admitter, "route1.test.local", now, nil, 0, 0)
}

func TestStatusRecordRejection(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
	}, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
}

func TestStatusRecordRejectionNoChange(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route1.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							Reason:             "Failed",
							Message:            "generic error",
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}, "Failed", "generic error")

	if len(c.Actions()) != 0 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
}

func TestStatusRecordRejectionWithStatus(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
}

func TestStatusRecordRejectionOnHostUpdateOnly(t *testing.T) {
	now := nowFn()
	nowFn = func() metav1.Time { return now }
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched,
							Reason:             "Failed",
							Message:            "generic error",
						},
					},
				},
			},
		},
	}, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
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
	c := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		return true, nil, errors.NewConflict(kapi.Resource("Route"), "route1", nil)
	})
	tracker := &fakeTracker{}
	admitter := NewStatusAdmitter(p, c.Route(), "test", "", noopLease{}, tracker)
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched,
						},
					},
				},
			},
		},
	}, "Failed", "generic error")

	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource().Resource != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(clientgotesting.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 || obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
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
	c1 := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker1 := &fakeTracker{}
	admitter1 := NewStatusAdmitter(p, c1.Route(), "test", "", noopLease{}, tracker1)
	err := admitter1.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status:     routeapi.RouteStatus{},
	})

	outObj1 := checkResult(t, err, c1, admitter1, "route1.test.local", now1, &now1.Time, 0, 0)
	if tracker1.cleared["uid1"].at != now1.Time {
		t.Fatal(tracker1)
	}

	// the new deployment's replica
	now2 := metav1.Time{Time: now1.Time.Add(2 * time.Minute)}
	nowFn = func() metav1.Time { return now2 }
	c2 := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	tracker2 := &fakeTracker{}
	admitter2 := NewStatusAdmitter(p, c2.Route(), "test", "", noopLease{}, tracker2)
	outObj1.Spec.Host = "route1.test-new.local"
	err = admitter2.HandleRoute(watch.Added, outObj1)

	outObj2 := checkResult(t, err, c2, admitter2, "route1.test-new.local", now2, &now2.Time, 0, 0)
	if tracker2.cleared["uid1"].at != now2.Time {
		t.Fatal(tracker2)
	}

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
	c1 := fake.NewSimpleClientset(&routeapi.Route{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")}})
	returnConflict := true
	c1.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if returnConflict {
			returnConflict = false
			return true, nil, errors.NewConflict(kapi.Resource("Route"), "route1", nil)
		}
		return false, nil, nil
	})
	tracker := &fakeTracker{}
	admitter1 := NewStatusAdmitter(p, c1.Route(), "test2", "", noopLease{}, tracker)
	err := admitter1.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route2.test-new.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route1.test.local",
					RouterName: "test1",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched1,
						},
					},
				},
				{
					Host:       "route1.test-new.local",
					RouterName: "test2",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched1,
						},
					},
				},
			},
		},
	})

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
	//c2 := fake.NewSimpleClientset(&routeapi.Route{})
	err = admitter1.HandleRoute(watch.Modified, &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route2.test-new.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route2.test.local",
					RouterName: "test1",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched1,
						},
					},
				},
				{
					Host:       "route1.test-new.local",
					RouterName: "test2",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Type:               routeapi.RouteAdmitted,
							Status:             kapi.ConditionFalse,
							LastTransitionTime: &touched2,
						},
					},
				},
			},
		},
	})

	checkResult(t, err, c1, admitter1, "route2.test-new.local", now1, &now2.Time, 1, 0)
	if tracker.contended["uid1"].at != now2.Time {
		t.Fatalf("should have recorded uid1 into tracker: %#v", tracker)
	}
}

func makePass(t *testing.T, host string, admitter *StatusAdmitter, srcObj *routeapi.Route, expectUpdate bool, conflict bool) *routeapi.Route {
	t.Helper()
	// initialize a new client
	c := fake.NewSimpleClientset(srcObj)
	if conflict {
		c.PrependReactor("update", "routes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() != "status" {
				return false, nil, nil
			}
			return true, nil, errors.NewConflict(kapi.Resource("Route"), "route1", nil)
		})
	}

	admitter.client = c.Route()

	inputObj := srcObj.DeepCopy()
	inputObj.Spec.Host = host

	err := admitter.HandleRoute(watch.Modified, inputObj)

	if expectUpdate {
		now := nowFn()
		var nowTime *time.Time
		if !conflict {
			nowTime = &now.Time
		}
		return checkResult(t, err, c, admitter, inputObj.Spec.Host, now, nowTime, 0, 0)
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

// This test tries an extended interaction between two "old" and two "new"
// router replicas.
func TestProtractedStatusFightBetweenRouters(t *testing.T) {
	p := &fakePlugin{}
	stopCh := make(chan struct{})
	defer close(stopCh)

	oldHost := "route1.test.local"
	newHost := "route1.test-new.local"
	oldHost2 := "route2.test.local"
	newHost2 := "route2.test-new.local"
	newHost3 := "route3.test-new.local"

	now := metav1.Now()
	nowFn = func() metav1.Time { return now }

	initObj := &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: newHost},
		Status:     routeapi.RouteStatus{},
	}

	// NB: contention period is 1 minute
	t1, t2, t3, t4 := NewSimpleContentionTracker(time.Minute), NewSimpleContentionTracker(time.Minute), NewSimpleContentionTracker(time.Minute), NewSimpleContentionTracker(time.Minute)

	oldAdmitter1 := NewStatusAdmitter(p, nil, "test", "", noopLease{}, t1)
	oldAdmitter2 := NewStatusAdmitter(p, nil, "test", "", noopLease{}, t2)
	newAdmitter1 := NewStatusAdmitter(p, nil, "test", "", noopLease{}, t3)
	newAdmitter2 := NewStatusAdmitter(p, nil, "test", "", noopLease{}, t4)

	t.Logf("Setup up the two 'old' routers")
	currObj := makePass(t, oldHost, oldAdmitter1, initObj, true, false)
	makePass(t, oldHost, oldAdmitter2, currObj, false, false)

	t.Logf("Phase in the two 'new' routers (with the second getting a conflict)...")
	now = metav1.Time{Time: now.Add(10 * time.Minute)}
	nowFn = func() metav1.Time { return now }
	makePass(t, newHost, newAdmitter2, currObj, true, true)
	currObj = makePass(t, newHost, newAdmitter1, currObj, true, false)

	t.Logf("...which should cause 'new' router #2 to receive an update and ignore it...")
	now = metav1.Time{Time: now.Add(1 * time.Second)}
	nowFn = func() metav1.Time { return now }
	makePass(t, newHost, newAdmitter2, currObj, false, false)

	t.Logf("...and cause the two 'old' routers to react (#2 conflicts)...")
	makePass(t, oldHost, oldAdmitter2, currObj, true, true)
	currObj = makePass(t, oldHost, oldAdmitter1, currObj, true, true)

	t.Logf("...causing 'old' #2 and 'new' #1 and #2 to receive updates...")
	now = metav1.Time{Time: now.Add(1 * time.Second)}
	nowFn = func() metav1.Time { return now }

	t.Logf("...where none of them react (leaving the 'old' status during the rolling update)")
	makePass(t, newHost, newAdmitter1, currObj, false, false)
	makePass(t, oldHost, oldAdmitter2, currObj, false, false)
	makePass(t, newHost, newAdmitter2, currObj, false, false)

	t.Logf("If we now send out a route update, 'old' router #1 should update the status...")
	now = metav1.Time{Time: now.Add(4 * time.Second)}
	nowFn = func() metav1.Time { return now }
	currObj = makePass(t, oldHost2, oldAdmitter1, currObj, true, false)

	t.Logf("...and the other routers should ignore the update...")
	makePass(t, newHost2, newAdmitter1, currObj, false, false)
	makePass(t, newHost2, newAdmitter2, currObj, false, false)
	makePass(t, oldHost2, oldAdmitter2, currObj, false, false)

	t.Logf("...and should receive an second update due to 'old' router #1, and ignore that as well")
	now = metav1.Time{Time: now.Add(1 * time.Second)}
	nowFn = func() metav1.Time { return now }
	makePass(t, newHost2, newAdmitter2, currObj, false, false)
	makePass(t, oldHost2, oldAdmitter1, currObj, false, false)
	makePass(t, oldHost2, oldAdmitter2, currObj, false, false)

	t.Logf("If an update occurs after the contention period has elapsed...")
	now = metav1.Time{Time: now.Add(1 * time.Minute)}
	nowFn = func() metav1.Time { return now }

	t.Logf("both of the 'new' routers should try to update the status, since the cache has expired")
	makePass(t, newHost3, newAdmitter1, currObj, true, true)
	currObj = makePass(t, newHost3, newAdmitter2, currObj, true, false)

	makePass(t, newHost3, newAdmitter1, currObj, false, false)
}
