// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	authapi "github.com/openshift/origin/pkg/auth/api"
	oapauth "github.com/openshift/origin/pkg/auth/authenticator/oauthpassword/registry"
	"github.com/openshift/origin/pkg/auth/context"
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
	deleteAllEtcdKeys()
	etcdClient := newEtcdClient()
	interfaces, _ := latest.InterfacesFor(latest.Version)
	userRegistry := etcd.New(tools.EtcdHelper{etcdClient, interfaces.Codec, tools.RuntimeVersionAdapter{interfaces.MetadataAccessor}}, user.NewDefaultUserInitStrategy())
	storage := map[string]apiserver.RESTStorage{
		"userIdentityMappings": useridentitymapping.NewREST(userRegistry),
		"users":                userregistry.NewREST(userRegistry),
	}

	server := httptest.NewServer(apiserver.Handle(storage, v1beta1.Codec, "/osapi/v1beta1", interfaces.MetadataAccessor))

	mapping := api.UserIdentityMapping{
		Identity: api.Identity{
			ObjectMeta: kapi.ObjectMeta{Name: "test"},
			Provider:   "",
			Extra: map[string]string{
				"name": "Mr. Test",
			},
		},
	}

	client, err := client.New(&kclient.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual, created, err := client.CreateOrUpdateUserIdentityMapping(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		// TODO: t.Errorf("expected created to be true")
	}

	expectedUser := api.User{
		ObjectMeta: kapi.ObjectMeta{Name: ":test"},
		FullName:   "Mr. Test",
	}
	expectedUser.Name = ":test"
	expectedUser.UID = actual.User.UID
	expected := &api.UserIdentityMapping{
		Identity: mapping.Identity,
		User:     expectedUser,
	}
	compareIgnoringSelfLinkAndVersion(t, expected, actual)

	user, err := userRegistry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, user)

	actualUser, err := client.GetUser(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, actualUser)
}

type testTokenSource struct {
	Token string
	Err   error
}

func (s *testTokenSource) AuthenticatePassword(username, password string) (string, bool, error) {
	return s.Token, s.Token != "", s.Err
}

func TestUserLookup(t *testing.T) {
	deleteAllEtcdKeys()
	etcdClient := newEtcdClient()
	interfaces, _ := latest.InterfacesFor(latest.Version)
	userRegistry := etcd.New(tools.EtcdHelper{etcdClient, interfaces.Codec, tools.RuntimeVersionAdapter{interfaces.MetadataAccessor}}, user.NewDefaultUserInitStrategy())
	userInfo := &authapi.DefaultUserInfo{
		Name: ":test",
	}
	userContext := context.NewRequestContextMap()
	userContextFunc := userregistry.UserContextFunc(func(req *http.Request) (userregistry.UserInfo, bool) {
		obj, found := userContext.Get(req)
		if user, ok := obj.(authapi.UserInfo); found && ok {
			return user, true
		}
		return nil, false
	})

	storage := map[string]apiserver.RESTStorage{
		"userIdentityMappings": useridentitymapping.NewREST(userRegistry),
		"users":                userregistry.NewREST(userRegistry),
	}

	apihandler := apiserver.Handle(storage, interfaces.Codec, "/osapi/v1beta1", interfaces.MetadataAccessor)
	apihandler = userregistry.NewCurrentUserContextFilter("/osapi/v1beta1/users/~", userContextFunc, apihandler)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userContext.Set(req, userInfo)
		apihandler.ServeHTTP(w, req)
	}))

	mapping := api.UserIdentityMapping{
		Identity: api.Identity{
			ObjectMeta: kapi.ObjectMeta{Name: "test"},
			Provider:   "",
			Extra: map[string]string{
				"name": "Mr. Test",
			},
		},
	}

	client, err := client.New(&kclient.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual, created, err := client.CreateOrUpdateUserIdentityMapping(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		// TODO: t.Errorf("expected created to be true")
	}
	expectedUser := api.User{
		ObjectMeta: kapi.ObjectMeta{Name: ":test"},
		FullName:   "Mr. Test",
	}
	expectedUser.Name = ":test"
	expectedUser.UID = actual.User.UID
	expected := &api.UserIdentityMapping{
		Identity: mapping.Identity,
		User:     expectedUser,
	}
	compareIgnoringSelfLinkAndVersion(t, expected, actual)

	// check the user returned by the registry
	user, err := userRegistry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, user)

	// check the user returned by the client
	actualUser, err := client.GetUser(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, actualUser)

	// check the current user
	currentUser, err := client.GetUser("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, currentUser)

	// test retrieving user info from a token
	authorizer := oapauth.New(&testTokenSource{Token: "token"}, server.URL, nil)
	info, ok, err := authorizer.AuthenticatePassword("foo", "bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("should have been authenticated")
	}
	if user.Name != info.GetName() || user.UID != info.GetUID() {
		t.Errorf("unexpected user info", info)
	}
}

func compareIgnoringSelfLinkAndVersion(t *testing.T, expected runtime.Object, actual runtime.Object) {
	if actualTypeMeta, _ := runtime.FindTypeMeta(actual); actualTypeMeta != nil {
		actualTypeMeta.SetSelfLink("")
		actualTypeMeta.SetResourceVersion("")
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected\n %#v,\n got\n %#v", expected, actual)
	}
}
