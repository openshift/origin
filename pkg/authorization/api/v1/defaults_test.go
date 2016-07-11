package v1_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestDefaults(t *testing.T) {
	obj := &v1.PolicyRule{
		APIGroups: nil,
		Verbs:     []string{api.VerbAll},
		Resources: []string{api.ResourceAll},
	}
	out := &api.PolicyRule{}
	if err := kapi.Scheme.Convert(obj, out, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.APIGroups, []string{api.APIGroupAll}) {
		t.Errorf("did not default api groups: %#v", out)
	}
}
