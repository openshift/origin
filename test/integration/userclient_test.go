// +build integration,etcd

package integration

import (
	"path"
	"reflect"
	"sync"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	etcdutil "k8s.io/kubernetes/pkg/storage/etcd/util"
	"k8s.io/kubernetes/pkg/types"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/user/api"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func init() {
	testutil.RequireEtcd()
}

func makeIdentityInfo(providerName, providerUserName string, extra map[string]string) authapi.UserIdentityInfo {
	info := authapi.NewDefaultUserIdentityInfo("idp", "bob")
	if extra != nil {
		info.Extra = extra
	}
	return info
}

func makeUser(name string, identities ...string) *api.User {
	return &api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Identities: identities,
	}
}
func makeIdentity(providerName, providerUserName string) *api.Identity {
	return &api.Identity{
		ObjectMeta: kapi.ObjectMeta{
			Name: providerName + ":" + providerUserName,
		},
		ProviderName:     providerName,
		ProviderUserName: providerUserName,
	}
}
func makeIdentityWithUserReference(providerName, providerUserName string, userName string, userUID types.UID) *api.Identity {
	identity := makeIdentity(providerName, providerUserName)
	identity.User.Name = userName
	identity.User.UID = userUID
	return identity
}
func makeMapping(user, identity string) *api.UserIdentityMapping {
	return &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{Name: identity},
		User:       kapi.ObjectReference{Name: user},
		Identity:   kapi.ObjectReference{Name: identity},
	}
}

func TestUserInitialization(t *testing.T) {

	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	etcdClient, err := etcd.MakeNewEtcdClient(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	storageVersion := unversioned.GroupVersion{Group: "", Version: masterConfig.EtcdStorageConfig.OpenShiftStorageVersion}
	etcdHelper, err := origin.NewEtcdStorage(etcdClient, storageVersion, masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userRegistry := userregistry.NewRegistry(useretcd.NewREST(etcdHelper))
	identityRegistry := identityregistry.NewRegistry(identityetcd.NewREST(etcdHelper))

	lookup, err := identitymapper.NewIdentityUserMapper(identityRegistry, userRegistry, identitymapper.MappingMethodLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	generate, err := identitymapper.NewIdentityUserMapper(identityRegistry, userRegistry, identitymapper.MappingMethodGenerate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	add, err := identitymapper.NewIdentityUserMapper(identityRegistry, userRegistry, identitymapper.MappingMethodAdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claim, err := identitymapper.NewIdentityUserMapper(identityRegistry, userRegistry, identitymapper.MappingMethodClaim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testcases := map[string]struct {
		Identity authapi.UserIdentityInfo
		Mapper   authapi.UserIdentityMapper

		CreateIdentity *api.Identity
		CreateUser     *api.User
		CreateMapping  *api.UserIdentityMapping
		UpdateUser     *api.User

		ExpectedErr        error
		ExpectedUserName   string
		ExpectedFullName   string
		ExpectedIdentities []string
	}{
		"lookup missing identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   lookup,

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"lookup existing identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   lookup,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),

			ExpectedUserName:   "mappeduser",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate missing identity and user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate missing identity and user with preferred username and display name": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityDisplayNameKey: "Bob, Sr.", authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   generate,

			ExpectedUserName:   "admin",
			ExpectedFullName:   "Bob, Sr.",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate missing identity for existing user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateUser: makeUser("bob", "idp:bob"),

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate missing identity with conflicting user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateUser: makeUser("bob"),

			ExpectedUserName:   "bob2",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate missing identity with conflicting user and preferred username": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   generate,

			CreateUser: makeUser("admin"),

			ExpectedUserName:   "admin2",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate with existing unmapped identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateIdentity: makeIdentity("idp", "bob"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"generate with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr:        kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
			ExpectedIdentities: []string{"idp:bob"},
		},
		"generate with existing mapped identity without user backreference": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),
			// Update user to a version which does not reference the identity
			UpdateUser: makeUser("mappeduser"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"generate returns existing mapping": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),

			ExpectedUserName:   "mappeduser",
			ExpectedIdentities: []string{"idp:bob"},
		},

		"add missing identity and user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"add missing identity and user with preferred username and display name": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityDisplayNameKey: "Bob, Sr.", authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   add,

			ExpectedUserName:   "admin",
			ExpectedFullName:   "Bob, Sr.",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"add missing identity for existing user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser: makeUser("bob", "idp:bob"),

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"add missing identity with conflicting user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser: makeUser("bob", "otheridp:otheruser"),

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"otheridp:otheruser", "idp:bob"},
		},
		"add missing identity with conflicting user and preferred username": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   add,

			CreateUser: makeUser("admin", "otheridp:otheruser"),

			ExpectedUserName:   "admin",
			ExpectedIdentities: []string{"otheridp:otheruser", "idp:bob"},
		},
		"add with existing unmapped identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateIdentity: makeIdentity("idp", "bob"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"add with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"add with existing mapped identity without user backreference": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),
			// Update user to a version which does not reference the identity
			UpdateUser: makeUser("mappeduser"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"add returns existing mapping": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),

			ExpectedUserName:   "mappeduser",
			ExpectedIdentities: []string{"idp:bob"},
		},

		"claim missing identity and user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"claim missing identity and user with preferred username and display name": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityDisplayNameKey: "Bob, Sr.", authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   claim,

			ExpectedUserName:   "admin",
			ExpectedFullName:   "Bob, Sr.",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"claim missing identity for existing user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser: makeUser("bob", "idp:bob"),

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"claim missing identity with existing available user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser: makeUser("bob"),

			ExpectedUserName:   "bob",
			ExpectedIdentities: []string{"idp:bob"},
		},
		"claim missing identity with conflicting user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser: makeUser("bob", "otheridp:otheruser"),

			ExpectedErr: identitymapper.NewClaimError(makeUser("bob", "otheridp:otheruser"), makeIdentity("idp", "bob")),
		},
		"claim missing identity with conflicting user and preferred username": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   claim,

			CreateUser: makeUser("admin", "otheridp:otheruser"),

			ExpectedErr: identitymapper.NewClaimError(makeUser("admin", "otheridp:otheruser"), makeIdentity("idp", "bob")),
		},
		"claim with existing unmapped identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateIdentity: makeIdentity("idp", "bob"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"claim with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"claim with existing mapped identity without user backreference": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),
			// Update user to a version which does not reference the identity
			UpdateUser: makeUser("mappeduser"),

			ExpectedErr: kerrs.NewNotFound(api.Resource("useridentitymapping"), "idp:bob"),
		},
		"claim returns existing mapping": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),

			ExpectedUserName:   "mappeduser",
			ExpectedIdentities: []string{"idp:bob"},
		},
	}

	oldEtcdClient, err := etcd.GetAndTestEtcdClient(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for k, testcase := range testcases {
		// Cleanup
		if _, err := oldEtcdClient.Delete(path.Join(masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix, useretcd.EtcdPrefix), true); err != nil && !etcdutil.IsEtcdNotFound(err) {
			t.Fatalf("Could not clean up users: %v", err)
		}
		if _, err := oldEtcdClient.Delete(path.Join(masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix, identityetcd.EtcdPrefix), true); err != nil && !etcdutil.IsEtcdNotFound(err) {
			t.Fatalf("Could not clean up identities: %v", err)
		}

		// Pre-create items
		if testcase.CreateUser != nil {
			_, err := clusterAdminClient.Users().Create(testcase.CreateUser)
			if err != nil {
				t.Errorf("%s: Could not create user: %v", k, err)
				continue
			}
		}
		if testcase.CreateIdentity != nil {
			_, err := clusterAdminClient.Identities().Create(testcase.CreateIdentity)
			if err != nil {
				t.Errorf("%s: Could not create identity: %v", k, err)
				continue
			}
		}
		if testcase.CreateMapping != nil {
			_, err := clusterAdminClient.UserIdentityMappings().Update(testcase.CreateMapping)
			if err != nil {
				t.Errorf("%s: Could not create mapping: %v", k, err)
				continue
			}
		}
		if testcase.UpdateUser != nil {
			if testcase.UpdateUser.ResourceVersion == "" {
				existingUser, err := clusterAdminClient.Users().Get(testcase.UpdateUser.Name)
				if err != nil {
					t.Errorf("%s: Could not get user to update: %v", k, err)
					continue
				}
				testcase.UpdateUser.ResourceVersion = existingUser.ResourceVersion
			}
			_, err := clusterAdminClient.Users().Update(testcase.UpdateUser)
			if err != nil {
				t.Errorf("%s: Could not update user: %v", k, err)
				continue
			}
		}

		// Spawn 5 simultaneous mappers to test race conditions
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				userInfo, err := testcase.Mapper.UserFor(testcase.Identity)
				if err != nil {
					if testcase.ExpectedErr == nil {
						t.Errorf("%s: Expected success, got error '%v'", k, err)
					} else if err.Error() != testcase.ExpectedErr.Error() {
						t.Errorf("%s: Expected error %v, got '%v'", k, testcase.ExpectedErr.Error(), err)
					}
					return
				}
				if err == nil && testcase.ExpectedErr != nil {
					t.Errorf("%s: Expected error '%v', got none", k, testcase.ExpectedErr)
					return
				}

				if userInfo.GetName() != testcase.ExpectedUserName {
					t.Errorf("%s: Expected username %s, got %s", k, testcase.ExpectedUserName, userInfo.GetName())
					return
				}

				user, err := clusterAdminClient.Users().Get(userInfo.GetName())
				if err != nil {
					t.Errorf("%s: Error getting user: %v", k, err)
				}
				if user.FullName != testcase.ExpectedFullName {
					t.Errorf("%s: Expected full name %s, got %s", k, testcase.ExpectedFullName, user.FullName)
				}
				if !reflect.DeepEqual(user.Identities, testcase.ExpectedIdentities) {
					t.Errorf("%s: Expected identities %v, got %v", k, testcase.ExpectedIdentities, user.Identities)
				}
			}()
		}
		wg.Wait()
	}
}
