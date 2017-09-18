package v1_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestDefaults(t *testing.T) {
	obj := &authorizationapiv1.PolicyRule{
		APIGroups: nil,
		Verbs:     []string{authorizationapi.VerbAll},
		Resources: []string{authorizationapi.ResourceAll},
	}
	out := &authorizationapi.PolicyRule{}
	if err := kapi.Scheme.Convert(obj, out, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.APIGroups, []string{authorizationapi.APIGroupAll}) {
		t.Errorf("did not default api groups: %#v", out)
	}
}
