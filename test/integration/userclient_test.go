// +build integration,!no-etcd

package integration

import (
	"net/http/httptest"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
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
	etcdClient := newEtcdClient()
	interfaces, _ := latest.InterfacesFor(latest.Version)
	userRegistry := etcd.New(tools.EtcdHelper{etcdClient, interfaces.Codec, interfaces.ResourceVersioner}, user.NewDefaultUserInitStrategy())
	storage := map[string]apiserver.RESTStorage{
		"userIdentityMappings": useridentitymapping.NewREST(userRegistry),
		"users":                userregistry.NewREST(userRegistry),
	}

	server := httptest.NewServer(apiserver.Handle(storage, v1beta1.Codec, "/osapi/v1beta1", interfaces.SelfLinker))

	mapping := api.UserIdentityMapping{
		Identity: api.Identity{
			Provider: "",
			Name:     "test",
			Extra: map[string]string{
				"name": "Mr. Test",
			},
		},
	}

	client, err := client.New(&kclient.Config{Host: server.URL})
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
	actual.JSONBase = kapi.JSONBase{}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected\n %#v,\n got\n %#v", expected, actual)
	}

	user, err := userRegistry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	user.JSONBase = kapi.JSONBase{}
	if !reflect.DeepEqual(&expected.User, user) {
		t.Errorf("expected\n %#v,\n got\n %#v", &expected.User, user)
	}

	actualUser, err := client.GetUser(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actualUser.JSONBase = kapi.JSONBase{}
	if !reflect.DeepEqual(&expected.User, actualUser) {
		t.Errorf("expected\n %#v,\n got\n %#v", &expected.User, actualUser)
	}
}
