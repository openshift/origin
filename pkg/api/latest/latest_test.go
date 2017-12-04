package latest

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	_ "k8s.io/kubernetes/pkg/apis/core/install"

	userapiv1 "github.com/openshift/api/user/v1"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	_ "github.com/openshift/origin/pkg/user/apis/user/install"
)

func TestRESTRootScope(t *testing.T) {
	for _, v := range [][]string{{"v1"}} {
		mapping, err := legacyscheme.Registry.RESTMapper().RESTMapping(kapi.Kind("Node"), v...)
		if err != nil {
			t.Fatal(err)
		}
		if mapping.Scope.Name() != meta.RESTScopeNameRoot {
			t.Errorf("Node should have a root scope: %#v", mapping.Scope)
		}
	}
}

func TestLegacyResourceToKind(t *testing.T) {
	// Ensure we resolve to latest.Version
	expectedGVK := Version.WithKind("User")
	gvk, err := legacyscheme.Registry.RESTMapper().KindFor(userapi.LegacySchemeGroupVersion.WithResource("User"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gvk != expectedGVK {
		t.Fatalf("Expected RESTMapper.KindFor('user') to be %#v, got %#v", expectedGVK, gvk)
	}
}

func TestResourceToKind(t *testing.T) {
	// Ensure we resolve to latest.Version
	expectedGVK := userapiv1.SchemeGroupVersion.WithKind("User")
	gvk, err := legacyscheme.Registry.RESTMapper().KindFor(userapi.SchemeGroupVersion.WithResource("User"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gvk != expectedGVK {
		t.Fatalf("Expected RESTMapper.KindFor('user') to be %#v, got %#v", expectedGVK, gvk)
	}
}

func TestUpstreamResourceToKind(t *testing.T) {
	// Ensure we resolve to klatest.ExternalVersions[0]
	meta, _ := legacyscheme.Registry.Group("")
	expectedGVK := meta.GroupVersion.WithKind("Pod")
	gvk, err := legacyscheme.Registry.RESTMapper().KindFor(kapi.SchemeGroupVersion.WithResource("Pod"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gvk != expectedGVK {
		t.Fatalf("Expected RESTMapper.KindFor('pod') to be %#v, got %#v", expectedGVK, gvk)
	}
}
