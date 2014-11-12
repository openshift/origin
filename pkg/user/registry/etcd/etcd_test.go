package etcd

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/api/latest"

	"github.com/coreos/go-etcd/etcd"
	"github.com/openshift/origin/pkg/user"
	userapi "github.com/openshift/origin/pkg/user/api"
)

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, tools.RuntimeVersionAdapter{latest.ResourceVersioner}}, user.NewDefaultUserInitStrategy())
}

// This copy and paste is not pure ignorance.  This is that we can be sure that the key is getting made as we
// expect it to. If someone changes the location of these resources by say moving all the resources to
// "/origin/resources" (which is a really good idea), then they've made a breaking change and something should
// fail to let them know they've change some significant change and that other dependent pieces may break.
func makeTestUserIdentityMapping(providerId, identityName string) string {
	return fmt.Sprintf("/userIdentityMappings/%s:%s", providerId, identityName)
}

func TestEtcdGetUser(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	expectedResultingMapping := &userapi.UserIdentityMapping{
		TypeMeta: kapi.TypeMeta{ID: "tango"},
		Identity: userapi.Identity{
			Name:     "uniform",
			Provider: "victor",
		},
		User: userapi.User{
			TypeMeta: kapi.TypeMeta{ID: "uniform"},
			Name:     "uniform",
		},
	}
	key := makeTestUserIdentityMapping(expectedResultingMapping.Identity.Provider, expectedResultingMapping.Identity.Name)
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, expectedResultingMapping), 0)
	registry := NewTestEtcd(fakeClient)

	user, err := registry.GetUser("victor:uniform")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user.Name != expectedResultingMapping.User.Name {
		t.Errorf("Expected %#v, but we got %#v", expectedResultingMapping.User, user)
	}
}

func TestEtcdCreateUserIdentityMapping(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	testMapping := &userapi.UserIdentityMapping{
		TypeMeta: kapi.TypeMeta{ID: "quebec"},
		Identity: userapi.Identity{
			Name:     "romeo",
			Provider: "sierra",
		},
		User: userapi.User{},
	}
	expectedResultingMapping := &userapi.UserIdentityMapping{
		TypeMeta: kapi.TypeMeta{ID: "quebec"},
		Identity: userapi.Identity{
			Name:     "romeo",
			Provider: "sierra",
		},
		User: userapi.User{
			TypeMeta: kapi.TypeMeta{ID: "romeo"},
			Name:     "romeo",
		},
	}
	key := makeTestUserIdentityMapping(testMapping.Identity.Provider, testMapping.Identity.Name)

	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	persistedUserIdentityMapping, created, err := registry.CreateOrUpdateUserIdentityMapping(testMapping)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !created {
		t.Errorf("Expected to be created, but we were updated instead")
	}
	if persistedUserIdentityMapping == nil {
		t.Errorf("Expected %#v, but we got %#v", expectedResultingMapping, persistedUserIdentityMapping)
	}
	if compareUserIdentityMappingFieldsThatAreFixed(expectedResultingMapping, persistedUserIdentityMapping) {
		t.Errorf("Expected %#v, but we got %#v", expectedResultingMapping, persistedUserIdentityMapping)
	}
}

func TestEtcdUpdateUserIdentityMappingWithConflictingUser(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	startingMapping := &userapi.UserIdentityMapping{
		TypeMeta: kapi.TypeMeta{ID: "whiskey"},
		Identity: userapi.Identity{
			Name:     "xray",
			Provider: "yankee",
		},
		User: userapi.User{
			TypeMeta: kapi.TypeMeta{ID: "xray"},
			Name:     "xray",
		},
	}
	// this key is intentionally wrong so that we can have an internally consistend UserIdentityMapping
	// that was placed in a bad key location
	key := makeTestUserIdentityMapping("zulu", "alfa")
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, startingMapping), 0)
	registry := NewTestEtcd(fakeClient)

	testMapping := &userapi.UserIdentityMapping{
		TypeMeta: kapi.TypeMeta{ID: "bravo"},
		Identity: userapi.Identity{
			Name:     "alfa",
			Provider: "zulu",
		},
		User: userapi.User{},
	}

	persistedUserIdentityMapping, created, err := registry.CreateOrUpdateUserIdentityMapping(testMapping)
	if err == nil {
		t.Errorf("Expected an error, but we didn't get one")
	}

	const expectedError = "the provided user name does not match the existing mapping"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error %v, but we got %v", expectedError, expectedError)
	}
	if created {
		t.Errorf("Expected  be updated, but we were created instead")
	}
	if persistedUserIdentityMapping != nil {
		t.Errorf("Expected nil, but we got %#v", persistedUserIdentityMapping)
	}
}

func compareUserIdentityMappingFieldsThatAreFixed(expected, actual *userapi.UserIdentityMapping) bool {
	if ((actual == nil) && (expected != nil)) || ((actual != nil) && (expected == nil)) {
		return false
	}
	if actual.ID != expected.ID {
		return false
	}
	if actual.Identity.Name != expected.Identity.Name || actual.Identity.Provider != expected.Identity.Provider {
		return false
	}
	if actual.User.Name != expected.User.Name || actual.User.ID != actual.User.ID {
		return false
	}

	return true
}
