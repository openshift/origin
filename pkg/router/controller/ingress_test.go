package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func TestIngressTranslator_TranslateIngressEvent(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())

	ingress := getTestIngress()

	testCases := map[string]struct {
		allowedNamespaces sets.String
		eventsExpected    bool
	}{
		"Events returned for ingress when allowed namespaces unset": {
			eventsExpected: true,
		},

		"Events returned for ingress in allowed namespace": {
			allowedNamespaces: sets.NewString(ingress.Namespace),
			eventsExpected:    true,
		},
		"Events not generated for ingress not in allowed namespace": {
			allowedNamespaces: sets.String{},
		},
	}
	for testName, tc := range testCases {
		it.allowedNamespaces = tc.allowedNamespaces
		events := it.TranslateIngressEvent(watch.Added, ingress)
		if tc.eventsExpected && len(events) == 0 {
			t.Fatalf("%v: Events should have been returned", testName)
		}
		if !tc.eventsExpected && len(events) > 0 {
			t.Fatalf("%v: Events should not have been returned", testName)
		}
	}
}

func TestIngressTranslator_TranslateSecretEvent(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())

	// Cache an ingress that will reference cached tls
	ingress := getTestIngress()
	ingressKey := getResourceKey(ingress.ObjectMeta)
	it.ingressMap[ingressKey] = ingress

	tls := &cachedTLS{
		cert:       "my-cert",
		privateKey: "my-private-key",
	}

	secretName := "my-secret"
	secretKey := getKey(ingress.Namespace, secretName)
	secret := getTestSecret(ingress.Namespace, secretName, tls.cert, tls.privateKey)

	testCases := map[string]struct {
		eventType     watch.EventType
		referencedTLS *referencedTLS
		eventCount    int
		tlsDeleted    bool
	}{
		"Secret not in the cache returns no events": {
			eventType: watch.Added,
		},
		"Unchanged tls returns no events": {
			eventType: watch.Added,
			referencedTLS: &referencedTLS{
				ingressKeys: sets.String{},
				tls:         tls,
			},
		},
		"Changed secret returns events for affected ingresses": {
			eventType: watch.Added,
			referencedTLS: &referencedTLS{
				ingressKeys: sets.NewString(ingressKey),
				tls:         tls,
			},
			eventCount: 1,
		},
		"Deleted secret removes tls from cache entry and returns events for affected ingresses": {
			eventType: watch.Deleted,
			referencedTLS: &referencedTLS{
				ingressKeys: sets.NewString(ingressKey),
				tls:         tls,
			},
			eventCount: 1,
			tlsDeleted: true,
		},
	}
	for testName, tc := range testCases {
		it.tlsMap[secretKey] = tc.referencedTLS
		events := it.TranslateSecretEvent(tc.eventType, secret)
		eventCount := len(events)
		if tc.eventCount != eventCount {
			t.Fatalf("%v: expected %d events, got %v", testName, tc.eventCount, eventCount)
		}
		if tc.tlsDeleted && tc.referencedTLS.tls != nil {
			t.Fatalf("%v: expected cached tls to be removed", testName)
		}
	}
}

func TestIngressTranslator_UpdateNamespaces(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())

	ingress := getTestIngress()
	ingressKey := getResourceKey(ingress.ObjectMeta)
	it.ingressMap[ingressKey] = ingress

	disallowed := getTestIngress()
	disallowed.Namespace = "not-allowed"
	disallowedKey := getResourceKey(disallowed.ObjectMeta)
	it.ingressMap[disallowedKey] = disallowed

	namespaces := sets.NewString(ingress.Namespace)
	it.UpdateNamespaces(namespaces)

	if !namespaces.Equal(it.allowedNamespaces) {
		t.Fatal("Allowed namespaces not set")
	}
	if it.ingressMap[ingressKey] == nil {
		t.Fatal("Allowed ingress removed")
	}
	if it.ingressMap[disallowedKey] != nil {
		t.Fatal("Disallowed ingress not removed")
	}
}

func TestIngressTranslator_unsafeTranslateIngressEvent(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())
	ingress := getTestIngress()
	events := it.TranslateIngressEvent(watch.Added, ingress)

	if len(events) != 1 {
		t.Fatal("expected route events for one ingress to be generated")
	}

	ingressRouteEvents := events[0]
	if ingressRouteEvents.ingressKey != getResourceKey(ingress.ObjectMeta) {
		t.Fatal("expected the ingress key to be set")
	}

	routeEvents := ingressRouteEvents.routeEvents
	if len(routeEvents) != 1 {
		t.Fatal("expected a single route event to have been generated")
	}
}

func TestIngressTranslator_handleIngressEvents(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())
	ingress := getTestIngress()
	ingressKey := getResourceKey(ingress.ObjectMeta)
	secretName := "my-secret"
	ingress.Spec.TLS = []extensions.IngressTLS{
		{
			SecretName: secretName,
		},
	}
	secretKey := getKey(ingress.Namespace, secretName)

	testCases := map[string]struct {
		eventType     watch.EventType
		ingress       *extensions.Ingress
		oldIngress    *extensions.Ingress
		cachedIngress bool
		cachedSecret  bool
	}{
		"Should be safe to attempt deletion of an ingress that isn't in the cache": {
			eventType: watch.Deleted,
			ingress:   ingress,
		},
		"Ingress addition should cache the ingress and referenced tls": {
			eventType:     watch.Added,
			ingress:       ingress,
			cachedIngress: true,
			cachedSecret:  true,
		},
		// This test depends on the addition test to prime the cache
		"Ingress deletion should remove ingress and tls from the cache": {
			eventType:  watch.Deleted,
			ingress:    ingress,
			oldIngress: ingress,
		},
	}
	for testName, tc := range testCases {
		it.handleIngressEvent(tc.eventType, tc.ingress, tc.oldIngress)
		if tc.cachedIngress {
			if it.ingressMap[ingressKey] != tc.ingress {
				t.Fatalf("%v: ingress not cached", testName)
			}
		} else if it.ingressMap[ingressKey] != nil {
			t.Fatalf("%v: ingress not removed from the cache", testName)
		}
		if tc.cachedSecret {
			if it.tlsMap[secretKey] == nil {
				t.Fatalf("%v: tls not cached", testName)
			}
		} else if it.tlsMap[secretKey] != nil {
			t.Fatalf("%v: tls not removed from the cache", testName)
		}
	}
}

func TestIngressTranslator_generateRouteEvents(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())

	ingress := getTestIngress()

	// Create an ingress with 2 rules to support validating that rule
	// removal results in a deletion event.
	twoRuleIngress := getTestIngress()
	twoRuleIngress.Spec.Rules = append(ingress.Spec.Rules, extensions.IngressRule{
		Host: "my.host",
		IngressRuleValue: extensions.IngressRuleValue{
			HTTP: &extensions.HTTPIngressRuleValue{
				Paths: []extensions.HTTPIngressPath{
					{
						Path: "/my-path2",
						Backend: extensions.IngressBackend{
							ServiceName: "my-service",
							ServicePort: intstr.FromString("80"),
						},
					},
				},
			},
		},
	})

	// Add tls to the cache to enable validation of tls addition to generated routes
	hostName := "my.host"
	secretName := "my-secret"
	secretKey := getKey(ingress.Namespace, secretName)
	cert := "my-cert"
	it.tlsMap[secretKey] = &referencedTLS{
		ingressKeys: sets.String{},
		tls: &cachedTLS{
			cert:       cert,
			privateKey: "my-key",
		},
	}

	// Add tls to the ingresses
	tls := []extensions.IngressTLS{
		{
			Hosts:      []string{hostName},
			SecretName: secretName,
		},
	}
	ingress.Spec.TLS = tls
	twoRuleIngress.Spec.TLS = tls

	type expectedEvent struct {
		eventType watch.EventType
		// Path is used as a shallow check that the expected route is
		// generated.  More comprehensive validation of route
		// generation is the responsibility of testing for
		// ingressToRoutes.
		path string
	}

	testCases := map[string]struct {
		eventType  watch.EventType
		ingress    *extensions.Ingress
		oldIngress *extensions.Ingress
		expected   []expectedEvent
	}{
		"Should generate a route addition event for each of the 2 rules": {
			eventType: watch.Added,
			ingress:   twoRuleIngress,
			expected: []expectedEvent{
				{
					eventType: watch.Added,
					path:      twoRuleIngress.Spec.Rules[0].HTTP.Paths[0].Path,
				},
				{
					eventType: watch.Added,
					path:      twoRuleIngress.Spec.Rules[1].HTTP.Paths[0].Path,
				},
			},
		},
		"Should generate a route addition and a route deletion for the missing rule": {
			eventType:  watch.Modified,
			ingress:    ingress,
			oldIngress: twoRuleIngress,
			expected: []expectedEvent{
				{
					eventType: watch.Deleted,
					path:      twoRuleIngress.Spec.Rules[1].HTTP.Paths[0].Path,
				},
				{
					eventType: watch.Modified,
					path:      ingress.Spec.Rules[0].HTTP.Paths[0].Path,
				},
			},
		},
		"Should generate a route deletion": {
			eventType:  watch.Deleted,
			ingress:    ingress,
			oldIngress: ingress,
			expected: []expectedEvent{
				{
					eventType: watch.Deleted,
					path:      ingress.Spec.Rules[0].HTTP.Paths[0].Path,
				},
			},
		},
		"Rule-less ingress should generate no events": {
			eventType: watch.Added,
			ingress: &extensions.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-ingress",
					Namespace: "my-namespace",
				},
			},
		},
	}
	for testName, tc := range testCases {
		events := it.generateRouteEvents(tc.eventType, tc.ingress, tc.oldIngress)
		expectedCount := len(tc.expected)
		if expectedCount != len(events) {
			t.Fatalf("%v: Expected %d route events to be generated", testName, expectedCount)
		}
		for i, expected := range tc.expected {
			if events[i].eventType != expected.eventType {
				t.Fatalf("%v: Expected event of type %v, got %v", testName, expected.eventType, events[i].eventType)
			}
			if events[i].route.Spec.Path != expected.path {
				t.Fatalf("%v: Expected path to be %v, got %v", testName, expected.path, events[i].route.Spec.Path)
			}
			// Cert is used for a shallow check of tls config being set.  More comprehensive
			// validation is the responsibility of testing for addRouteTLS.
			if expected.eventType != watch.Deleted && events[i].route.Spec.TLS.Certificate != cert {
				t.Fatalf("%v: TLS not applied to route", testName)
			}
		}
	}
}

func TestIngressTranslator_cacheTLS(t *testing.T) {
	secretName := "my-secret"
	namespace := "my-namespace"

	cert := "my-cert"
	privateKey := "my-key"

	// Ensure a secret can be retrieved via the client
	secret := getTestSecret(namespace, secretName, cert, privateKey)
	objects := []runtime.Object{secret}
	client := fake.NewSimpleClientset(objects...)

	testCases := map[string]struct {
		secretName    string
		alreadyCached bool
		success       bool
	}{
		"Should create cache entry even if secret cannot be read": {
			secretName: "unknown-secret",
		},
		"Should cache tls if secret read successfully": {
			secretName: secretName,
			success:    true,
		},
		"Should retrieve cached tls if available": {
			secretName:    "already-cached",
			alreadyCached: true,
			success:       true,
		},
	}
	for testName, tc := range testCases {
		it := NewIngressTranslator(client.Core())
		ingressKey := getKey(namespace, "my-ingress")

		secretKey := getKey(namespace, tc.secretName)

		if tc.alreadyCached {
			it.tlsMap[secretKey] = &referencedTLS{
				ingressKeys: sets.String{},
				tls:         &cachedTLS{},
			}
		}

		ingressTLS := []extensions.IngressTLS{
			{
				SecretName: tc.secretName,
			},
		}
		success := it.cacheTLS(ingressTLS, namespace, ingressKey)

		if success != tc.success {
			t.Fatalf("%v: Expected success to be %v, got %v", testName, tc.success, success)
		}

		if refTLS := it.tlsMap[secretKey]; refTLS == nil {
			t.Fatalf("%v: TLS entry is missing", testName)
		} else {
			if !refTLS.ingressKeys.Has(ingressKey) {
				t.Fatalf("%v: Ingress key not referenced by the TLS cache entry", testName)
			}
			if success && refTLS.tls == nil {
				t.Fatalf("%v: TLS data missing", testName)
			}
		}
	}
}

func TestIngressTranslator_dereferenceTLS(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())

	namespace := "my-namespace"
	ingress1Key := getKey(namespace, "my-ingress1")
	ingress2Key := getKey(namespace, "my-ingress2")
	secretName := "my-secret"
	secretKey := getKey(namespace, secretName)

	// Add cache entry
	refTLS := &referencedTLS{
		ingressKeys: sets.NewString(ingress1Key, ingress2Key),
	}
	it.tlsMap[secretKey] = refTLS

	ingressTLS := []extensions.IngressTLS{
		{
			SecretName: secretName,
		},
	}

	// Cache entry should exist but only be referenced by the second ingress
	it.dereferenceTLS(ingressTLS, namespace, ingress1Key)
	expectedKeys := sets.NewString(ingress2Key)
	if _, ok := it.tlsMap[secretKey]; !ok {
		t.Fatalf("TLS cache entry removed while still referenced")
	} else if !refTLS.ingressKeys.Equal(expectedKeys) {
		t.Fatalf("Expected cache entry to have ingress keys of %v but got %v", expectedKeys, refTLS.ingressKeys)
	}

	// Cache entry should be removed because it is no longer referenced
	it.dereferenceTLS(ingressTLS, namespace, ingress2Key)
	if _, ok := it.tlsMap[secretKey]; ok {
		t.Fatalf("TLS cache entry not removed")
	}
}

func TestIngressTranslator_tlsForHost(t *testing.T) {
	client := fake.NewSimpleClientset()
	it := NewIngressTranslator(client.Core())

	namespace := "my-namespace"
	secretName1 := "my-secret"
	secretKey1 := getKey(namespace, secretName1)
	secretName2 := "other-secret"
	secretKey2 := getKey(namespace, secretName2)

	firstCert := "my-cert"

	// Cache the tls
	it.tlsMap[secretKey1] = &referencedTLS{
		ingressKeys: sets.String{},
		tls: &cachedTLS{
			cert:       firstCert,
			privateKey: "my-private-key",
		},
	}
	it.tlsMap[secretKey2] = &referencedTLS{
		ingressKeys: sets.String{},
		tls: &cachedTLS{
			cert:       "other-cert",
			privateKey: "my-private-key",
		},
	}

	defaultHost := "foo"

	testCases := map[string]struct {
		tlsHost string
		host    string
		success bool
	}{
		"Unmatched host returns no tls": {
			tlsHost: defaultHost,
			host:    "bar",
		},
		"Matched host returns tls": {
			tlsHost: defaultHost,
			host:    defaultHost,
			success: true,
		},
		"Empty host matches tls with empty host": {
			tlsHost: "",
			host:    "",
			success: true,
		},
		"Empty host without tls with empty host returns no tls": {
			tlsHost: defaultHost,
			host:    "",
		},
	}
	for testName, tc := range testCases {
		// Create tls items that match on the same host to validate
		// that the first matching tls wins.
		ingressTLS := []extensions.IngressTLS{
			{
				Hosts:      []string{tc.tlsHost},
				SecretName: secretName1,
			},
			{
				Hosts:      []string{tc.tlsHost},
				SecretName: secretName2,
			},
		}
		tls := it.tlsForHost(ingressTLS, namespace, tc.host)
		// Validate that only the first secret is ever returned
		if tc.success && (tls == nil || tls.cert != firstCert) {
			t.Fatalf("%v: tls not returned as expected", testName)
		}
		if !tc.success && tls != nil {
			t.Fatalf("%v: tls unexpectedly returned", testName)
		}
	}
}

func TestTLSFromSecret(t *testing.T) {
	testCases := map[string]struct {
		cert       string
		privateKey string
		error      bool
	}{
		"Invalid cert and private key should result in an error": {
			error: true,
		},
		"Valid cert and private key should return a tls object": {
			cert:       "my-cert",
			privateKey: "my-key",
		},
	}
	for testName, tc := range testCases {
		secret := getTestSecret("my-namespace", "my-secret", tc.cert, tc.privateKey)
		tls, err := tlsFromSecret(secret)
		if tc.error && err == nil {
			t.Fatalf("%v: Error not returned", testName)
		}
		if tls == nil {
			if !tc.error {
				t.Fatalf("%v: tls not returned", testName)
			}
		} else {
			if tls.cert != tc.cert {
				t.Fatalf("%v: expected tls cert to be %v, got %v", testName, tc.cert, tls.cert)
			}
			if tls.privateKey != tc.privateKey {
				t.Fatalf("%v: expected tls private key to be %v, got %v", testName, tc.privateKey, tls.privateKey)
			}
		}
	}
}

func TestIngressToRoutes(t *testing.T) {
	noRuleValueIngress := getTestIngress()
	noRuleValueIngress.Spec.Rules[0].HTTP = nil

	testCases := map[string]struct {
		ingress       *extensions.Ingress
		expectedCount int
	}{
		"Ingress without rule value generates no routes": {
			ingress: noRuleValueIngress,
		},
		"Ingress with one path generates one route": {
			ingress:       getTestIngress(),
			expectedCount: 1,
		},
	}
	for testName, tc := range testCases {
		routes, routeNames := ingressToRoutes(tc.ingress)
		routeCount := len(routes)
		if routeCount != tc.expectedCount {
			t.Fatalf("%v: Expected %v route(s) to be generated from ingress, got %v", testName, tc.expectedCount, routeCount)
		}
		nameCount := len(routeNames)
		if nameCount != tc.expectedCount {
			t.Fatalf("%v: Expected %v route(s) to be generated from ingress, got %v", testName, tc.expectedCount, nameCount)
		}
	}
}

func TestMatchesHost(t *testing.T) {
	testCases := map[string]struct {
		pattern string
		host    string
		matches bool
	}{
		"zero length pattern or host does not match": {},
		"differing number of path components does not match": {
			pattern: "foo.bar",
			host:    "bar",
		},
		"wildcard pattern with differing path components does not match": {
			pattern: "*.foo.bar",
			host:    "foo.baz",
		},
		"single component pattern and host match": {
			pattern: "foo",
			host:    "foo",
			matches: true,
		},
		"multiple component pattern and host match": {
			pattern: "foo.bar",
			host:    "foo.bar",
			matches: true,
		},
		"multiple component wildcard pattern and host match": {
			pattern: "*.bar.baz",
			host:    "foo.bar.baz",
			matches: true,
		},
	}
	for testName, tc := range testCases {
		matches := matchesHost(tc.pattern, tc.host)
		if matches != tc.matches {
			t.Fatalf("%v: expected match to be %v, got %v", testName, tc.matches, matches)
		}
	}
}

func TestGetNameForHost(t *testing.T) {
	host := "foo"
	testCases := map[string]struct {
		name        string
		nameForHost string
	}{
		"Route name is returned": {
			name:        host,
			nameForHost: host,
		},
		"Ingress name is returned": {
			name:        generateRouteName(host, "", ""),
			nameForHost: host,
		},
	}
	for testName, tc := range testCases {
		nameForHost := GetNameForHost(tc.name)
		if nameForHost != tc.nameForHost {
			t.Fatalf("%v: expected %v, got %v", testName, tc.nameForHost, nameForHost)
		}
	}
}

func getTestIngress() *extensions.Ingress {
	return &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: "my-namespace",
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: "my.host",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: "/my-path",
									Backend: extensions.IngressBackend{
										ServiceName: "my-service",
										ServicePort: intstr.FromString("80"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getTestSecret(namespace, name, cert, privateKey string) *kapi.Secret {
	return &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte(cert),
			"tls.key": []byte(privateKey),
		},
		Type: kapi.SecretTypeOpaque,
	}
}
