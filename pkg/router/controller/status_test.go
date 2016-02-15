package controller

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client/testclient"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

type fakePlugin struct {
	t     watch.EventType
	route *routeapi.Route
	err   error
}

func (p *fakePlugin) HandleRoute(t watch.EventType, route *routeapi.Route) error {
	p.t, p.route = t, route
	return p.err
}
func (p *fakePlugin) HandleEndpoints(watch.EventType, *kapi.Endpoints) error {
	return fmt.Errorf("not expected")
}
func (p *fakePlugin) HandleNamespaces(namespaces sets.String) error {
	return fmt.Errorf("not expected")
}

func TestStatusNoOp(t *testing.T) {
	now := unversioned.Now()
	touched := unversioned.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := testclient.NewSimpleFake()
	admitter := NewStatusAdmitter(p, c, "test")
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
		Spec:       routeapi.RouteSpec{Host: "route1.test.local"},
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					Host:       "route1.test.local",
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Actions()) > 0 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
}

func TestStatusResetsHost(t *testing.T) {
	now := unversioned.Now()
	nowFn = func() unversioned.Time { return now }
	touched := unversioned.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := testclient.NewSimpleFake(&routeapi.Route{})
	admitter := NewStatusAdmitter(p, c, "test")
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource() != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(ktestclient.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 && obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionTrue || condition.Reason != "" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
	if v, ok := admitter.expected.Peek(types.UID("uid1")); !ok || !reflect.DeepEqual(v, now.Time) {
		t.Fatalf("did not record last modification time: %#v %#v", admitter.expected, v)
	}
}

func TestStatusBackoffOnConflict(t *testing.T) {
	now := unversioned.Now()
	nowFn = func() unversioned.Time { return now }
	touched := unversioned.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := testclient.NewSimpleFake(&(errors.NewConflict(kapi.Resource("Route"), "route1", nil).(*errors.StatusError).ErrStatus))
	admitter := NewStatusAdmitter(p, c, "test")
	err := admitter.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
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
	if len(c.Actions()) != 1 {
		t.Fatalf("unexpected actions: %#v", c.Actions())
	}
	action := c.Actions()[0]
	if action.GetVerb() != "update" || action.GetResource() != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(ktestclient.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 && obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionTrue || condition.Reason != "" {
		t.Fatalf("unexpected condition: %#v", condition)
	}

	if err == nil {
		t.Fatalf("unexpected non-error: %#v", admitter.expected)
	}
	if v, ok := admitter.expected.Peek(types.UID("uid1")); !ok || !reflect.DeepEqual(v, time.Time{}) {
		t.Fatalf("expected empty time: %#v", v)
	}
}

func TestStatusRecordRejection(t *testing.T) {
	now := unversioned.Now()
	nowFn = func() unversioned.Time { return now }
	touched := unversioned.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := testclient.NewSimpleFake(&routeapi.Route{})
	admitter := NewStatusAdmitter(p, c, "test")
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
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
	if action.GetVerb() != "update" || action.GetResource() != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(ktestclient.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 && obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
	if v, ok := admitter.expected.Peek(types.UID("uid1")); !ok || !reflect.DeepEqual(v, now.Time) {
		t.Fatalf("expected empty time: %#v", v)
	}
}

func TestStatusRecordRejectionConflict(t *testing.T) {
	now := unversioned.Now()
	nowFn = func() unversioned.Time { return now }
	touched := unversioned.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := testclient.NewSimpleFake(&(errors.NewConflict(kapi.Resource("Route"), "route1", nil).(*errors.StatusError).ErrStatus))
	admitter := NewStatusAdmitter(p, c, "test")
	admitter.RecordRouteRejection(&routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route1", Namespace: "default", UID: types.UID("uid1")},
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
	if action.GetVerb() != "update" || action.GetResource() != "routes" || action.GetSubresource() != "status" {
		t.Fatalf("unexpected action: %#v", action)
	}
	obj := c.Actions()[0].(ktestclient.UpdateAction).GetObject().(*routeapi.Route)
	if len(obj.Status.Ingress) != 1 && obj.Status.Ingress[0].Host != "route1.test.local" {
		t.Fatalf("expected route reset: %#v", obj)
	}
	condition := obj.Status.Ingress[0].Conditions[0]
	if condition.LastTransitionTime == nil || *condition.LastTransitionTime != now || condition.Status != kapi.ConditionFalse || condition.Reason != "Failed" || condition.Message != "generic error" {
		t.Fatalf("unexpected condition: %#v", condition)
	}
	if v, ok := admitter.expected.Peek(types.UID("uid1")); !ok || !reflect.DeepEqual(v, time.Time{}) {
		t.Fatalf("expected empty time: %#v", v)
	}
}

func TestFindOrCreateIngress(t *testing.T) {
	route := &routeapi.Route{
		Status: routeapi.RouteStatus{
			Ingress: []routeapi.RouteIngress{
				{
					RouterName: "bar",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Reason: "bar",
						},
					},
				},
				{
					RouterName: "foo",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Reason: "foo1",
						},
					},
				},
				{
					RouterName: "baz",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Reason: "baz",
						},
					},
				},
				{
					RouterName: "foo",
					Conditions: []routeapi.RouteIngressCondition{
						{
							Reason: "foo2",
						},
					},
				},
			},
		},
	}

	routerName := "foo"
	ingress, changed := findOrCreateIngress(route, routerName)
	if !changed {
		t.Errorf("expected the route list to be changed: %#v", route.Status.Ingress)
	}
	if ingress.RouterName != routerName {
		t.Errorf("returned ingress had router name %s but expected %s", ingress.RouterName, routerName)
	}
	t.Logf("routes: %#v", route.Status.Ingress)
}
