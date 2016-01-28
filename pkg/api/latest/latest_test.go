package latest

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	klatest "k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
)

func TestRESTRootScope(t *testing.T) {
	for _, v := range [][]string{{"v1beta3"}, {"v1"}} {
		mapping, err := RESTMapper.RESTMapping(kapi.Kind("Node"), v...)
		if err != nil {
			t.Fatal(err)
		}
		if mapping.Scope.Name() != meta.RESTScopeNameRoot {
			t.Errorf("Node should have a root scope: %#v", mapping.Scope)
		}
	}
}

func TestResourceToKind(t *testing.T) {
	// Ensure we resolve to latest.Version
	expectedGVK := Version.WithKind("User")
	gvk, err := RESTMapper.KindFor("user")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gvk != expectedGVK {
		t.Fatalf("Expected RESTMapper.KindFor('user') to be %#v, got %#v", expectedGVK, gvk)
	}
}

func TestUpstreamResourceToKind(t *testing.T) {
	// Ensure we resolve to klatest.ExternalVersions[0]
	expectedGVK := klatest.ExternalVersions[0].WithKind("Pod")
	gvk, err := registered.GroupOrDie(kapi.SchemeGroupVersion.Group).RESTMapper.KindFor("pod")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gvk != expectedGVK {
		t.Fatalf("Expected RESTMapper.KindFor('pod') to be %#v, got %#v", expectedGVK, gvk)
	}
}
