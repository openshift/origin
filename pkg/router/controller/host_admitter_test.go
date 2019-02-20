package controller

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/fake"
)

const (
	BlockedTestDomain = "domain.blocked.test"
)

type rejectionRecorder struct {
	rejections map[string]string
}

func (_ rejectionRecorder) rejectionKey(route *routev1.Route) string {
	return route.Namespace + "-" + route.Name
}

func (r rejectionRecorder) RecordRouteRejection(route *routev1.Route, reason, message string) {
	r.rejections[r.rejectionKey(route)] = reason
}

func (r rejectionRecorder) Clear() {
	r.rejections = make(map[string]string)
}

func wildcardAdmitter(route *routev1.Route) error {
	if len(route.Spec.Host) < 1 {
		return nil
	}

	if strings.HasSuffix(route.Spec.Host, "."+BlockedTestDomain) {
		return fmt.Errorf("host is not allowed")
	}

	return nil
}

func wildcardRejecter(route *routev1.Route) error {
	if len(route.Spec.Host) < 1 {
		return nil
	}

	if strings.HasSuffix(route.Spec.Host, "."+BlockedTestDomain) {
		return fmt.Errorf("host is not allowed")
	}

	if len(route.Spec.WildcardPolicy) > 0 && route.Spec.WildcardPolicy != routev1.WildcardPolicyNone {
		return fmt.Errorf("wildcards not admitted test")
	}

	return nil
}

func TestHostAdmit(t *testing.T) {
	p := &fakePlugin{}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, false, LogRejections)
	tests := []struct {
		name   string
		host   string
		policy routev1.WildcardPolicyType
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
			policy: routev1.WildcardPolicyNone,
			errors: true,
		},
		{
			name:   "blockedwildcard",
			host:   "blocker." + BlockedTestDomain,
			policy: routev1.WildcardPolicySubdomain,
			errors: true,
		},
		{
			name:   "wildcard1",
			host:   "www1.aces.wild.test",
			policy: routev1.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "wildcard2",
			host:   "www2.aces.wild.test",
			policy: routev1.WildcardPolicySubdomain,
			errors: false,
		},
	}

	for _, tc := range tests {
		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tc.name,
				Namespace: "allow",
			},
			Spec: routev1.RouteSpec{
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
	admitter := NewHostAdmitter(p, wildcardRejecter, false, false, LogRejections)
	tests := []struct {
		name   string
		host   string
		policy routev1.WildcardPolicyType
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
			policy: routev1.WildcardPolicyNone,
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
			policy: routev1.WildcardPolicyNone,
			errors: true,
		},
		{
			name:   "blockedwildcard",
			host:   "www.wildcard." + BlockedTestDomain,
			policy: routev1.WildcardPolicySubdomain,
			errors: true,
		},
		{
			name:   "anotherblockedwildcard",
			host:   "api.wildcard." + BlockedTestDomain,
			policy: routev1.WildcardPolicySubdomain,
			errors: true,
		},
		{
			name:   "wildcard",
			host:   "www.aces.wild.test",
			policy: routev1.WildcardPolicySubdomain,
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
			policy: routev1.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "tldwildcard2",
			host:   "test.org",
			policy: routev1.WildcardPolicySubdomain,
			errors: false,
		},
		{
			name:   "multilevelwildcard",
			host:   "www.dept1.group2.div3.org4.com5.test",
			policy: routev1.WildcardPolicySubdomain,
			errors: false,
		},
	}

	for _, tc := range tests {
		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tc.name,
				Namespace: "deny",
			},
			Spec: routev1.RouteSpec{Host: tc.host},
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
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, false, recorder)

	oldest := metav1.Time{Time: time.Now()}

	ownerRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: oldest,
			Name:              "first",
			Namespace:         "owner",
			UID:               types.UID("uid1"),
		},
		Spec: routev1.RouteSpec{
			Host:           "owner.namespace.test",
			WildcardPolicy: routev1.WildcardPolicySubdomain,
		},
	}

	err := admitter.HandleRoute(watch.Added, ownerRoute)
	if err != nil {
		t.Fatalf("Owner route not admitted: %v", err)
	}

	tests := []struct {
		createdAt metav1.Time
		name      string
		namespace string
		host      string
		policy    routev1.WildcardPolicyType
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
			policy:    routev1.WildcardPolicyNone,
			reason:    "RouteNotAdmitted",
		},
		{
			name:      "blockedhostwildcard",
			namespace: "blocked",
			host:      "www.wildcard." + BlockedTestDomain,
			policy:    routev1.WildcardPolicySubdomain,
			reason:    "RouteNotAdmitted",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffnamespace",
			namespace: "notowner",
			host:      "www.namespace.test",
			reason:    "HostAlreadyClaimed",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffnamespace2",
			namespace: "notowner",
			host:      "www.namespace.test",
			policy:    routev1.WildcardPolicyNone,
			reason:    "HostAlreadyClaimed",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffnamespacewildcard",
			namespace: "notowner",
			host:      "www.namespace.test",
			policy:    routev1.WildcardPolicySubdomain,
			reason:    "HostAlreadyClaimed",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(2 * time.Hour)},
			name:      "diffns2",
			namespace: "fortytwo",
			host:      "www.namespace.test",
			policy:    routev1.WildcardPolicyNone,
			reason:    "HostAlreadyClaimed",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(3 * time.Hour)},
			name:      "host2diffns2",
			namespace: "fortytwo",
			host:      "api.namespace.test",
			policy:    routev1.WildcardPolicyNone,
			reason:    "HostAlreadyClaimed",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(3 * time.Hour)},
			name:      "host2diffns3",
			namespace: "fortytwo",
			host:      "api.namespace.test",
			policy:    routev1.WildcardPolicySubdomain,
			reason:    "HostAlreadyClaimed",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(4 * time.Hour)},
			name:      "ownernshost",
			namespace: "owner",
			host:      "api.namespace.test",
		},
		{
			createdAt: metav1.Time{Time: oldest.Add(4 * time.Hour)},
			name:      "ownernswildcardhost",
			namespace: "owner",
			host:      "wild.namespace.test",
			policy:    routev1.WildcardPolicySubdomain,
			reason:    "HostAlreadyClaimed",
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
			policy:    routev1.WildcardPolicyNone,
		},
		{
			name:      "tldhostwildcard",
			namespace: "wild",
			host:      "wild.play",
			policy:    routev1.WildcardPolicySubdomain,
		},
		{
			name:      "anothertldhostwildcard",
			namespace: "oscarwilde",
			host:      "oscarwilde.com",
			policy:    routev1.WildcardPolicySubdomain,
		},
		{
			name:      "yatldhostwildcard",
			namespace: "yap",
			host:      "test.me",
			policy:    routev1.WildcardPolicySubdomain,
		},
		{
			name:      "yatldhost2",
			namespace: "yap",
			host:      "vinyl.play",
			policy:    routev1.WildcardPolicyNone,
			reason:    "HostAlreadyClaimed",
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
			policy:    routev1.WildcardPolicyNone,
		},
		{
			name:      "level2sub3",
			namespace: "l2s",
			host:      "qe.co.us",
			policy:    routev1.WildcardPolicySubdomain,
		},
	}

	for idx, tc := range tests {
		ruid := fmt.Sprintf("uid%d", idx+10)
		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: tc.createdAt,
				Name:              tc.name,
				Namespace:         tc.namespace,
				UID:               types.UID(ruid),
			},
			Spec: routev1.RouteSpec{
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

	wildcardRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{Time: oldest.Add(time.Hour)},
			Name:              "wildcard-owner",
			Namespace:         "owner",
		},
		Spec: routev1.RouteSpec{
			Host:           "wildcard.namespace.test",
			WildcardPolicy: routev1.WildcardPolicySubdomain,
		},
	}

	err = admitter.HandleRoute(watch.Added, wildcardRoute)
	if err != nil {
		k := recorder.rejectionKey(wildcardRoute)
		if recorder.rejections[k] != "HostAlreadyClaimed" {
			t.Fatalf("Wildcard route expected host already claimed error, got %v - error=%v", recorder.rejections[k], err)
		}
	} else {
		t.Fatalf("Newer wildcard route expected errors, got none")
	}

	// bounce all the routes from the namespace "owner" and claim
	// ownership of the subdomain for the namespace "bouncer".
	bouncer := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{Time: oldest.Add(-1 * time.Hour)},
			Name:              "hosted",
			Namespace:         "bouncer",
		},
		Spec: routev1.RouteSpec{
			Host: "api.namespace.test",
		},
	}

	err = admitter.HandleRoute(watch.Added, bouncer)
	if err != nil {
		t.Fatalf("bouncer route expected no errors, got %v", err)
	}

	// The bouncer route should kick out the owner and wildcard routes.
	bouncedRoutes := []*routev1.Route{ownerRoute, wildcardRoute}
	for _, route := range bouncedRoutes {
		k := recorder.rejectionKey(route)
		if recorder.rejections[k] != "HostAlreadyClaimed" {
			t.Fatalf("bounced route %s expected a subdomain already claimed error, got %s", k, recorder.rejections[k])
		}
	}
}

func TestValidRouteAdmissionFuzzing(t *testing.T) {
	p := &fakePlugin{}

	admitAll := func(route *routev1.Route) error { return nil }
	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, RouteAdmissionFunc(admitAll), true, false, recorder)

	oldest := metav1.Time{Time: time.Now()}

	makeTime := func(d time.Duration) metav1.Time {
		return metav1.Time{Time: oldest.Add(d)}
	}

	routes := []*routev1.Route{
		makeRoute("ns1", "r1", "net", "", false, makeTime(0*time.Second)),
		makeRoute("ns2", "r2", "com", "", false, makeTime(1*time.Second)),
		makeRoute("ns3", "r3", "domain1.com", "", false, makeTime(2*time.Second)),
		makeRoute("ns4", "r4", "domain2.com", "", false, makeTime(3*time.Second)),
		makeRoute("ns5", "r5", "foo.domain1.com", "", false, makeTime(4*time.Second)),
		makeRoute("ns6", "r6", "bar.domain1.com", "", false, makeTime(5*time.Second)),
		makeRoute("ns7", "r7", "sub.foo.domain1.com", "", true, makeTime(6*time.Second)),
		makeRoute("ns8", "r8", "sub.bar.domain1.com", "", true, makeTime(7*time.Second)),
		makeRoute("ns8", "r9", "sub.bar.domain1.com", "/p1", true, makeTime(8*time.Second)),
		makeRoute("ns8", "r10", "sub.bar.domain1.com", "/p2", true, makeTime(9*time.Second)),
		makeRoute("ns8", "r11", "sub.bar.domain1.com", "/p1/p2/p3", true, makeTime(10*time.Second)),
		makeRoute("ns9", "r12", "sub.bar.domain2.com", "", false, makeTime(11*time.Second)),
		makeRoute("ns9", "r13", "sub.bar.domain2.com", "/p1", false, makeTime(12*time.Second)),
		makeRoute("ns9", "r14", "sub.bar.domain2.com", "/p2", false, makeTime(13*time.Second)),
	}

	rand.Seed(1)
	existing := sets.NewInt()
	errors := sets.NewString()
	for i := 0; i < 1000; i++ {
		add := false
		switch {
		case len(existing) == len(routes):
			add = false
		case len(existing) == 0:
			add = true
		default:
			add = (rand.Intn(4) > 0)
		}

		index := 0
		if add {
			index = rand.Intn(len(routes))
			if existing.Has(index) {
				// t.Logf("%d: updated route %d", i, index)
				if err := admitter.HandleRoute(watch.Modified, routes[index]); err != nil {
					errors.Insert(fmt.Sprintf("error updating route %s/%s: %v", routes[index].Namespace, routes[index].Name, err.Error()))
				}
			} else {
				// t.Logf("%d: added route %d", i, index)
				if err := admitter.HandleRoute(watch.Added, routes[index]); err != nil {
					errors.Insert(fmt.Sprintf("error adding route %s/%s: %v", routes[index].Namespace, routes[index].Name, err.Error()))
				}
			}
			existing.Insert(index)
		} else {
			index = existing.List()[rand.Intn(len(existing))]
			// t.Logf("%d: deleted route %d", i, index)
			if err := admitter.HandleRoute(watch.Deleted, routes[index]); err != nil {
				errors.Insert(fmt.Sprintf("error deleting route %s/%s: %v", routes[index].Namespace, routes[index].Name, err.Error()))
			}
			existing.Delete(index)
		}
	}

	if len(errors) > 0 {
		t.Errorf("Unexpected errors:\n%s", strings.Join(errors.List(), "\n"))
	}
	if len(recorder.rejections) > 0 {
		t.Errorf("Unexpected rejections: %#v", recorder.rejections)
	}
}

func makeRoute(ns, name, host, path string, wildcard bool, creationTimestamp metav1.Time) *routev1.Route {
	policy := routev1.WildcardPolicyNone
	if wildcard {
		policy = routev1.WildcardPolicySubdomain
	}
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: creationTimestamp,
			UID:               types.UID(fmt.Sprintf("%d_%s_%s", creationTimestamp.Time.Unix(), ns, name)),
		},
		Spec: routev1.RouteSpec{
			Host:           host,
			Path:           path,
			WildcardPolicy: policy,
		},
	}
}

func TestInvalidRouteAdmissionFuzzing(t *testing.T) {
	p := &fakePlugin{}

	admitAll := func(route *routev1.Route) error { return nil }
	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, RouteAdmissionFunc(admitAll), true, false, recorder)

	oldest := metav1.Time{Time: time.Now()}

	makeTime := func(d time.Duration) metav1.Time {
		return metav1.Time{Time: oldest.Add(d)}
	}

	routes := []struct {
		Route    *routev1.Route
		ErrIfInt sets.Int
		ErrIf    sets.String
	}{
		// Wildcard and explicit allowed in same namespace
		{Route: makeRoute("ns1", "r1", "net", "", false, makeTime(0*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r2", "net", "", true, makeTime(1*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r3", "www.same.net", "", false, makeTime(2*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r4", "www.same.net", "", true, makeTime(3*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r5", "foo.same.net", "", true, makeTime(4*time.Second)), ErrIf: sets.NewString(`ns1/r4`)},
		{Route: makeRoute("ns2", "r1", "com", "", false, makeTime(10*time.Second)), ErrIf: sets.NewString(`ns1/r2`)},
		{Route: makeRoute("ns2", "r2", "com", "", true, makeTime(11*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r2`)},
		{Route: makeRoute("ns2", "r3", "www.same.com", "", false, makeTime(12*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r4", "www.same.com", "", true, makeTime(13*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r5", "www.same.com", "/abc", true, makeTime(13*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r6", "foo.same.com", "", true, makeTime(14*time.Second)), ErrIf: sets.NewString(`ns2/r4`)},
		{Route: makeRoute("ns2", "r7", "foo.same.com", "/abc", true, makeTime(14*time.Second)), ErrIf: sets.NewString(`ns2/r5`)},
		// Fails because of other namespaces
		{Route: makeRoute("ns3", "r1", "net", "", false, makeTime(20*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r2`, `ns2/r2`)},
		{Route: makeRoute("ns3", "r2", "net", "", true, makeTime(21*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r2`, `ns2/r1`, `ns2/r2`)},
		{Route: makeRoute("ns3", "r3", "net", "/p1", true, makeTime(22*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r2`, `ns2/r1`, `ns2/r2`)},
		{Route: makeRoute("ns3", "r4", "com", "", false, makeTime(23*time.Second)), ErrIf: sets.NewString(`ns1/r2`, `ns2/r1`, `ns2/r2`)},
		{Route: makeRoute("ns3", "r5", "com", "", true, makeTime(24*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r2`, `ns2/r1`, `ns2/r2`, `ns3/r2`)},
		{Route: makeRoute("ns3", "r6", "com", "/p1/p2", true, makeTime(25*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r2`, `ns2/r1`, `ns2/r2`)},

		// Interleaved ages between namespaces
		{Route: makeRoute("ns4", "r1", "domain1.com", "", false, makeTime(30*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns5", "r1", "domain1.com", "", false, makeTime(31*time.Second)), ErrIf: sets.NewString(`ns4/r1`)},
		{Route: makeRoute("ns4", "r2", "domain1.com", "", false, makeTime(32*time.Second)), ErrIf: sets.NewString(`ns4/r1`, `ns5/r1`)},
		{Route: makeRoute("ns5", "r2", "domain1.com", "", false, makeTime(33*time.Second)), ErrIf: sets.NewString(`ns4/r1`, `ns5/r1`, `ns4/r2`)},

		// namespace with older wildcard wins over specific and wildcard routes in other namespaces
		{Route: makeRoute("ns6", "r1", "foo.domain1.com", "", true, makeTime(40*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns7", "r1", "bar.domain1.com", "", true, makeTime(50*time.Second)), ErrIf: sets.NewString(`ns6/r1`)},
		{Route: makeRoute("ns7", "r2", "bar.domain1.com", "", false, makeTime(51*time.Second)), ErrIf: sets.NewString(`ns6/r1`)},
		{Route: makeRoute("ns7", "r3", "bar.domain1.com", "/foo", false, makeTime(51*time.Second)), ErrIf: sets.NewString(`ns6/r1`)},
		{Route: makeRoute("ns8", "r1", "baz.domain1.com", "", true, makeTime(60*time.Second)), ErrIf: sets.NewString(`ns6/r1`, `ns7/r1`, `ns7/r2`, `ns7/r3`)},
		{Route: makeRoute("ns8", "r2", "baz.domain1.com", "", false, makeTime(61*time.Second)), ErrIf: sets.NewString(`ns6/r1`, `ns7/r1`)},

		// namespace with older explicit host and wildcard wins over specific and wildcard routes in other namespaces
		{Route: makeRoute("ns9", "r1", "foo.domain2.com", "", false, makeTime(40*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns10", "r1", "bar.domain2.com", "", true, makeTime(50*time.Second)), ErrIf: sets.NewString(`ns9/r1`)},
		{Route: makeRoute("ns10", "r2", "bar.domain2.com", "", false, makeTime(51*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns10", "r3", "foo.domain2.com", "", false, makeTime(52*time.Second)), ErrIf: sets.NewString(`ns9/r1`)},
		{Route: makeRoute("ns10", "r4", "foo.domain2.com", "/p1", false, makeTime(53*time.Second)), ErrIf: sets.NewString(`ns9/r1`)},
		{Route: makeRoute("ns10", "r5", "foo.domain2.com", "/p2", false, makeTime(54*time.Second)), ErrIf: sets.NewString(`ns9/r1`)},
		{Route: makeRoute("ns10", "r6", "foo.domain2.com", "/p1/p2/other", false, makeTime(55*time.Second)), ErrIf: sets.NewString(`ns9/r1`)},
		{Route: makeRoute("ns10", "r7", "foo.domain2.com", "/someother", false, makeTime(56*time.Second)), ErrIf: sets.NewString(`ns9/r1`)},
		{Route: makeRoute("ns11", "r1", "baz.domain2.com", "", true, makeTime(60*time.Second)), ErrIf: sets.NewString(`ns9/r1`, `ns10/r1`, `ns10/r2`, `ns10/r3`, `ns10/r4`, `ns10/r5`, `ns10/r6`, `ns10/r7`)},
		{Route: makeRoute("ns11", "r2", "baz.domain2.com", "", false, makeTime(61*time.Second)), ErrIf: sets.NewString(`ns10/r1`)},

		// namespace with specific and wildcard route with paths wins over specific and wildcard routes in other namespaces
		{Route: makeRoute("ns12", "r1", "foo.domain3.com", "", false, makeTime(70*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns12", "r2", "bar.domain3.com", "/abc", false, makeTime(71*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns13", "r1", "foo.domain3.com", "", true, makeTime(80*time.Second)), ErrIf: sets.NewString(`ns12/r1`, `ns12/r2`)},
		{Route: makeRoute("ns13", "r2", "bar.domain3.com", "", false, makeTime(81*time.Second)), ErrIf: sets.NewString(`ns12/r2`)},
		{Route: makeRoute("ns13", "r3", "bar.domain3.com", "/abc", false, makeTime(82*time.Second)), ErrIf: sets.NewString(`ns12/r2`)},
		{Route: makeRoute("ns13", "r4", "bar.domain3.com", "/def", false, makeTime(83*time.Second)), ErrIf: sets.NewString(`ns12/r2`)},
		{Route: makeRoute("ns13", "r5", "wild.domain3.com", "/aces", true, makeTime(84*time.Second)), ErrIf: sets.NewString(`ns12/r1`, `ns12/r2`)},
		{Route: makeRoute("ns13", "r6", "wild.domain3.com", "", true, makeTime(85*time.Second)), ErrIf: sets.NewString(`ns12/r1`, `ns12/r2`, `ns13/r1`)},
		{Route: makeRoute("ns14", "r1", "foo.domain3.com", "", false, makeTime(90*time.Second)), ErrIf: sets.NewString(`ns12/r1`, `ns13/r1`, `ns13/r5`, `ns13/r6`)},
		{Route: makeRoute("ns14", "r2", "bar.domain3.com", "", false, makeTime(91*time.Second)), ErrIf: sets.NewString(`ns12/r2`, `ns13/r1`, `ns13/r2`, `ns13/r3`, `ns13/r4`, `ns13/r5`, `ns13/r6`)},

		// namespace with oldest wildcard and non-wildcard routes with same paths wins
		{Route: makeRoute("ns15", "r1", "foo.domain4.com", "", false, makeTime(100*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns15", "r2", "foo.domain4.com", "/abc", false, makeTime(101*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns15", "r3", "foo.domain4.com", "", false, makeTime(102*time.Second)), ErrIf: sets.NewString(`ns15/r1`)},
		{Route: makeRoute("ns15", "r4", "foo.domain4.com", "/abc", false, makeTime(103*time.Second)), ErrIf: sets.NewString(`ns15/r2`)},
		{Route: makeRoute("ns15", "r5", "www.domain4.com", "", true, makeTime(104*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns15", "r6", "www.domain4.com", "/abc", true, makeTime(105*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns15", "r7", "www.domain4.com", "", true, makeTime(106*time.Second)), ErrIf: sets.NewString(`ns15/r5`)},
		{Route: makeRoute("ns15", "r8", "www.domain4.com", "/abc", true, makeTime(107*time.Second)), ErrIf: sets.NewString(`ns15/r6`)},
		{Route: makeRoute("ns15", "r9", "www.domain4.com", "/def", true, makeTime(108*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns15", "r10", "www.domain4.com", "/def", true, makeTime(109*time.Second)), ErrIf: sets.NewString(`ns15/r9`)},
	}

	nameToIndex := map[string]int{}
	for i, tc := range routes {
		name := tc.Route.Namespace + "/" + tc.Route.Name
		if _, exists := nameToIndex[name]; exists {
			t.Fatalf("%d has a duplicate route name %s", i, name)
		}
		nameToIndex[name] = i
	}
	for i, tc := range routes {
		errIfInt := sets.NewInt()
		for name := range tc.ErrIf {
			if index, ok := nameToIndex[name]; ok {
				errIfInt.Insert(index)
			} else {
				t.Fatalf("%d references an unknown route name: %s", i, name)
			}
		}
		tc.ErrIfInt = errIfInt
		routes[i] = tc
	}

	rand.Seed(1)
	existing := sets.NewInt()
	errors := sets.NewString()
	for i := 0; i < 10000; i++ {
		add := false
		switch {
		case len(existing) == len(routes):
			add = false
		case len(existing) == 0:
			add = true
		default:
			add = (rand.Intn(4) > 0)
		}

		index := 0
		eventType := watch.Deleted
		if add {
			index = rand.Intn(len(routes))
			if existing.Has(index) {
				eventType = watch.Modified
			} else {
				eventType = watch.Added
			}
		} else {
			index = existing.List()[rand.Intn(len(existing))]
			eventType = watch.Deleted
		}

		route := routes[index].Route
		err := admitter.HandleRoute(eventType, route)
		if eventType != watch.Deleted && existing.HasAny(routes[index].ErrIfInt.List()...) {
			if err == nil {
				errors.Insert(fmt.Sprintf("no error %s route %s/%s (existing=%v, errif=%v)", eventType, route.Namespace, route.Name, existing.List(), routes[index].ErrIfInt.List()))
			}
		} else {
			if err != nil {
				errors.Insert(fmt.Sprintf("error %s route %s/%s: %v (existing=%v, errif=%v)", eventType, route.Namespace, route.Name, err.Error(), existing.List(), routes[index].ErrIfInt.List()))
			}
		}

		existingNames := sets.NewString()
		for _, routes := range admitter.claimedHosts {
			for _, route := range routes {
				existingNames.Insert(route.Namespace + "/" + route.Name)
			}
		}
		for _, routes := range admitter.claimedWildcards {
			for _, route := range routes {
				existingNames.Insert(route.Namespace + "/" + route.Name)
			}
		}
		for _, routes := range admitter.blockedWildcards {
			for _, route := range routes {
				if !existingNames.Has(route.Namespace + "/" + route.Name) {
					t.Fatalf("blockedWildcards has %s/%s, not in claimedHosts or claimedWildcards", route.Namespace, route.Name)
				}
			}
		}
		existing = sets.NewInt()
		for name := range existingNames {
			index, ok := nameToIndex[name]
			if !ok {
				t.Fatalf("unknown route %s", name)
			}
			existing.Insert(index)
		}
	}

	if len(errors) > 0 {
		t.Errorf("Unexpected errors:\n%s", strings.Join(errors.List(), "\n"))
	}
}

func TestStatusWildcardPolicyNoOp(t *testing.T) {
	now := nowFn()
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset()
	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, false, recorder)
	err := admitter.HandleRoute(watch.Added, &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "thing", UID: types.UID("uid8")},
		Spec: routev1.RouteSpec{
			Host:           "wild.test.local",
			WildcardPolicy: routev1.WildcardPolicySubdomain,
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "wild.test.local",
					RouterName: "wilder",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               routev1.RouteAdmitted,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
					WildcardPolicy: routev1.WildcardPolicySubdomain,
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

func TestStatusWildcardPolicyNotAllowedNoOp(t *testing.T) {
	now := nowFn()
	touched := metav1.Time{Time: now.Add(-time.Minute)}
	p := &fakePlugin{}
	c := fake.NewSimpleClientset()
	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, wildcardAdmitter, false, false, recorder)
	err := admitter.HandleRoute(watch.Added, &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "thing", UID: types.UID("uid8")},
		Spec: routev1.RouteSpec{
			Host:           "wild.test.local",
			WildcardPolicy: "nono",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host:       "wild.test.local",
					RouterName: "wilder",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:               "RouteNotAdmitted",
							Status:             corev1.ConditionTrue,
							LastTransitionTime: &touched,
						},
					},
					WildcardPolicy: routev1.WildcardPolicyNone,
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

func TestDisableOwnershipChecksFuzzing(t *testing.T) {
	p := &fakePlugin{}

	admitAll := func(route *routev1.Route) error { return nil }
	recorder := rejectionRecorder{rejections: make(map[string]string)}
	uniqueHostPlugin := NewUniqueHost(p, true, recorder)
	admitter := NewHostAdmitter(uniqueHostPlugin, RouteAdmissionFunc(admitAll), true, true, recorder)

	oldest := metav1.Time{Time: time.Now()}

	makeTime := func(d time.Duration) metav1.Time {
		return metav1.Time{Time: oldest.Add(d)}
	}

	routes := []struct {
		Route    *routev1.Route
		ErrIfInt sets.Int
		ErrIf    sets.String
	}{
		// Wildcard and explicit allowed in different namespaces.
		{Route: makeRoute("ns1", "r1", "org", "", true, makeTime(0*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r2", "org", "/p1", false, makeTime(1*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r3", "www.w3.org", "", false, makeTime(2*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r4", "www.w3.org", "/p1", true, makeTime(3*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns1", "r5", "info", "", false, makeTime(4*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r1", "info", "/p1", false, makeTime(10*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r2", "www.server.info", "", false, makeTime(11*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r3", "www.server.info", "/p1", false, makeTime(12*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r4", "wild.server.info", "", true, makeTime(13*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r5", "wilder.server.info", "/p1", true, makeTime(14*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns2", "r6", "org", "/other", false, makeTime(15*time.Second)), ErrIf: sets.NewString()},

		// Fails because of another wildcard/regular route
		{Route: makeRoute("ns3", "r1", "org", "", true, makeTime(20*time.Second)), ErrIf: sets.NewString(`ns1/r1`)},
		{Route: makeRoute("ns3", "r2", "org", "/p1", false, makeTime(21*time.Second)), ErrIf: sets.NewString(`ns1/r2`)},
		{Route: makeRoute("ns3", "r3", "org", "", true, makeTime(22*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns3/r1`)},
		{Route: makeRoute("ns3", "r4", "info", "", true, makeTime(23*time.Second)), ErrIf: sets.NewString(`ns1/r1`, `ns1/r5`, `ns3/r1`, `ns3/r3`)},

		{Route: makeRoute("ns4", "r1", "www.server.info", "", false, makeTime(24*time.Second)), ErrIf: sets.NewString(`ns2/r2`)},
		{Route: makeRoute("ns4", "r2", "www.server.info", "/p1", false, makeTime(25*time.Second)), ErrIf: sets.NewString(`ns2/r3`)},
		{Route: makeRoute("ns4", "r3", "wild.server.info", "", true, makeTime(26*time.Second)), ErrIf: sets.NewString(`ns2/r4`)},
		{Route: makeRoute("ns4", "r4", "wild.server.info", "", true, makeTime(27*time.Second)), ErrIf: sets.NewString(`ns2/r4`, `ns4/r3`)},
		{Route: makeRoute("ns4", "r5", "wilder.server.info", "/p1", true, makeTime(28*time.Second)), ErrIf: sets.NewString(`ns2/r5`)},

		// Works because of uniqueness.
		{Route: makeRoute("ns5", "r1", "org", "/abc", true, makeTime(30*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns5", "r2", "www.server.info", "/xyz", false, makeTime(31*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("p5", "r3", "www.server.info", "/abc/xyz", true, makeTime(32*time.Second)), ErrIf: sets.NewString()},

		// Interleaved ages between namespaces
		{Route: makeRoute("ns6", "r1", "somedomain.org", "", false, makeTime(40*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns7", "r1", "somedomain.org", "", false, makeTime(41*time.Second)), ErrIf: sets.NewString(`ns6/r1`)},
		{Route: makeRoute("ns6", "r2", "somedomain.org", "", false, makeTime(42*time.Second)), ErrIf: sets.NewString(`ns6/r1`, `ns7/r1`)},
		{Route: makeRoute("ns7", "r2", "somedomain.org", "", false, makeTime(43*time.Second)), ErrIf: sets.NewString(`ns6/r1`, `ns7/r1`, `ns6/r2`)},

		// namespace with older wildcard wins over specific but allows non-overlapping routes in other namespaces
		{Route: makeRoute("ns8", "r1", "foo.somedomain.org", "", true, makeTime(50*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns8", "r2", "foo.somedomain.org", "/path1", true, makeTime(51*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns9", "r1", "foo.somedomain.org", "", true, makeTime(52*time.Second)), ErrIf: sets.NewString(`ns8/r1`)},
		{Route: makeRoute("ns9", "r2", "foo.somedomain.org", "/path1", true, makeTime(53*time.Second)), ErrIf: sets.NewString(`ns8/r2`)},
		{Route: makeRoute("ns9", "r3", "bar.somedomain.org", "", false, makeTime(54*time.Second)), ErrIf: sets.NewString()},
		{Route: makeRoute("ns10", "r1", "baz.somedomain.org", "", true, makeTime(55*time.Second)), ErrIf: sets.NewString(`ns8/r1`, `ns9/r1`)},
		{Route: makeRoute("ns10", "r2", "foo.somedomain.org", "", true, makeTime(56*time.Second)), ErrIf: sets.NewString(`ns8/r1`, `ns9/r1`, `ns10/r1`)},
		{Route: makeRoute("ns10", "r3", "bar.somedomain.org", "", false, makeTime(57*time.Second)), ErrIf: sets.NewString(`ns9/r3`)},
		{Route: makeRoute("ns10", "r4", "bar.somedomain.org", "/path2", false, makeTime(58*time.Second)), ErrIf: sets.NewString()},
	}

	nameToIndex := map[string]int{}
	for i, tc := range routes {
		name := tc.Route.Namespace + "/" + tc.Route.Name
		if _, exists := nameToIndex[name]; exists {
			t.Fatalf("%d has a duplicate route name %s", i, name)
		}
		nameToIndex[name] = i
	}
	for i, tc := range routes {
		errIfInt := sets.NewInt()
		for name := range tc.ErrIf {
			if index, ok := nameToIndex[name]; ok {
				errIfInt.Insert(index)
			} else {
				t.Fatalf("%d references an unknown route name: %s", i, name)
			}
		}
		tc.ErrIfInt = errIfInt
		routes[i] = tc
	}

	rand.Seed(1)
	existing := sets.NewInt()
	errors := sets.NewString()
	for i := 0; i < 10000; i++ {
		add := false
		switch {
		case len(existing) == len(routes):
			add = false
		case len(existing) == 0:
			add = true
		default:
			add = (rand.Intn(4) > 0)
		}

		index := 0
		eventType := watch.Deleted
		if add {
			index = rand.Intn(len(routes))
			if existing.Has(index) {
				eventType = watch.Modified
			} else {
				eventType = watch.Added
			}
		} else {
			index = existing.List()[rand.Intn(len(existing))]
			eventType = watch.Deleted
		}

		route := routes[index].Route
		err := admitter.HandleRoute(eventType, route)
		if eventType != watch.Deleted && existing.HasAny(routes[index].ErrIfInt.List()...) {
			k := recorder.rejectionKey(route)
			if err == nil && (recorder.rejections[k] != "HostAlreadyClaimed") {
				errors.Insert(fmt.Sprintf("no error %s route %s/%s (existing=%v, errif=%v)", eventType, route.Namespace, route.Name, existing.List(), routes[index].ErrIfInt.List()))
			}
		} else {
			//
			if eventType != watch.Deleted && err != nil {
				errors.Insert(fmt.Sprintf("error %s route %s/%s: %v (existing=%v, errif=%v)", eventType, route.Namespace, route.Name, err.Error(), existing.List(), routes[index].ErrIfInt.List()))
			}
			if eventType == watch.Deleted && err == nil {
				delete(recorder.rejections, recorder.rejectionKey(route))
			}
		}

		existingNames := sets.NewString()
		for _, routes := range admitter.claimedHosts {
			for _, route := range routes {
				existingNames.Insert(route.Namespace + "/" + route.Name)
			}
		}
		for _, routes := range admitter.claimedWildcards {
			for _, route := range routes {
				existingNames.Insert(route.Namespace + "/" + route.Name)
			}
		}
		for _, routes := range admitter.blockedWildcards {
			for _, route := range routes {
				if !existingNames.Has(route.Namespace + "/" + route.Name) {
					t.Fatalf("blockedWildcards has %s/%s, not in claimedHosts or claimedWildcards", route.Namespace, route.Name)
				}
			}
		}
		existing = sets.NewInt()
		for name := range existingNames {
			index, ok := nameToIndex[name]
			if !ok {
				t.Fatalf("unknown route %s", name)
			}
			existing.Insert(index)
		}
	}

	if len(errors) > 0 {
		t.Errorf("Unexpected errors:\n%s", strings.Join(errors.List(), "\n"))
	}
}

func TestHandleNamespaceProcessing(t *testing.T) {
	p := &fakePlugin{}
	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, false, recorder)

	// Set namespaces handled in the host admitter plugin, the fakePlugin in
	// the test chain doesn't support this, so ignore not expected error.
	err := admitter.HandleNamespaces(sets.NewString("ns1", "ns2", "nsx"))
	if err != nil && err.Error() != "not expected" {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name      string
		namespace string
		host      string
		policy    routev1.WildcardPolicyType
		expected  bool
	}{
		{
			name:      "expected",
			namespace: "ns1",
			host:      "update.expected.test",
			policy:    routev1.WildcardPolicyNone,
			expected:  true,
		},
		{
			name:      "not-expected",
			namespace: "updatemenot",
			host:      "no-update.expected.test",
			policy:    routev1.WildcardPolicyNone,
			expected:  false,
		},
		{
			name:      "expected-wild",
			namespace: "ns1",
			host:      "update.wild.expected.test",
			policy:    routev1.WildcardPolicySubdomain,
			expected:  true,
		},
		{
			name:      "not-expected-wild-not-owner",
			namespace: "nsx",
			host:      "second.wild.expected.test",
			policy:    routev1.WildcardPolicySubdomain,
			expected:  false,
		},
		{
			name:      "not-expected-wild",
			namespace: "otherns",
			host:      "noupdate.wild.expected.test",
			policy:    routev1.WildcardPolicySubdomain,
			expected:  false,
		},
		{
			name:      "expected-wild-other-subdomain",
			namespace: "nsx",
			host:      "host.third.wild.expected.test",
			policy:    routev1.WildcardPolicySubdomain,
			expected:  true,
		},
		{
			name:      "not-expected-plain-2",
			namespace: "notallowed",
			host:      "not.allowed.expected.test",
			policy:    routev1.WildcardPolicyNone,
			expected:  false,
		},
		{
			name:      "not-expected-blocked",
			namespace: "nsx",
			host:      "blitz.domain.blocked.test",
			policy:    routev1.WildcardPolicyNone,
			expected:  false,
		},
		{
			name:      "not-expected-blocked-wildcard",
			namespace: "ns2",
			host:      "wild.blocked.domain.blocked.test",
			policy:    routev1.WildcardPolicySubdomain,
			expected:  false,
		},
	}

	for _, tc := range tests {
		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tc.name,
				Namespace: tc.namespace,
				UID:       types.UID(tc.name),
			},
			Spec: routev1.RouteSpec{
				Host:           tc.host,
				WildcardPolicy: tc.policy,
			},
			Status: routev1.RouteStatus{
				Ingress: []routev1.RouteIngress{
					{
						Host:           tc.host,
						RouterName:     "nsproc",
						Conditions:     []routev1.RouteIngressCondition{},
						WildcardPolicy: tc.policy,
					},
				},
			},
		}

		err := admitter.HandleRoute(watch.Added, route)
		if tc.expected {
			if err != nil {
				t.Fatalf("test case %s unexpected error: %v", tc.name, err)
			}
			if !reflect.DeepEqual(p.route, route) {
				t.Fatalf("test case %s expected route to be processed: %+v", tc.name, route)
			}
		} else if err == nil && reflect.DeepEqual(p.route, route) {
			t.Fatalf("test case %s did not expected route to be processed: %+v", tc.name, route)
		}
	}
}

func TestWildcardPathRoutesWithoutNSCheckResyncs(t *testing.T) {
	p := &fakePlugin{}

	recorder := rejectionRecorder{rejections: make(map[string]string)}
	admitter := NewHostAdmitter(p, wildcardAdmitter, true, true, recorder)

	oldest := metav1.Time{Time: time.Now()}

	tests := []struct {
		namespace string
		name      string
		host      string
		path      string
		wildcard  bool
		createdAt metav1.Time
		errors    bool
	}{
		{
			namespace: "wildness",
			name:      "owner-wildcard-path",
			host:      "star.wildcard.test",
			path:      "/wildflowers",
			wildcard:  true,
			createdAt: oldest,
			errors:    false,
		},
		{
			namespace: "wildness",
			name:      "same-ns-wildcard-frontend-nopath",
			host:      "star.wildcard.test",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(1 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildness",
			name:      "same-ns-wildcard-mobile-path",
			host:      "star.wildcard.test",
			path:      "/mobile",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(2 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildness",
			name:      "same-ns-wildcard-auth-path",
			host:      "star.wildcard.test",
			path:      "/auth",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(3 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildness",
			name:      "same-ns-wildcard-nopath-rejected",
			host:      "star.wildcard.test",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(4 * time.Hour)},
			errors:    true,
		},
		{
			namespace: "wildness",
			name:      "same-ns-plain-nopath",
			host:      "plain.wildcard.test",
			createdAt: metav1.Time{Time: oldest.Add(5 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildness",
			name:      "same-ns-plain-path",
			host:      "star.wildcard.test",
			path:      "/plain/rain",
			createdAt: metav1.Time{Time: oldest.Add(6 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildness",
			name:      "same-ns-dup-plain-nopath-rejected",
			host:      "plain.wildcard.test",
			createdAt: metav1.Time{Time: oldest.Add(7 * time.Hour)},
			errors:    true,
		},
		{
			namespace: "bewilder",
			name:      "other-ns-wildcard-status-path",
			host:      "star.wildcard.test",
			path:      "/status",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(10 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "bewilder",
			name:      "other-ns-plain-nopath-rejected",
			host:      "plain.wildcard.test",
			createdAt: metav1.Time{Time: oldest.Add(11 * time.Hour)},
			errors:    true,
		},
		{
			namespace: "bewilder",
			name:      "other-ns-plain-path",
			host:      "star.wildcard.test",
			path:      "/explain/ed",
			createdAt: metav1.Time{Time: oldest.Add(12 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildcat",
			name:      "another-ns-wildcard-nopath-rejected",
			host:      "star.wildcard.test",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(20 * time.Hour)},
			errors:    true,
		},
		{
			namespace: "wildcat",
			name:      "another-ns-dup-wildcard-path-rejected",
			host:      "star.wildcard.test",
			path:      "/auth",
			wildcard:  true,
			createdAt: metav1.Time{Time: oldest.Add(21 * time.Hour)},
			errors:    true,
		},
		{
			namespace: "wildcat",
			name:      "another-ns-plain-path",
			host:      "plain.wildcard.test",
			path:      "/re/explain/ed",
			createdAt: metav1.Time{Time: oldest.Add(22 * time.Hour)},
			errors:    false,
		},
		{
			namespace: "wildcat",
			name:      "another-ns-plain-path-rejected",
			host:      "star.wildcard.test",
			path:      "/plain/rain",
			createdAt: metav1.Time{Time: oldest.Add(23 * time.Hour)},
			errors:    true,
		},
	}

	routes := make([]*routev1.Route, len(tests))
	for idx, tc := range tests {
		route := makeRoute(tc.namespace, tc.name, tc.host, tc.path, tc.wildcard, tc.createdAt)
		routes[idx] = route

		err := admitter.HandleRoute(watch.Added, route)
		if tc.errors {
			if err == nil {
				k := recorder.rejectionKey(route)
				rejection := recorder.rejections[k]
				t.Fatalf("Test case %s expected errors, got none rejection=%s", tc.name, rejection)
			}

			k := recorder.rejectionKey(route)
			if _, ok := recorder.rejections[k]; !ok {
				t.Fatalf("Test case %s expected a rejection, got none", tc.name)
			}
		} else {
			if err != nil {
				t.Fatalf("Test case %s expected no errors, got %v", tc.name, err)
			}
		}
	}

	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < 10000; i++ {
		index := rand.Intn(len(tests))
		tc := tests[index]
		route := routes[index]

		eventType := watch.Modified
		if rand.Intn(100)%2 == 0 {
			eventType = watch.Added
		}

		// recorder.Clear()
		err := admitter.HandleRoute(eventType, route)
		if tc.errors {
			if err == nil {
				t.Fatalf("resync route for test case %s expected errors, got none", tc.name)
			}

			k := recorder.rejectionKey(route)
			if _, ok := recorder.rejections[k]; !ok {
				t.Fatalf("resync route for test case %s expected a rejection, got none", tc.name)
			}
		} else {
			if err != nil {
				t.Fatalf("resync route for test case %s expected no errors, got %v", tc.name, err)
			}

			k := recorder.rejectionKey(route)
			if rejection, ok := recorder.rejections[k]; ok {
				t.Fatalf("resync route for test case %s event=%s expected no rejection, got %s", tc.name, eventType, rejection)
			}
		}
	}
}
