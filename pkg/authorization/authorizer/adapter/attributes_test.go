package adapter

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/util/sets"

	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
)

// ensure we satisfy both interfaces
var _ = oauthorizer.AuthorizationAttributes(AdapterAttributes{})
var _ = kauthorizer.Attributes(AdapterAttributes{})

func TestRoundTrip(t *testing.T) {
	// Start with origin attributes
	oattrs := oauthorizer.DefaultAuthorizationAttributes{
		Verb:              "get",
		APIVersion:        "av",
		APIGroup:          "ag",
		Resource:          "r",
		ResourceName:      "rn",
		RequestAttributes: "ra",
		NonResourceURL:    true,
		URL:               "/123",
	}

	// Convert to kube attributes
	kattrs := KubernetesAuthorizerAttributes("ns", "myuser", []string{"mygroup"}, oattrs)
	if kattrs.GetUserName() != "myuser" {
		t.Errorf("Expected %v, got %v", "myuser", kattrs.GetUserName())
	}
	if !reflect.DeepEqual(kattrs.GetGroups(), []string{"mygroup"}) {
		t.Errorf("Expected %v, got %v", []string{"mygroup"}, kattrs.GetGroups())
	}
	if kattrs.GetVerb() != "get" {
		t.Errorf("Expected %v, got %v", "get", kattrs.GetVerb())
	}
	if kattrs.IsReadOnly() != true {
		t.Errorf("Expected %v, got %v", true, kattrs.IsReadOnly())
	}
	if kattrs.GetNamespace() != "ns" {
		t.Errorf("Expected %v, got %v", "ns", kattrs.GetNamespace())
	}
	if kattrs.GetResource() != "r" {
		t.Errorf("Expected %v, got %v", "", kattrs.GetResource())
	}

	// Convert back to context+origin attributes
	ctx, oattrs2 := OriginAuthorizerAttributes(kattrs)

	// Ensure namespace/user info is preserved
	if user, ok := kapi.UserFrom(ctx); !ok {
		t.Errorf("No user in context")
	} else if user.GetName() != "myuser" {
		t.Errorf("Expected %v, got %v", "myuser", user.GetName())
	} else if !reflect.DeepEqual(user.GetGroups(), []string{"mygroup"}) {
		t.Errorf("Expected %v, got %v", []string{"mygroup"}, user.GetGroups())
	}

	// Ensure common attribute info is preserved
	if oattrs.GetVerb() != oattrs2.GetVerb() {
		t.Errorf("Expected %v, got %v", oattrs.GetVerb(), oattrs2.GetVerb())
	}
	if oattrs.GetResource() != oattrs2.GetResource() {
		t.Errorf("Expected %v, got %v", oattrs.GetResource(), oattrs2.GetResource())
	}

	// Ensure origin-specific info is preserved
	if oattrs.GetAPIVersion() != oattrs2.GetAPIVersion() {
		t.Errorf("Expected %v, got %v", oattrs.GetAPIVersion(), oattrs2.GetAPIVersion())
	}
	if oattrs.GetAPIGroup() != oattrs2.GetAPIGroup() {
		t.Errorf("Expected %v, got %v", oattrs.GetAPIGroup(), oattrs2.GetAPIGroup())
	}
	if oattrs.GetResourceName() != oattrs2.GetResourceName() {
		t.Errorf("Expected %v, got %v", oattrs.GetResourceName(), oattrs2.GetResourceName())
	}
	if oattrs.GetRequestAttributes() != oattrs2.GetRequestAttributes() {
		t.Errorf("Expected %v, got %v", oattrs.GetRequestAttributes(), oattrs2.GetRequestAttributes())
	}
	if oattrs.IsNonResourceURL() != oattrs2.IsNonResourceURL() {
		t.Errorf("Expected %v, got %v", oattrs.IsNonResourceURL(), oattrs2.IsNonResourceURL())
	}
	if oattrs.GetURL() != oattrs2.GetURL() {
		t.Errorf("Expected %v, got %v", oattrs.GetURL(), oattrs2.GetURL())
	}
}

func TestAttributeIntersection(t *testing.T) {
	// These are the things we expect to be shared
	// Everything in this list should be used by OriginAuthorizerAttributes
	expectedIntersection := sets.NewString("GetVerb", "GetResource")

	// These are the things we expect to only be in the Kubernetes interface
	// Everything in this list should be used by OriginAuthorizerAttributes or derivative (like IsReadOnly)
	expectedKubernetesOnly := sets.NewString(
		// used to build context in OriginAuthorizerAttributes
		"GetGroups", "GetUserName", "GetNamespace",
		// Based on verb, derivative
		"IsReadOnly",
	)

	kattributesType := reflect.TypeOf((*kauthorizer.Attributes)(nil)).Elem()
	oattributesType := reflect.TypeOf((*oauthorizer.AuthorizationAttributes)(nil)).Elem()

	kattributesMethods := sets.NewString()
	for i := 0; i < kattributesType.NumMethod(); i++ {
		kattributesMethods.Insert(kattributesType.Method(i).Name)
	}

	oattributesMethods := sets.NewString()
	for i := 0; i < oattributesType.NumMethod(); i++ {
		oattributesMethods.Insert(oattributesType.Method(i).Name)
	}

	// Make sure all shared functions are used
	intersection := oattributesMethods.Intersection(kattributesMethods)
	if !intersection.HasAll(expectedIntersection.List()...) {
		t.Errorf(
			"Kubernetes authorizer.Attributes interface changed. Missing expected methods: %v",
			expectedIntersection.Difference(intersection).List(),
		)
	}
	if !expectedIntersection.HasAll(intersection.List()...) {
		t.Errorf(
			"Kubernetes authorizer.Attributes interface changed. Update OriginAuthorizerAttributes to use data from additional shared methods: %v",
			intersection.Difference(expectedIntersection).List(),
		)
	}

	// Make sure all unshared functions are expected
	kattributesOnlyMethods := kattributesMethods.Difference(oattributesMethods)
	if !expectedKubernetesOnly.IsSuperset(kattributesOnlyMethods) {
		t.Errorf(
			"Kubernetes authorizer.Attributes interface changed. Check if new functions should be used in OriginAuthorizerAttributes: %v",
			kattributesOnlyMethods.Difference(expectedKubernetesOnly).List(),
		)
	}
}
