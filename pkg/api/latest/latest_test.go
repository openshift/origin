package latest

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
)

func TestRESTRootScope(t *testing.T) {
	for _, v := range [][]string{{"v1beta1"}, {"v1beta2"}, {"v1beta3"}, {"v1"}, {"", "v1beta1"}} {
		mapping, err := RESTMapper.RESTMapping("Node", v...)
		if err != nil {
			t.Fatal(err)
		}
		if mapping.Scope.Name() != meta.RESTScopeNameRoot {
			t.Errorf("Node should have a root scope: %#v", mapping.Scope)
		}
	}
	mapping, err := RESTMapper.RESTMapping("User", "v1beta1")
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Scope.Name() != meta.RESTScopeNameRoot {
		t.Errorf("User should have a root scope: %#v", mapping.Scope)
	}

	mapping, err = RESTMapper.RESTMapping("Status", "v1beta1")
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Scope.Name() != meta.RESTScopeNameRoot {
		t.Errorf("Status should have a root scope: %#v", mapping.Scope)
	}
}
