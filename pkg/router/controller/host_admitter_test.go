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

func wildcardAdmitter(route *routeapi.Route) (error, bool) {
	if len(route.Spec.Host) < 1 {
		return nil, true
	}

	if strings.HasSuffix(route.Spec.Host, "."+BlockedTestDomain) {
		return fmt.Errorf("host is not allowed"), false
	}

	return nil, true
}

func wildcardRejecter(route *routeapi.Route) (error, bool) {
	if len(route.Spec.Host) < 1 {
		return nil, true
	}

	if strings.HasSuffix(route.Spec.Host, "."+BlockedTestDomain) {
		return fmt.Errorf("host is not allowed"), false
	}

	if len(route.Spec.WildcardPolicy) > 0 && route.Spec.WildcardPolicy != routeapi.WildcardPolicyNone {
		return fmt.Errorf("wildcards not admitted test"), true
	}

	return nil, true
}

func TestHostAdmit(t *testing.T) {
	p := &fakePlugin{}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, LogRejections)
	tests := []struct {
		name   string
		host   string
		policy routeapi.WildcardPolicyType
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
			name:   "blocked2",
			host:   "www." + BlockedTestDomain,
			policy: routeapi.WildcardPolicyNone,
			errors: true,
		},
		{
			name:   "blockedwildcard",
			host:   "blocker." + BlockedTestDomain,
			policy: routeapi.WildcardPolicySubdomain,
			errors: true,
		},
		{
			name:   "wildcard1",
			host:   "www1.aces.wild.test",
			policy: routeapi.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "wildcard2",
			host:   "www2.aces.wild.test",
			policy: routeapi.WildcardPolicySubdomain,
			errors: false,
		},
	}

	for _, tc := range tests {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      tc.name,
				Namespace: "allow",
			},
			Spec: routeapi.RouteSpec{
				Host:           tc.host,
				WildcardPolicy: tc.policy,
			},
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
		policy routeapi.WildcardPolicyType
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
			name:   "allowed2",
			host:   "www.host.admission.test",
			policy: routeapi.WildcardPolicyNone,
			errors: false,
		},
		{
			name:   "blocked",
			host:   "www.wildcard." + BlockedTestDomain,
			errors: true,
		},
		{
			name:   "anotherblockedhost",
			host:   "api.wildcard." + BlockedTestDomain,
			policy: routeapi.WildcardPolicyNone,
			errors: true,
		},
		{
			name:   "blockedwildcard",
			host:   "www.wildcard." + BlockedTestDomain,
			policy: routeapi.WildcardPolicySubdomain,
			errors: true,
		},
		{
			name:   "anotherblockedwildcard",
			host:   "api.wildcard." + BlockedTestDomain,
			policy: routeapi.WildcardPolicySubdomain,
			errors: true,
		},
		{
			name:   "wildcard",
			host:   "www.aces.wild.test",
			policy: routeapi.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "tld1",
			host:   "wild1.test",
			errors: false,
		},
		{
			name:   "tld2",
			host:   "test.org",
			errors: false,
		},
		{
			name:   "tldwildcard",
			host:   "wild.test",
			policy: routeapi.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "tldwildcard2",
			host:   "test.org",
			policy: routeapi.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "multilevelwildcard",
			host:   "www.dept1.group2.div3.org4.com5.test",
			policy: routeapi.WildcardPolicySubdomain,
			errors: false,
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
			Host:           "owner.namespace.test",
			WildcardPolicy: routeapi.WildcardPolicySubdomain,
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
		policy    routeapi.WildcardPolicyType
		reason    string
	}{
		{
			name:      "nohost",
			namespace: "something",
		},
		{
			name:      "blockedhost",
			namespace: "blocked",
			host:      "www.internal." + BlockedTestDomain,
			reason:    "RouteNotAdmitted",
		},
		{
			name:      "blockedhost2",
			namespace: "blocked",
			host:      "www.internal." + BlockedTestDomain,
			policy:    routeapi.WildcardPolicyNone,
			reason:    "RouteNotAdmitted",
		},
		{
			name:      "blockedhostwildcard",
			namespace: "blocked",
			host:      "www.wildcard." + BlockedTestDomain,
			policy:    routeapi.WildcardPolicySubdomain,
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
			name:      "diffnamespace2",
			namespace: "notowner",
			host:      "www.namespace.test",
			policy:    routeapi.WildcardPolicyNone,
			reason:    "SubdomainAlreadyClaimed",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffnamespacewildcard",
			namespace: "notowner",
			host:      "www.namespace.test",
			policy:    routeapi.WildcardPolicySubdomain,
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
			policy:    routeapi.WildcardPolicyNone,
			reason:    "SubdomainAlreadyClaimed",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(3 * time.Hour)},
			name:      "host2diffns3",
			namespace: "fortytwo",
			host:      "api.namespace.test",
			policy:    routeapi.WildcardPolicySubdomain,
			reason:    "SubdomainAlreadyClaimed",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(4 * time.Hour)},
			name:      "ownernshost",
			namespace: "owner",
			host:      "api.namespace.test",
		},
		{
			createdAt: unversioned.Time{Time: oldest.Add(4 * time.Hour)},
			name:      "ownernswildcardhost",
			namespace: "owner",
			host:      "wild.namespace.test",
			policy:    routeapi.WildcardPolicySubdomain,
		},
		{
			name:      "tldhost",
			namespace: "ns1",
			host:      "ns1.org",
		},
		{
			name:      "tldhost2",
			namespace: "ns2",
			host:      "ns2.org",
			policy:    routeapi.WildcardPolicyNone,
		},
		{
			name:      "tldhostwildcard",
			namespace: "wild",
			host:      "wild.play",
			policy:    routeapi.WildcardPolicySubdomain,
		},
		{
			name:      "anothertldhostwildcard",
			namespace: "oscarwilde",
			host:      "oscarwilde.com",
			policy:    routeapi.WildcardPolicySubdomain,
		},
		{
			name:      "yatldhostwildcard",
			namespace: "yap",
			host:      "test.me",
			policy:    routeapi.WildcardPolicySubdomain,
		},
		{
			name:      "yatldhost2",
			namespace: "yap",
			host:      "vinyl.play",
			policy:    routeapi.WildcardPolicyNone,
		},
		{
			name:      "level2sub",
			namespace: "l2s",
			host:      "test.co.us",
		},
		{
			name:      "level2sub2",
			namespace: "l2s",
			host:      "unit.co.us",
			policy:    routeapi.WildcardPolicyNone,
		},
		{
			name:      "level2sub3",
			namespace: "l2s",
			host:      "qe.co.us",
			policy:    routeapi.WildcardPolicySubdomain,
		},
	}

	for _, tc := range tests {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: tc.createdAt,
				Name:              tc.name,
				Namespace:         tc.namespace,
			},
			Spec: routeapi.RouteSpec{
				Host:           tc.host,
				WildcardPolicy: tc.policy,
			},
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
			Host:           "wildcard.namespace.test",
			WildcardPolicy: routeapi.WildcardPolicySubdomain,
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
