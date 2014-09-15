// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authapi "github.com/openshift/origin/pkg/authn/api"
	oapauth "github.com/openshift/origin/pkg/authn/authenticator/oauthpassword/registry"
	"github.com/openshift/origin/pkg/authn/context"
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

	actual, created, err := client.CreateOrUpdateUserIdentityMapping(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Errorf("expected created to be true")
	}

	expectedUser := api.User{
		Name:     ":test",
		UID:      actual.User.UID,
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

type testTokenSource struct {
	Token string
	Err   error
}

func (s *testTokenSource) AuthenticatePassword(username, password string) (string, bool, error) {
	return s.Token, s.Token != "", s.Err
}

func TestUserLookup(t *testing.T) {
	deleteAllEtcdKeys()
	registry := etcd.New(newEtcdClient(), user.NewDefaultUserInitStrategy())
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

	apihandler := apiserver.Handle(map[string]apiserver.RESTStorage{
		"userIdentityMappings": useridentitymapping.NewREST(registry),
		"users":                userregistry.NewREST(registry),
	}, runtime.Codec, "/osapi/v1beta1")
	apihandler = userregistry.NewCurrentUserContextFilter("/osapi/v1beta1/users/~", userContextFunc, apihandler)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userContext.Set(req, userInfo)
		apihandler.ServeHTTP(w, req)
	}))

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

	actual, created, err := client.CreateOrUpdateUserIdentityMapping(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Errorf("expected created to be true")
	}
	expectedUser := api.User{
		Name:     ":test",
		UID:      actual.User.UID,
		FullName: "Mr. Test",
	}
	expected := &api.UserIdentityMapping{
		Identity: mapping.Identity,
		User:     expectedUser,
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %#v, got %#v", expected, actual)
	}

	// check the user returned by the registry
	user, err := registry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(&expected.User, user) {
		t.Errorf("expected %#v, got %#v", expected.User, user)
	}

	// check the user returned by the client
	actualUser, err := client.GetUser(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(&expected.User, actualUser) {
		t.Errorf("expected %#v, got %#v", expected.User, actualUser)
	}

	// check the current user
	currentUser, err := client.GetUser("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(&expected.User, currentUser) {
		t.Errorf("expected %#v, got %#v", expected.User, currentUser)
	}

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
