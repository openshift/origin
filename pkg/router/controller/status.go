package controller

import (
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// StatusAdmitter ensures routes added to the plugin have status set.
type StatusAdmitter struct {
	plugin     router.Plugin
	client     client.RoutesNamespacer
	routerName string
}

// NewStatusAdmitter creates a plugin wrapper that ensures every accepted
// route has a status field set that matches this router.
func NewStatusAdmitter(plugin router.Plugin, client client.RoutesNamespacer, name string) *StatusAdmitter {
	return &StatusAdmitter{
		plugin:     plugin,
		client:     client,
		routerName: name,
	}
}

// findOrCreateIngress loops through the router status ingress array looking for an entry
// that matches name. If there is no entry in the array, it creates one and appends it
// to the array. If there are multiple entries with that name, the first one is
// returned and later ones are removed. True is returned if the route was altered.
func findOrCreateIngress(route *routeapi.Route, name string) (*routeapi.RouteIngress, bool) {
	position := -1
	changed := false
	updated := make([]routeapi.RouteIngress, 0, len(route.Status.Ingress))
	for i := range route.Status.Ingress {
		existing := &route.Status.Ingress[i]
		if existing.RouterName != name {
			updated = append(updated, *existing)
			continue
		}
		if position != -1 {
			changed = true
			continue
		}
		position = i
	}
	switch {
	case position == -1:
		position = len(route.Status.Ingress)
		route.Status.Ingress = append(route.Status.Ingress, routeapi.RouteIngress{
			RouterName: name,
			Host:       route.Spec.Host,
		})
		changed = true
	case changed:
		route.Status.Ingress = updated
	}
	ingress := &route.Status.Ingress[position]
	return ingress, changed
}

// admitRoute returns true if the route has already been accepted to this router, or
// updates the route to contain an accepted condition. Returns an error if the route could
// not be admitted.
func admitRoute(oc client.RoutesNamespacer, route *routeapi.Route, name string) (bool, error) {
	ingress, updated := findOrCreateIngress(route, name)
	for i := range ingress.Conditions {
		cond := &ingress.Conditions[i]
		if cond.Type == routeapi.RouteAdmitted && cond.Status == kapi.ConditionTrue {
			glog.V(4).Infof("route already admitted")
			return true, nil
		}
	}
	now := util.Now()
	ingress.Conditions = []routeapi.RouteIngressCondition{
		{
			Type:               routeapi.RouteAdmitted,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: &now,
		},
	}
	glog.V(4).Infof("admitting route by updating status: %s (%t): %#v", route.Name, updated, route)
	out, err := oc.Routes(route.Namespace).UpdateStatus(route)
	if err != nil {
		glog.Errorf("unable to update status: %v", err)
		return false, err
	}
	glog.V(4).Infof("after status update: %#v", out)
	return false, nil
}

func (a *StatusAdmitter) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	switch eventType {
	case watch.Added, watch.Modified:
		ok, err := admitRoute(a.client, route, a.routerName)
		if err != nil {
			return err
		}
		if !ok {
			glog.V(4).Infof("skipping route: %s", route.Name)
			return nil
		}
	}
	return a.plugin.HandleRoute(eventType, route)
}

func (a *StatusAdmitter) HandleEndpoints(eventType watch.EventType, route *kapi.Endpoints) error {
	return a.plugin.HandleEndpoints(eventType, route)
}

func (a *StatusAdmitter) HandleNamespaces(namespaces util.StringSet) error {
	return a.plugin.HandleNamespaces(namespaces)
}
