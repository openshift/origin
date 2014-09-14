// +build integration,!no-etcd

package integration

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/user"
	"github.com/openshift/origin/pkg/user/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
	"github.com/openshift/origin/pkg/user/registry/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

func init() {
	requireEtcd()
}

func TestUserInitialization(t *testing.T) {
	registry := etcd.New(newEtcdClient(), user.NewDefaultUserInitStrategy())
	server := httptest.NewServer(apiserver.Handle(map[string]apiserver.RESTStorage{
		"userIdentityMappings": useridentitymapping.NewREST(registry),
		"users":                userregistry.NewREST(registry),
	}, runtime.Codec, "/osapi/v1beta1"))
	mapping := api.UserIdentityMapping{
		Identity: api.Identity{
			Provider: "",
			Name:     "test",
			Extra: map[string]string{
				"name": "Mr. Test",
			},
		},
	}

	client, err := client.New(server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual, err := client.GetOrCreateUserIdentityMapping(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedUser := api.User{
		Name:     ":test",
		FullName: "Mr. Test",
	}
	expected := &api.UserIdentityMapping{
		Identity: mapping.Identity,
		User:     expectedUser,
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %#v, got %#v", expected, actual)
	}

	user, err := registry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(&expected.User, user) {
		t.Errorf("expected %#v, got %#v", expected.User, user)
	}

	actualUser, err := client.GetUser(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(&expected.User, actualUser) {
		t.Errorf("expected %#v, got %#v", expected.User, actualUser)
	}
}
