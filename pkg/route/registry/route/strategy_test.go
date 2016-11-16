package route

import (
	"testing"

	"github.com/openshift/origin/pkg/route/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"
)

type testAllocator struct {
}

func (t testAllocator) AllocateRouterShard(*api.Route) (*api.RouterShard, error) {
	return &api.RouterShard{}, nil
}
func (t testAllocator) GenerateHostname(*api.Route, *api.RouterShard) string {
	return "mygeneratedhost.com"
}

func TestEmptyHostDefaulting(t *testing.T) {
	ctx := kapi.NewContext()
	strategy := NewStrategy(testAllocator{})

	hostlessCreatedRoute := &api.Route{}
	strategy.PrepareForCreate(ctx, hostlessCreatedRoute)
	if hostlessCreatedRoute.Spec.Host != "mygeneratedhost.com" {
		t.Fatalf("Expected host to be allocated, got %s", hostlessCreatedRoute.Spec.Host)
	}

	persistedRoute := &api.Route{
		ObjectMeta: kapi.ObjectMeta{
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
	ctx := kapi.NewContext()
	strategy := NewStrategy(testAllocator{})

	tests := []struct {
		name           string
		host           string
		wildcardPolicy api.WildcardPolicyType
		expected       string
	}{
		{
			name:     "nohostemptypolicy",
			expected: "mygeneratedhost.com",
		},
		{
			name:           "nohostnopolicy",
			wildcardPolicy: api.WildcardPolicyNone,
			expected:       "mygeneratedhost.com",
		},
		{
			name:           "nohostwildcardsubdomain",
			wildcardPolicy: api.WildcardPolicySubdomain,
			expected:       "",
		},
		{
			name:     "hostemptypolicy",
			host:     "empty.policy.test",
			expected: "empty.policy.test",
		},
		{
			name:           "hostnopolicy",
			host:           "no.policy.test",
			wildcardPolicy: api.WildcardPolicyNone,
			expected:       "no.policy.test",
		},
		{
			name:           "hostwildcardsubdomain",
			host:           "wildcard.policy.test",
			wildcardPolicy: api.WildcardPolicySubdomain,
			expected:       "wildcard.policy.test",
		},
	}

	for _, tc := range tests {
		route := &api.Route{
			ObjectMeta: kapi.ObjectMeta{
				Namespace:       "wildcard",
				Name:            tc.name,
				UID:             types.UID("wild"),
				ResourceVersion: "1",
			},
			Spec: api.RouteSpec{
				Host:           tc.host,
				WildcardPolicy: tc.wildcardPolicy,
			},
		}

		strategy.PrepareForCreate(ctx, route)
		if route.Spec.Host != tc.expected {
			t.Fatalf("test case %s expected host %s, got %s", tc.name, tc.expected, route.Spec.Host)
		}
	}
}
