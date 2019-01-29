package hostindex

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/route/controller/routeapihelpers"
)

func oldest(changes Changed, active []*routev1.Route, routes ...*routev1.Route) (updated, displaced []*routev1.Route) {
	if len(routes) == 0 {
		return active, nil
	}
	route := routes[0]
	if len(active) == 0 {
		changes.Activated(route)
		return []*routev1.Route{route}, routes[1:]
	}
	if routeapihelpers.RouteLessThan(active[0], route) {
		return active, routes
	}
	changes.Activated(route)
	for _, displaced := range active {
		changes.Displaced(displaced)
	}
	return []*routev1.Route{route}, append(active, routes[1:]...)
}

func newRoute(namespace, name string, uid, rv int, spec routev1.RouteSpec) *routev1.Route {
	route := &routev1.Route{
		Spec: spec,
	}
	route.Name = name
	route.Namespace = namespace
	route.CreationTimestamp.Time = route.CreationTimestamp.Add(time.Duration(uid) * 100 * time.Millisecond)
	route.UID = types.UID(fmt.Sprintf("%03d", uid))
	route.ResourceVersion = fmt.Sprintf("%d", rv)
	return route
}

func Test_hostIndex(t *testing.T) {
	type step struct {
		remove bool
		route  *routev1.Route
	}
	tests := []struct {
		name       string
		activateFn RouteActivationFunc
		steps      []step

		// assert the return values
		newRoute bool
		// assert the reported change state for the last step
		activates map[string]struct{}
		displaces map[string]struct{}
		// assert the state of the index
		active   map[string][]string
		inactive map[string][]string
	}{
		{
			name: "add",
			steps: []step{
				{route: newRoute("test", "1", 0, 1, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"000"}},
			activates: map[string]struct{}{"000": {}},
			newRoute:  true,
		},
		{
			name: "add - stable",
			steps: []step{
				{route: newRoute("test", "1", 0, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 0, 2, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"000"}},
			activates: map[string]struct{}{"000": {}},
		},
		{
			name: "add - UID change",
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 2, 2, routev1.RouteSpec{Host: "test.com"})},
			},
			active: map[string][]string{"test.com": {"002"}},
			// because the UID changes, we aren't displacing ourselves
			activates: map[string]struct{}{"002": {}},
		},
		{
			name: "add - UID change in reverse order",
			steps: []step{
				{route: newRoute("test", "1", 2, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 2, routev1.RouteSpec{Host: "test.com"})},
			},
			active: map[string][]string{"test.com": {"001"}},
			// because the UID changes, we aren't displacing ourselves
			activates: map[string]struct{}{"001": {}},
		},
		{
			name: "add - skip displacement",
			steps: []step{
				{route: newRoute("test", "1", 0, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 1, 2, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"000"}},
			inactive:  map[string][]string{"test.com": {"001"}},
			displaces: map[string]struct{}{"001": {}},
			newRoute:  true,
		},
		{
			name: "add - displace",
			steps: []step{
				{route: newRoute("test", "2", 1, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 0, 2, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"000"}},
			activates: map[string]struct{}{"000": {}},
			inactive:  map[string][]string{"test.com": {"001"}},
			displaces: map[string]struct{}{"001": {}},
			newRoute:  true,
		},
		{
			name: "coexist",
			steps: []step{
				{route: newRoute("test", "1", 0, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 1, 2, routev1.RouteSpec{Host: "test2.com"})},
			},
			active: map[string][]string{
				"test.com":  {"000"},
				"test2.com": {"001"},
			},
			activates: map[string]struct{}{"001": {}},
			newRoute:  true,
		},
		{
			name: "update - active",
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 2, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"001"}},
			activates: map[string]struct{}{"001": {}},
		},
		{
			name: "update - no change",
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
			},
			active: map[string][]string{"test.com": {"001"}},
		},
		{
			name: "update - inactive",
			steps: []step{
				{route: newRoute("test", "2", 11, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 2, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "2", 11, 3, routev1.RouteSpec{Host: "test.com"})},
			},
			active: map[string][]string{"test.com": {"001"}},
		},
		{
			name: "update - displace when hostname changes",
			steps: []step{
				{route: newRoute("test", "1", 11, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 1, 2, routev1.RouteSpec{Host: "test2.com"})},
				{route: newRoute("test", "2", 1, 3, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"001"}},
			activates: map[string]struct{}{"001": {}},
			inactive:  map[string][]string{"test.com": {"011"}},
			displaces: map[string]struct{}{"011": {}},
		},
		{
			name: "remove - does not exist",
			steps: []step{
				{remove: true, route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
			},
		},
		{
			name: "remove - become active",
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 11, 2, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"011"}},
			activates: map[string]struct{}{"011": {}},
		},
		{
			name: "remove - multiple inactive",
			steps: []step{
				{route: newRoute("test", "3", 111, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 11, 2, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "1", 1, 4, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"011"}},
			activates: map[string]struct{}{"011": {}},
			inactive:  map[string][]string{"test.com": {"111"}},
		},
		{
			name: "remove - multiple inactive - all",
			steps: []step{
				{route: newRoute("test", "3", 111, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 11, 2, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "1", 1, 4, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "2", 11, 5, routev1.RouteSpec{Host: "test.com"})},
			},
			active:    map[string][]string{"test.com": {"111"}},
			activates: map[string]struct{}{"111": {}},
		},
		{
			name: "remove - inactive",
			steps: []step{
				{route: newRoute("test", "2", 11, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 2, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "2", 11, 3, routev1.RouteSpec{Host: "test.com"})},
			},
			active: map[string][]string{"test.com": {"001"}},
		},
		{
			name: "remove - does not become active",
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 11, 2, routev1.RouteSpec{Host: "test.com"})},
				{remove: true, route: newRoute("test", "2", 11, 3, routev1.RouteSpec{Host: "test.com"})},
			},
			active: map[string][]string{"test.com": {"001"}},
		},
		{
			name:       "multiple changes to same path-based route",
			activateFn: SameNamespace,
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{route: newRoute("test", "1", 1, 2, routev1.RouteSpec{Host: "test.com", Path: "/bar"})},
				{route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{route: newRoute("test", "1", 1, 4, routev1.RouteSpec{Host: "test.com", Path: "/bar"})},
				{route: newRoute("test", "1", 1, 5, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
			},
			active:    map[string][]string{"test.com": {"001"}},
			activates: map[string]struct{}{"001": {}},
		},
		{
			name:       "remove unchanged path-based route",
			activateFn: SameNamespace,
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{remove: true, route: newRoute("test", "1", 0, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
			},
		},
		{
			name:       "remove updated path-based route",
			activateFn: SameNamespace,
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{route: newRoute("test", "1", 1, 2, routev1.RouteSpec{Host: "test.com", Path: "/bar"})},
				{route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{route: newRoute("test", "1", 1, 4, routev1.RouteSpec{Host: "test.com", Path: "/bar"})},
				{remove: true, route: newRoute("test", "1", 1, 4, routev1.RouteSpec{Host: "test.com", Path: "/bar"})},
			},
		},
		{
			name:       "missed delete of path-based route",
			activateFn: SameNamespace,
			steps: []step{
				{route: newRoute("test", "1", 2, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{route: newRoute("test", "2", 1, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
			},
			newRoute:  true,
			active:    map[string][]string{"test.com": {"001"}},
			activates: map[string]struct{}{"001": {}},
			displaces: map[string]struct{}{"002": {}},
			inactive:  map[string][]string{"test.com": {"002"}},
		},
		{
			name:       "path-based route rejection",
			activateFn: SameNamespace,
			steps: []step{
				{route: newRoute("test", "1", 1, 1, routev1.RouteSpec{Host: "test.com", Path: "/x/y/z"})},
				{route: newRoute("test", "2", 2, 1, routev1.RouteSpec{Host: "test.com", Path: "/foo"})},
				{route: newRoute("test", "2", 2, 2, routev1.RouteSpec{Host: "test.com", Path: "/bar"})},
				{route: newRoute("test", "2", 2, 3, routev1.RouteSpec{Host: "test.com", Path: "/x/y/z"})},
			},
			active:    map[string][]string{"test.com": {"001"}},
			displaces: map[string]struct{}{"002": {}},
			inactive:  map[string][]string{"test.com": {"002"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.activates == nil {
				tt.activates = make(map[string]struct{})
			}
			if tt.displaces == nil {
				tt.displaces = make(map[string]struct{})
			}
			if tt.active == nil {
				tt.active = make(map[string][]string)
			}
			if tt.inactive == nil {
				tt.inactive = make(map[string][]string)
			}

			fn := tt.activateFn
			if fn == nil {
				fn = oldest
			}
			hi := New(fn).(*hostIndex)
			var changed Changes
			var newRoute bool
			for _, step := range tt.steps {
				if step.remove {
					changed = hi.Remove(step.route)
					newRoute = false
				} else {
					changed, newRoute = hi.Add(step.route)
				}
			}

			if newRoute != tt.newRoute {
				t.Errorf("Unexpected new route status: %t", newRoute)
			}

			activates := changesToMap(changed.GetActivated())
			if !reflect.DeepEqual(tt.activates, activates) {
				t.Errorf("Unexpected activated changes: %s", diff.ObjectReflectDiff(tt.activates, activates))
			}

			displaces := changesToMap(changed.GetDisplaced())
			if !reflect.DeepEqual(tt.displaces, displaces) {
				t.Errorf("Unexpected displaced changes: %s", diff.ObjectReflectDiff(tt.displaces, displaces))
			}

			active := make(map[string][]string)
			inactive := make(map[string][]string)
			for host, rules := range hi.hostToRoute {
				for _, route := range rules.active {
					active[host] = append(active[host], string(route.UID))
				}
				for _, route := range rules.inactive {
					inactive[host] = append(inactive[host], string(route.UID))
				}
			}
			if !reflect.DeepEqual(tt.active, active) {
				t.Errorf("Unexpected active: %s", diff.ObjectReflectDiff(tt.active, active))
			}
			if !reflect.DeepEqual(tt.inactive, inactive) {
				t.Errorf("Unexpected inactive: %s", diff.ObjectReflectDiff(tt.inactive, inactive))
			}
		})
	}
}

func Test_Filter(t *testing.T) {
	type step struct {
		remove bool
		route  *routev1.Route
	}
	tests := []struct {
		name       string
		activateFn RouteActivationFunc
		steps      []step
		filter     func(route *routev1.Route) bool

		// assert the state of the index
		active   map[string][]string
		inactive map[string][]string
		// assert the reported change state for the last step
		activates map[string]struct{}
		displaces map[string]struct{}
	}{
		{
			name: "multiple active and inactive routes",
			steps: []step{
				{route: newRoute("test", "3", 111, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 11, 2, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "4", 12, 4, routev1.RouteSpec{Host: "test4.com"})},
				{route: newRoute("test", "5", 13, 5, routev1.RouteSpec{Host: "test5.com"})},
			},
			filter: func(route *routev1.Route) bool {
				return route.Name == "5" || route.Name == "2"
			},
			active: map[string][]string{
				"test.com":  {"011"},
				"test5.com": {"013"},
			},
			activates: map[string]struct{}{"011": {}},
		},
		{
			name: "remove all inactive",
			steps: []step{
				{route: newRoute("test", "3", 111, 1, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "2", 11, 2, routev1.RouteSpec{Host: "test.com"})},
				{route: newRoute("test", "1", 1, 3, routev1.RouteSpec{Host: "test.com"})},
			},
			filter: func(route *routev1.Route) bool {
				return route.Name == "1"
			},
			active: map[string][]string{
				"test.com": {"001"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.activates == nil {
				tt.activates = make(map[string]struct{})
			}
			if tt.displaces == nil {
				tt.displaces = make(map[string]struct{})
			}
			if tt.active == nil {
				tt.active = make(map[string][]string)
			}
			if tt.inactive == nil {
				tt.inactive = make(map[string][]string)
			}

			fn := tt.activateFn
			if fn == nil {
				fn = oldest
			}
			hi := New(fn).(*hostIndex)
			for _, step := range tt.steps {
				if step.remove {
					hi.Remove(step.route)
				} else {
					hi.Add(step.route)
				}
			}

			changed := hi.Filter(tt.filter)

			activates := changesToMap(changed.GetActivated())
			if !reflect.DeepEqual(tt.activates, activates) {
				t.Errorf("Unexpected activated changes: %s", diff.ObjectReflectDiff(tt.activates, activates))
			}

			displaces := changesToMap(changed.GetDisplaced())
			if !reflect.DeepEqual(tt.displaces, displaces) {
				t.Errorf("Unexpected displaced changes: %s", diff.ObjectReflectDiff(tt.displaces, displaces))
			}

			active := make(map[string][]string)
			inactive := make(map[string][]string)
			for host, rules := range hi.hostToRoute {
				for _, route := range rules.active {
					active[host] = append(active[host], string(route.UID))
				}
				for _, route := range rules.inactive {
					inactive[host] = append(inactive[host], string(route.UID))
				}
			}
			if !reflect.DeepEqual(tt.active, active) {
				t.Errorf("Unexpected active: %s", diff.ObjectReflectDiff(tt.active, active))
			}
			if !reflect.DeepEqual(tt.inactive, inactive) {
				t.Errorf("Unexpected inactive: %s", diff.ObjectReflectDiff(tt.inactive, inactive))
			}
		})
	}
}

func changesToMap(routes []*routev1.Route) map[string]struct{} {
	m := make(map[string]struct{})
	for _, route := range routes {
		m[string(route.UID)] = struct{}{}
	}
	return m
}
