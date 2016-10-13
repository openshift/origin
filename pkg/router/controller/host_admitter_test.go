package controller

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

const (
	BlockedTestDomain = "domain.blocked.test"
)

type rejectionRecorder struct {
	rejections map[string]string
}

func (_ rejectionRecorder) rejectionKey(route *routeapi.Route) string {
	return route.Namespace + "-" + route.Name
}

func (r rejectionRecorder) RecordRouteRejection(route *routeapi.Route, reason, message string) {
	r.rejections[r.rejectionKey(route)] = reason
}

func wildcardAdmitter(route *routeapi.Route) error {
	if len(route.Spec.Host) < 1 {
		return nil
	}

	if strings.HasSuffix(route.Spec.Host, "."+BlockedTestDomain) {
		return fmt.Errorf("host is not allowed")
	}

	return nil
}

func wildcardRejecter(route *routeapi.Route) error {
	if len(route.Spec.Host) < 1 {
		return nil
	}

	if strings.HasSuffix(route.Spec.Host, "."+BlockedTestDomain) {
		return fmt.Errorf("host is not allowed")
	}

	_, wildcard := routeapi.NormalizeWildcardHost(route.Spec.Host)
	if wildcard {
		return fmt.Errorf("wildcards not admitted test")
	}

	return nil
}

func TestHostAdmit(t *testing.T) {
	p := &fakePlugin{}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, LogRejections)
	tests := []struct {
		name   string
		host   string
		errors bool
	}{
		{
			name:   "nohost",
			errors: false,
		},
		{
			name:   "allowed",
			host:   "www.host.admission.test",
			errors: false,
		},
		{
			name:   "blocked",
			host:   "www." + BlockedTestDomain,
			errors: true,
		},
		{
			name:   "wildcard",
			host:   "*.aces.wild.test",
			errors: false,
		},
		{
			name:   "blockedwildcard",
			host:   "*." + BlockedTestDomain,
			errors: true,
		},
	}

	for _, tc := range tests {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      tc.name,
				Namespace: "allow",
			},
			Spec: routeapi.RouteSpec{Host: tc.host},
		}

		err := admitter.HandleRoute(watch.Added, route)
		if tc.errors {
			if err == nil {
				t.Fatalf("Test case %s expected errors, got none", tc.name)
			}
		} else {
			if err != nil {
				t.Fatalf("Test case %s expected no errors, got %v", tc.name, err)
			}
		}
	}
}

func TestWildcardHostDeny(t *testing.T) {
	p := &fakePlugin{}
	admitter := NewHostAdmitter(p, wildcardRejecter, false, LogRejections)
	tests := []struct {
		name   string
		host   string
		errors bool
	}{
		{
			name:   "nohost",
			errors: false,
		},
		{
			name:   "allowed",
			host:   "www.host.admission.test",
			errors: false,
		},
		{
			name:   "blocked",
			host:   "www.wildcard." + BlockedTestDomain,
			errors: true,
		},
		{
			name:   "wildcard",
			host:   "*.aces.wild.test",
			errors: true,
		},
		{
			name:   "blockedwildcard",
			host:   "*.wildcard." + BlockedTestDomain,
			errors: true,
		},
		{
			name:   "anotherblockedwildcard",
			host:   "api.wildcard." + BlockedTestDomain,
			errors: true,
		},
	}

	for _, tc := range tests {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      tc.name,
				Namespace: "deny",
			},
			Spec: routeapi.RouteSpec{Host: tc.host},
		}

		err := admitter.HandleRoute(watch.Added, route)
		if tc.errors {
			if err == nil {
				t.Fatalf("Test case %s expected errors, got none", tc.name)
			}
		} else {
			if err != nil {
				t.Fatalf("Test case %s expected no errors, got %v", tc.name, err)
			}
		}
	}
}

func TestWildcardSubDomainOwnership(t *testing.T) {
	p := &fakePlugin{}

	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, recorder)

	oldest := unversioned.Time{Time: time.Now()}

	ownerRoute := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: oldest,
			Name:              "first",
			Namespace:         "owner",
		},
		Spec: routeapi.RouteSpec{
			Host: "owner.namespace.test",
		},
	}

	err := admitter.HandleRoute(watch.Added, ownerRoute)
	if err != nil {
		t.Fatalf("Owner route not admitted: %v", err)
	}

	tests := []struct {
		createdAt unversioned.Time
		name      string
		namespace string
		host      string
		reason    string
	}{
		{
			name:      "nohost",
			namespace: "something",
		},
		{
			name:      "blockedhost",
			namespace: "blocked",
			host:      "www.wildcard." + BlockedTestDomain,
			reason:    "RouteNotAdmitted",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffnamespace",
			namespace: "notowner",
			host:      "www.namespace.test",
			reason:    "SubdomainAlreadyClaimed",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffns2",
			namespace: "fortytwo",
			host:      "www.namespace.test",
			reason:    "SubdomainAlreadyClaimed",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(3 * time.Hour)},
			name:      "host2diffns2",
			namespace: "fortytwo",
			host:      "api.namespace.test",
			reason:    "SubdomainAlreadyClaimed",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(4 * time.Hour)},
			name:      "ownernshost",
			namespace: "owner",
			host:      "api.namespace.test",
		},
	}

	for _, tc := range tests {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: tc.createdAt,
				Name:              tc.name,
				Namespace:         tc.namespace,
			},
			Spec: routeapi.RouteSpec{Host: tc.host},
		}

		err := admitter.HandleRoute(watch.Added, route)
		if tc.reason != "" {
			if err == nil {
				t.Fatalf("Test case %s expected errors, got none", tc.name)
			}

			k := recorder.rejectionKey(route)
			if recorder.rejections[k] != tc.reason {
				t.Fatalf("Test case %s expected error %s, got %s", tc.name, tc.reason, recorder.rejections[k])
			}
		} else {
			if err != nil {
				t.Fatalf("Test case %s expected no errors, got %v", tc.name, err)
			}
		}
	}

	wildcardRoute := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: unversioned.Time{Time: oldest.Add(time.Hour)},
			Name:              "wildcard-owner",
			Namespace:         "owner",
		},
		Spec: routeapi.RouteSpec{
			Host: "*.namespace.test",
		},
	}

	err = admitter.HandleRoute(watch.Added, wildcardRoute)
	if err != nil {
		t.Fatalf("Wildcard route not admitted: %v", err)
	}

	// bounce all the routes from the namespace "owner" and claim
	// ownership of the subdomain for the namespace "bouncer".
	bouncer := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: unversioned.Time{Time: oldest.Add(-1 * time.Hour)},
			Name:              "hosted",
			Namespace:         "bouncer",
		},
		Spec: routeapi.RouteSpec{
			Host: "api.namespace.test",
		},
	}

	err = admitter.HandleRoute(watch.Added, bouncer)
	if err != nil {
		t.Fatalf("bouncer route expected no errors, got %v", err)
	}

	// The bouncer route should kick out the owner and wildcard routes.
	bouncedRoutes := []*routeapi.Route{ownerRoute, wildcardRoute}
	for _, route := range bouncedRoutes {
		k := recorder.rejectionKey(route)
		if recorder.rejections[k] != "SubdomainAlreadyClaimed" {
			t.Fatalf("bounced route %s expected a subdomain already claimed error, got %s", k, recorder.rejections[k])
		}
	}
}
