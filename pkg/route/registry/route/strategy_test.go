package route

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

type testAllocator struct {
}

func (t testAllocator) AllocateRouterShard(*routeapi.Route) (*routeapi.RouterShard, error) {
	return &routeapi.RouterShard{}, nil
}
func (t testAllocator) GenerateHostname(*routeapi.Route, *routeapi.RouterShard) string {
	return "mygeneratedhost.com"
}

type testSAR struct {
	allow bool
	err   error
	sar   *authorizationapi.SubjectAccessReview
}

func (t *testSAR) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReview, error) {
	t.sar = subjectAccessReview
	return &authorizationapi.SubjectAccessReview{
		Status: authorizationapi.SubjectAccessReviewStatus{
			Allowed: t.allow,
		},
	}, t.err
}

func TestEmptyHostDefaulting(t *testing.T) {
	ctx := apirequest.NewContext()
	strategy := NewStrategy(testAllocator{}, &testSAR{allow: true})

	hostlessCreatedRoute := &routeapi.Route{}
	strategy.Validate(ctx, hostlessCreatedRoute)
	if hostlessCreatedRoute.Spec.Host != "mygeneratedhost.com" {
		t.Fatalf("Expected host to be allocated, got %s", hostlessCreatedRoute.Spec.Host)
	}

	persistedRoute := &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "foo",
			Name:            "myroute",
			UID:             types.UID("abc"),
			ResourceVersion: "1",
		},
		Spec: routeapi.RouteSpec{
			Host: "myhost.com",
		},
	}
	hostlessUpdatedRoute := persistedRoute.DeepCopy()
	hostlessUpdatedRoute.Spec.Host = ""
	strategy.PrepareForUpdate(ctx, hostlessUpdatedRoute, persistedRoute)
	if hostlessUpdatedRoute.Spec.Host != "myhost.com" {
		t.Fatalf("expected empty spec.host to default to existing spec.host, got %s", hostlessUpdatedRoute.Spec.Host)
	}
}

func TestEmptyDefaultCACertificate(t *testing.T) {
	testCases := []struct {
		route *routeapi.Route
	}{
		{
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       "foo",
					Name:            "myroute",
					UID:             types.UID("abc"),
					ResourceVersion: "1",
				},
				Spec: routeapi.RouteSpec{
					Host: "myhost.com",
				},
			},
		},
	}
	for i, testCase := range testCases {
		copied := testCase.route.DeepCopy()
		if err := DecorateLegacyRouteWithEmptyDestinationCACertificates(copied); err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
			continue
		}
		routeStrategy{}.PrepareForCreate(nil, copied)
		if !reflect.DeepEqual(testCase.route, copied) {
			t.Errorf("%d: unexpected change: %#v", i, copied)
			continue
		}
		if err := DecorateLegacyRouteWithEmptyDestinationCACertificates(copied); err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
			continue
		}
		routeStrategy{}.PrepareForUpdate(nil, copied, &routeapi.Route{})
		if !reflect.DeepEqual(testCase.route, copied) {
			t.Errorf("%d: unexpected change: %#v", i, copied)
			continue
		}
	}
}

func TestHostWithWildcardPolicies(t *testing.T) {
	ctx := apirequest.NewContext()
	ctx = apirequest.WithUser(ctx, &user.DefaultInfo{Name: "bob"})

	tests := []struct {
		name           string
		host, oldHost  string
		wildcardPolicy routeapi.WildcardPolicyType
		tls, oldTLS    *routeapi.TLSConfig
		expected       string
		errs           int
		allow          bool
	}{
		{
			name:     "no-host-empty-policy",
			expected: "mygeneratedhost.com",
			allow:    true,
		},
		{
			name:           "no-host-nopolicy",
			wildcardPolicy: routeapi.WildcardPolicyNone,
			expected:       "mygeneratedhost.com",
			allow:          true,
		},
		{
			name:           "no-host-wildcard-subdomain",
			wildcardPolicy: routeapi.WildcardPolicySubdomain,
			expected:       "",
			allow:          true,
			errs:           1,
		},
		{
			name:     "host-empty-policy",
			host:     "empty.policy.test",
			expected: "empty.policy.test",
			allow:    true,
		},
		{
			name:           "host-no-policy",
			host:           "no.policy.test",
			wildcardPolicy: routeapi.WildcardPolicyNone,
			expected:       "no.policy.test",
			allow:          true,
		},
		{
			name:           "host-wildcard-subdomain",
			host:           "wildcard.policy.test",
			wildcardPolicy: routeapi.WildcardPolicySubdomain,
			expected:       "wildcard.policy.test",
			allow:          true,
		},
		{
			name:           "custom-host-permission-denied",
			host:           "another.test",
			expected:       "another.test",
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-destination",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-cert",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Certificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-ca-cert",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, CACertificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-key",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "no-host-but-allowed",
			expected:       "mygeneratedhost.com",
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
		},
		{
			name:           "update-changed-host-denied",
			host:           "new.host",
			expected:       "new.host",
			oldHost:        "original.host",
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "update-changed-host-allowed",
			host:           "new.host",
			expected:       "new.host",
			oldHost:        "original.host",
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          true,
			errs:           0,
		},
		{
			name:           "key-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "key-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "b"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "certificate-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Certificate: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Certificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "certificate-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Certificate: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Certificate: "b"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "ca-certificate-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, CACertificate: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, CACertificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "ca-certificate-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, CACertificate: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, CACertificate: "b"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "key-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "key-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge, Key: "b"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "destination-ca-certificate-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "destination-ca-certificate-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, DestinationCACertificate: "b"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "set-to-edge-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge},
			oldTLS:         nil,
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "cleared-edge",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            nil,
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationEdge},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "removed-certificate",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt},
			oldTLS:         &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, Certificate: "a"},
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "added-certificate-and-fails",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &routeapi.TLSConfig{Termination: routeapi.TLSTerminationReencrypt, Certificate: "a"},
			oldTLS:         nil,
			wildcardPolicy: routeapi.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
	}

	for _, tc := range tests {
		sar := &testSAR{allow: tc.allow}
		strategy := NewStrategy(testAllocator{}, sar)

		route := &routeapi.Route{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "wildcard",
				Name:            tc.name,
				UID:             types.UID("wild"),
				ResourceVersion: "1",
			},
			Spec: routeapi.RouteSpec{
				Host:           tc.host,
				WildcardPolicy: tc.wildcardPolicy,
				TLS:            tc.tls,
				To: routeapi.RouteTargetReference{
					Name: "test",
					Kind: "Service",
				},
			},
		}

		var errs field.ErrorList
		if len(tc.oldHost) > 0 {
			oldRoute := &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       "wildcard",
					Name:            tc.name,
					UID:             types.UID("wild"),
					ResourceVersion: "1",
				},
				Spec: routeapi.RouteSpec{
					Host:           tc.oldHost,
					WildcardPolicy: tc.wildcardPolicy,
					TLS:            tc.oldTLS,
					To: routeapi.RouteTargetReference{
						Name: "test",
						Kind: "Service",
					},
				},
			}
			errs = strategy.ValidateUpdate(ctx, route, oldRoute)
		} else {
			errs = strategy.Validate(ctx, route)
		}

		if route.Spec.Host != tc.expected {
			t.Errorf("test case %s expected host %s, got %s", tc.name, tc.expected, route.Spec.Host)
			continue
		}
		if len(errs) != tc.errs {
			t.Errorf("test case %s unexpected errors: %v %#v", tc.name, errs, sar)
		}
	}
}
