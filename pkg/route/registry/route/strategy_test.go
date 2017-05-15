package route

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/route/api"
)

type testAllocator struct {
}

func (t testAllocator) AllocateRouterShard(*api.Route) (*api.RouterShard, error) {
	return &api.RouterShard{}, nil
}
func (t testAllocator) GenerateHostname(*api.Route, *api.RouterShard) string {
	return "mygeneratedhost.com"
}

type testSAR struct {
	allow bool
	err   error
	sar   *authorizationapi.SubjectAccessReview
}

func (t *testSAR) CreateSubjectAccessReview(ctx apirequest.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	t.sar = subjectAccessReview
	return &authorizationapi.SubjectAccessReviewResponse{Allowed: t.allow}, t.err
}

func TestEmptyHostDefaulting(t *testing.T) {
	ctx := apirequest.NewContext()
	strategy := NewStrategy(testAllocator{}, &testSAR{allow: true})

	hostlessCreatedRoute := &api.Route{}
	strategy.Validate(ctx, hostlessCreatedRoute)
	if hostlessCreatedRoute.Spec.Host != "mygeneratedhost.com" {
		t.Fatalf("Expected host to be allocated, got %s", hostlessCreatedRoute.Spec.Host)
	}

	persistedRoute := &api.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "foo",
			Name:            "myroute",
			UID:             types.UID("abc"),
			ResourceVersion: "1",
		},
		Spec: api.RouteSpec{
			Host: "myhost.com",
		},
	}
	obj, _ := kapi.Scheme.DeepCopy(persistedRoute)
	hostlessUpdatedRoute := obj.(*api.Route)
	hostlessUpdatedRoute.Spec.Host = ""
	strategy.PrepareForUpdate(ctx, hostlessUpdatedRoute, persistedRoute)
	if hostlessUpdatedRoute.Spec.Host != "myhost.com" {
		t.Fatalf("expected empty spec.host to default to existing spec.host, got %s", hostlessUpdatedRoute.Spec.Host)
	}
}

func TestHostWithWildcardPolicies(t *testing.T) {
	ctx := apirequest.NewContext()
	ctx = apirequest.WithUser(ctx, &user.DefaultInfo{Name: "bob"})

	tests := []struct {
		name           string
		host, oldHost  string
		wildcardPolicy api.WildcardPolicyType
		tls, oldTLS    *api.TLSConfig
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
			wildcardPolicy: api.WildcardPolicyNone,
			expected:       "mygeneratedhost.com",
			allow:          true,
		},
		{
			name:           "no-host-wildcard-subdomain",
			wildcardPolicy: api.WildcardPolicySubdomain,
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
			wildcardPolicy: api.WildcardPolicyNone,
			expected:       "no.policy.test",
			allow:          true,
		},
		{
			name:           "host-wildcard-subdomain",
			host:           "wildcard.policy.test",
			wildcardPolicy: api.WildcardPolicySubdomain,
			expected:       "wildcard.policy.test",
			allow:          true,
		},
		{
			name:           "custom-host-permission-denied",
			host:           "another.test",
			expected:       "another.test",
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-destination",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-cert",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Certificate: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-ca-cert",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, CACertificate: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "tls-permission-denied-key",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "no-host-but-allowed",
			expected:       "mygeneratedhost.com",
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
		},
		{
			name:           "update-changed-host-denied",
			host:           "new.host",
			expected:       "new.host",
			oldHost:        "original.host",
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "update-changed-host-allowed",
			host:           "new.host",
			expected:       "new.host",
			oldHost:        "original.host",
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          true,
			errs:           0,
		},
		{
			name:           "key-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "key-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "b"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "certificate-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Certificate: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, Certificate: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "certificate-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Certificate: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, Certificate: "b"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "ca-certificate-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, CACertificate: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, CACertificate: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "ca-certificate-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, CACertificate: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, CACertificate: "b"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "key-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "key-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationEdge, Key: "b"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
		{
			name:           "destination-ca-certificate-unchanged",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           0,
		},
		{
			name:           "destination-ca-certificate-changed",
			host:           "host",
			expected:       "host",
			oldHost:        "host",
			tls:            &api.TLSConfig{Termination: api.TLSTerminationReencrypt, DestinationCACertificate: "a"},
			oldTLS:         &api.TLSConfig{Termination: api.TLSTerminationReencrypt, DestinationCACertificate: "b"},
			wildcardPolicy: api.WildcardPolicyNone,
			allow:          false,
			errs:           1,
		},
	}

	for _, tc := range tests {
		sar := &testSAR{allow: tc.allow}
		strategy := NewStrategy(testAllocator{}, sar)

		route := &api.Route{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "wildcard",
				Name:            tc.name,
				UID:             types.UID("wild"),
				ResourceVersion: "1",
			},
			Spec: api.RouteSpec{
				Host:           tc.host,
				WildcardPolicy: tc.wildcardPolicy,
				TLS:            tc.tls,
				To: api.RouteTargetReference{
					Name: "test",
					Kind: "Service",
				},
			},
		}

		var errs field.ErrorList
		if len(tc.oldHost) > 0 {
			oldRoute := &api.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       "wildcard",
					Name:            tc.name,
					UID:             types.UID("wild"),
					ResourceVersion: "1",
				},
				Spec: api.RouteSpec{
					Host:           tc.oldHost,
					WildcardPolicy: tc.wildcardPolicy,
					TLS:            tc.oldTLS,
					To: api.RouteTargetReference{
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
