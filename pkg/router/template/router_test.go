package templaterouter

import (
	"crypto/md5"
	"fmt"
	"testing"

	routeapi "github.com/openshift/origin/pkg/route/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

// TestCreateServiceUnit tests creating a service unit and finding it in router state
func TestCreateServiceUnit(t *testing.T) {
	router := NewFakeTemplateRouter()
	suKey := "test"
	router.CreateServiceUnit("test")

	if _, ok := router.FindServiceUnit(suKey); !ok {
		t.Errorf("Unable to find serivce unit %s after creation", suKey)
	}
}

// TestDeleteServiceUnit tests that deleted service units no longer exist in state
func TestDeleteServiceUnit(t *testing.T) {
	router := NewFakeTemplateRouter()
	suKey := "test"
	router.CreateServiceUnit(suKey)

	if _, ok := router.FindServiceUnit(suKey); !ok {
		t.Errorf("Unable to find serivce unit %s after creation", suKey)
	}

	router.DeleteServiceUnit(suKey)

	if _, ok := router.FindServiceUnit(suKey); ok {
		t.Errorf("Service unit %s was found in state after delete", suKey)
	}
}

// TestAddEndpoints test adding endpoints to service units
func TestAddEndpoints(t *testing.T) {
	router := NewFakeTemplateRouter()
	suKey := "test"
	router.CreateServiceUnit(suKey)

	if _, ok := router.FindServiceUnit(suKey); !ok {
		t.Errorf("Unable to find serivce unit %s after creation", suKey)
	}

	endpoint := Endpoint{
		ID:     "ep1",
		IP:     "ip",
		Port:   "port",
		IdHash: fmt.Sprintf("%x", md5.Sum([]byte("ep1ipport"))),
	}

	router.AddEndpoints(suKey, []Endpoint{endpoint})

	su, ok := router.FindServiceUnit(suKey)

	if !ok {
		t.Errorf("Unable to find created service unit %s", suKey)
	} else {
		if len(su.EndpointTable) != 1 {
			t.Errorf("Expected endpoint table to contain 1 entry")
		} else {
			actualEp := su.EndpointTable[0]
			if endpoint.IP != actualEp.IP || endpoint.Port != actualEp.Port || endpoint.IdHash != actualEp.IdHash {
				t.Errorf("Expected endpoint %v did not match actual endpoint %v", endpoint, actualEp)
			}
		}
	}
}

// Test that AddEndpoints returns true and false correctly for changed endpoints.
func TestAddEndpointDuplicates(t *testing.T) {
	router := NewFakeTemplateRouter()
	suKey := "test"
	router.CreateServiceUnit(suKey)
	if _, ok := router.FindServiceUnit(suKey); !ok {
		t.Fatalf("Unable to find serivce unit %s after creation", suKey)
	}

	endpoint := Endpoint{
		ID:   "ep1",
		IP:   "1.1.1.1",
		Port: "80",
	}
	endpoint2 := Endpoint{
		ID:   "ep2",
		IP:   "2.2.2.2",
		Port: "8080",
	}
	endpoint3 := Endpoint{
		ID:   "ep3",
		IP:   "3.3.3.3",
		Port: "8888",
	}

	testCases := []struct {
		name      string
		endpoints []Endpoint
		expected  bool
	}{
		{
			name:      "initial add",
			endpoints: []Endpoint{endpoint, endpoint2},
			expected:  true,
		},
		{
			name:      "add same endpoints",
			endpoints: []Endpoint{endpoint, endpoint2},
			expected:  false,
		},
		{
			name:      "add changed endpoints",
			endpoints: []Endpoint{endpoint3, endpoint2},
			expected:  true,
		},
	}

	for _, v := range testCases {
		added := router.AddEndpoints(suKey, v.endpoints)
		if added != v.expected {
			t.Errorf("%s expected to return %v but got %v", v.name, v.expected, added)
		}
		su, ok := router.FindServiceUnit(suKey)
		if !ok {
			t.Errorf("%s was unable to find created service unit %s", v.name, suKey)
			continue
		}
		if len(su.EndpointTable) != len(v.endpoints) {
			t.Errorf("%s expected endpoint table to contain %d entries but found %v", v.name, len(v.endpoints), su.EndpointTable)
			continue
		}
		for i, ep := range su.EndpointTable {
			expected := v.endpoints[i]
			if expected.IP != ep.IP || expected.Port != ep.Port {
				t.Errorf("%s expected endpoint %v did not match actual endpoint %v", v.name, endpoint, ep)
			}
		}
	}
}

// TestDeleteEndpoints tests removing endpoints from service units
func TestDeleteEndpoints(t *testing.T) {
	router := NewFakeTemplateRouter()
	suKey := "test"
	router.CreateServiceUnit(suKey)

	if _, ok := router.FindServiceUnit(suKey); !ok {
		t.Errorf("Unable to find serivce unit %s after creation", suKey)
	}

	router.AddEndpoints(suKey, []Endpoint{
		{
			ID:   "ep1",
			IP:   "ip",
			Port: "port",
		},
	})

	su, ok := router.FindServiceUnit(suKey)

	if !ok {
		t.Errorf("Unable to find created service unit %s", suKey)
	} else {
		if len(su.EndpointTable) != 1 {
			t.Errorf("Expected endpoint table to contain 1 entry")
		} else {
			router.DeleteEndpoints(suKey)

			su, ok := router.FindServiceUnit(suKey)

			if !ok {
				t.Errorf("Unable to find created service unit %s", suKey)
			} else {
				if len(su.EndpointTable) > 0 {
					t.Errorf("Expected endpoint table to be empty")
				}
			}
		}
	}
}

// TestRouteKey tests that route keys are created as expected
func TestRouteKey(t *testing.T) {
	router := NewFakeTemplateRouter()
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}

	key := router.routeKey(route)

	if key != "foo_bar" {
		t.Errorf("Expected key 'foo_bar' but got: %s", key)
	}

	testCases := []struct {
		Namespace string
		Name      string
	}{
		{
			Namespace: "foo-bar",
			Name:      "baz",
		},
		{
			Namespace: "foo",
			Name:      "bar-baz",
		},
		{
			Namespace: "usain-bolt",
			Name:      "dash-dash",
		},
		{
			Namespace: "usain",
			Name:      "bolt-dash-dash",
		},
		{
			Namespace: "",
			Name:      "ab-testing",
		},
		{
			Namespace: "ab-testing",
			Name:      "",
		},
		{
			Namespace: "ab",
			Name:      "testing",
		},
	}

	suKey := "test"
	router.CreateServiceUnit(suKey)
	_, ok := router.FindServiceUnit(suKey)
	if !ok {
		t.Fatalf("Unable to find created service unit %s", suKey)
	}

	startCount := len(router.state)
	for _, tc := range testCases {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: tc.Namespace,
				Name:      tc.Name,
			},
			Spec: routeapi.RouteSpec{
				Host: "host",
				Path: "path",
				TLS: &routeapi.TLSConfig{
					Termination:              routeapi.TLSTerminationEdge,
					Certificate:              "abc",
					Key:                      "def",
					CACertificate:            "ghi",
					DestinationCACertificate: "jkl",
				},
			},
		}

		// add route always returns true
		added := router.AddRoute(suKey, 100, route, route.Spec.Host)
		if !added {
			t.Fatalf("expected AddRoute to return true but got false")
		}

		routeKey := router.routeKey(route)
		_, ok := router.state[routeKey]
		if !ok {
			t.Errorf("Unable to find created service alias config for route %s", routeKey)
		}
	}

	// ensure all the generated routes were added.
	numRoutesAdded := len(router.state) - startCount
	expectedCount := len(testCases)
	if numRoutesAdded != expectedCount {
		t.Errorf("Expected %v routes to be added but only %v were actually added", expectedCount, numRoutesAdded)
	}
}

// TestAddRoute tests adding a service alias config to a service unit
func TestAddRoute(t *testing.T) {
	router := NewFakeTemplateRouter()
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
		Spec: routeapi.RouteSpec{
			Host: "host",
			Path: "path",
			TLS: &routeapi.TLSConfig{
				Termination:              routeapi.TLSTerminationEdge,
				Certificate:              "abc",
				Key:                      "def",
				CACertificate:            "ghi",
				DestinationCACertificate: "jkl",
			},
		},
	}
	suKey := "test"
	router.CreateServiceUnit(suKey)

	// add route always returns true
	added := router.AddRoute(suKey, 100, route, route.Spec.Host)
	if !added {
		t.Fatalf("expected AddRoute to return true but got false")
	}

	_, ok := router.FindServiceUnit(suKey)

	if !ok {
		t.Errorf("Unable to find created service unit %s", suKey)
	} else {
		routeKey := router.routeKey(route)
		saCfg, ok := router.state[routeKey]

		if !ok {
			t.Errorf("Unable to find created service alias config for route %s", routeKey)
		} else {
			if saCfg.Host != route.Spec.Host || saCfg.Path != route.Spec.Path || !compareTLS(route, saCfg, t) {
				t.Errorf("Route %v did not match serivce alias config %v", route, saCfg)
			}
		}
	}
}

// compareTLS is a utility to help compare cert contents between an route and a config
func compareTLS(route *routeapi.Route, saCfg ServiceAliasConfig, t *testing.T) bool {
	return findCert(route.Spec.TLS.DestinationCACertificate, saCfg.Certificates, false, t) &&
		findCert(route.Spec.TLS.CACertificate, saCfg.Certificates, false, t) &&
		findCert(route.Spec.TLS.Key, saCfg.Certificates, true, t) &&
		findCert(route.Spec.TLS.Certificate, saCfg.Certificates, false, t)
}

// findCert is a utility to help find the cert in a config's set of certificates
func findCert(cert string, certs map[string]Certificate, isPrivateKey bool, t *testing.T) bool {
	found := false

	for _, c := range certs {
		if isPrivateKey {
			if c.PrivateKey == cert {
				found = true
				break
			}
		} else {
			if c.Contents == cert {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("unable to find cert %s in %v", cert, certs)
	}

	return found
}

// TestRemoveRoute tests removing a ServiceAliasConfig from a ServiceUnit
func TestRemoveRoute(t *testing.T) {
	router := NewFakeTemplateRouter()
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
		Spec: routeapi.RouteSpec{
			Host: "host",
		},
	}
	route2 := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "bar2",
		},
		Spec: routeapi.RouteSpec{
			Host: "host",
		},
	}
	suKey := "test"

	router.CreateServiceUnit(suKey)
	router.AddRoute(suKey, 100, route, route.Spec.Host)
	router.AddRoute(suKey, 100, route2, route2.Spec.Host)

	_, ok := router.FindServiceUnit(suKey)
	if !ok {
		t.Fatalf("Unable to find created service unit %s", suKey)
	}

	routeKey := router.routeKey(route)
	saCfg, ok := router.state[routeKey]
	if !ok {
		t.Fatalf("Unable to find created serivce alias config for route %s", routeKey)
	}
	if saCfg.Host != route.Spec.Host || saCfg.Path != route.Spec.Path {
		t.Fatalf("Route %v did not match serivce alias config %v", route, saCfg)
	}

	router.RemoveRoute(route)
	if _, ok := router.state[routeKey]; ok {
		t.Errorf("Route %v was expected to be deleted but was still found", route)
	}
	if _, ok := router.state[router.routeKey(route2)]; !ok {
		t.Errorf("Route %v was expected to exist but was not found", route2)
	}
}

func TestShouldWriteCertificates(t *testing.T) {
	testCases := []struct {
		name             string
		cfg              *ServiceAliasConfig
		shouldWriteCerts bool
	}{
		{
			name: "no termination",
			cfg: &ServiceAliasConfig{
				TLSTermination: "",
			},
			shouldWriteCerts: false,
		},
		{
			name: "passthrough termination",
			cfg: &ServiceAliasConfig{
				TLSTermination: routeapi.TLSTerminationPassthrough,
			},
			shouldWriteCerts: false,
		},
		{
			name: "edge termination true",
			cfg: &ServiceAliasConfig{
				Host:           "edgetermtrue",
				TLSTermination: routeapi.TLSTerminationEdge,
				Certificates:   makeCertMap("edgetermtrue", true),
			},
			shouldWriteCerts: true,
		},
		{
			name: "edge termination false",
			cfg: &ServiceAliasConfig{
				Host:           "edgetermfalse",
				TLSTermination: routeapi.TLSTerminationEdge,
				Certificates:   makeCertMap("edgetermfalse", false),
			},
			shouldWriteCerts: false,
		},
		{
			name: "reencrypt termination true",
			cfg: &ServiceAliasConfig{
				Host:           "reencrypttermtrue",
				TLSTermination: routeapi.TLSTerminationReencrypt,
				Certificates:   makeCertMap("reencrypttermtrue", true),
			},
			shouldWriteCerts: true,
		},
		{
			name: "reencrypt termination false",
			cfg: &ServiceAliasConfig{
				Host:           "reencrypttermfalse",
				TLSTermination: routeapi.TLSTerminationReencrypt,
				Certificates:   makeCertMap("reencrypttermfalse", false),
			},
			shouldWriteCerts: false,
		},
	}

	router := NewFakeTemplateRouter()
	for _, tc := range testCases {
		result := router.shouldWriteCerts(tc.cfg)
		if result != tc.shouldWriteCerts {
			t.Errorf("test case %s failed.  Expected shouldWriteCerts to return %t but found %t.  Cfg: %#v", tc.name, tc.shouldWriteCerts, result, tc.cfg)
		}
	}
}

func TestHasRequiredEdgeCerts(t *testing.T) {
	validCertMap := makeCertMap("host", true)
	cfg := &ServiceAliasConfig{
		Host:         "host",
		Certificates: validCertMap,
	}
	if !hasRequiredEdgeCerts(cfg) {
		t.Errorf("expected %#v to return true for valid edge certs", cfg)
	}

	invalidCertMap := makeCertMap("host", false)
	cfg.Certificates = invalidCertMap
	if hasRequiredEdgeCerts(cfg) {
		t.Errorf("expected %#v to return false for invalid edge certs", cfg)
	}
}

func makeCertMap(host string, valid bool) map[string]Certificate {
	privateKey := "private Key"
	if !valid {
		privateKey = ""
	}
	certMap := map[string]Certificate{
		host: {
			ID:         "host certificate",
			Contents:   "certificate",
			PrivateKey: privateKey,
		},
	}
	return certMap
}

// TestAddRouteEdgeTerminationInsecurePolicy tests adding an insecure edge
// terminated routes to a service unit
func TestAddRouteEdgeTerminationInsecurePolicy(t *testing.T) {
	router := NewFakeTemplateRouter()

	testCases := []struct {
		Name           string
		InsecurePolicy routeapi.InsecureEdgeTerminationPolicyType
	}{
		{
			Name:           "none",
			InsecurePolicy: routeapi.InsecureEdgeTerminationPolicyNone,
		},
		{
			Name:           "allow",
			InsecurePolicy: routeapi.InsecureEdgeTerminationPolicyAllow,
		},
		{
			Name:           "redirect",
			InsecurePolicy: routeapi.InsecureEdgeTerminationPolicyRedirect,
		},
		{
			Name:           "httpsec",
			InsecurePolicy: routeapi.InsecureEdgeTerminationPolicyType("httpsec"),
		},
		{
			Name:           "hsts",
			InsecurePolicy: routeapi.InsecureEdgeTerminationPolicyType("hsts"),
		},
	}

	for _, tc := range testCases {
		route := &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "foo",
				Name:      tc.Name,
			},
			Spec: routeapi.RouteSpec{
				Host: fmt.Sprintf("%s-host", tc.Name),
				Path: "path",
				TLS: &routeapi.TLSConfig{
					Termination:                   routeapi.TLSTerminationEdge,
					Certificate:                   "abc",
					Key:                           "def",
					CACertificate:                 "ghi",
					DestinationCACertificate:      "jkl",
					InsecureEdgeTerminationPolicy: tc.InsecurePolicy,
				},
			},
		}

		suKey := fmt.Sprintf("%s-test", tc.Name)
		router.CreateServiceUnit(suKey)

		// add route always returns true
		added := router.AddRoute(suKey, 100, route, route.Spec.Host)
		if !added {
			t.Fatalf("InsecureEdgeTerminationPolicy test %s: expected AddRoute to return true but got false", tc.Name)
		}

		_, ok := router.FindServiceUnit(suKey)

		if !ok {
			t.Errorf("InsecureEdgeTerminationPolicy test %s: unable to find created service unit %s",
				tc.Name, suKey)
		} else {
			routeKey := router.routeKey(route)
			saCfg, ok := router.state[routeKey]

			if !ok {
				t.Errorf("InsecureEdgeTerminationPolicy test %s: unable to find created service alias config for route %s",
					tc.Name, routeKey)
			} else {
				if saCfg.Host != route.Spec.Host || saCfg.Path != route.Spec.Path || !compareTLS(route, saCfg, t) || saCfg.InsecureEdgeTerminationPolicy != tc.InsecurePolicy {
					t.Errorf("InsecureEdgeTerminationPolicy test %s: route %v did not match serivce alias config %v",
						tc.Name, route, saCfg)
				}
			}
		}
	}
}
