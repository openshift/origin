// +build integration,!no-etcd

package integration

import (
	"sync"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	etcdstorage "github.com/GoogleCloudPlatform/kubernetes/pkg/storage/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/user/api"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	testutil "github.com/openshift/origin/test/util"
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

	masterConfig, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	etcdClient, err := etcd.GetAndTestEtcdClient(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	etcdHelper, err := origin.NewEtcdStorage(etcdClient, masterConfig.EtcdStorageConfig.OpenShiftStorageVersion, masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	userRegistry := userregistry.NewRegistry(useretcd.NewREST(etcdHelper))
	identityRegistry := identityregistry.NewRegistry(identityetcd.NewREST(etcdHelper))
	useridentityMappingRegistry := useridentitymapping.NewRegistry(useridentitymapping.NewREST(userRegistry, identityRegistry))

	lookup := identitymapper.NewLookupIdentityMapper(useridentityMappingRegistry, userRegistry)
	provisioner := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(identityRegistry, userRegistry)

	testcases := map[string]struct {
		Identity authapi.UserIdentityInfo
		Mapper   authapi.UserIdentityMapper

		CreateIdentity *api.Identity
		CreateUser     *api.User
		CreateMapping  *api.UserIdentityMapping
		UpdateUser     *api.User

		ExpectedErr      error
		ExpectedUserName string
		ExpectedFullName string
	}{
		"lookup missing identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   lookup,

			ExpectedErr: kerrs.NewNotFound("UserIdentityMapping", "idp:bob"),
		},
		"lookup existing identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   lookup,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),

			ExpectedUserName: "mappeduser",
		},
		"provision missing identity and user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			ExpectedUserName: "bob",
		},
		"provision missing identity and user with preferred username and display name": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityDisplayNameKey: "Bob, Sr.", authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   provisioner,

			ExpectedUserName: "admin",
			ExpectedFullName: "Bob, Sr.",
		},
		"provision missing identity for existing user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			CreateUser: makeUser("bob", "idp:bob"),

			ExpectedUserName: "bob",
		},
		"provision missing identity with conflicting user": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			CreateUser: makeUser("bob"),

			ExpectedUserName: "bob2",
		},
		"provision missing identity with conflicting user and preferred username": {
			Identity: makeIdentityInfo("idp", "bob", map[string]string{authapi.IdentityPreferredUsernameKey: "admin"}),
			Mapper:   provisioner,

			CreateUser: makeUser("admin"),

			ExpectedUserName: "admin2",
		},
		"provision with existing unmapped identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			CreateIdentity: makeIdentity("idp", "bob"),

			ExpectedErr: kerrs.NewNotFound("UserIdentityMapping", "idp:bob"),
		},
		"provision with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr: kerrs.NewNotFound("UserIdentityMapping", "idp:bob"),
		},
		"provision with existing mapped identity without user backreference": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),
			// Update user to a version which does not reference the identity
			UpdateUser: makeUser("mappeduser"),

			ExpectedErr: kerrs.NewNotFound("UserIdentityMapping", "idp:bob"),
		},
		"provision returns existing mapping": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   provisioner,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),

			ExpectedUserName: "mappeduser",
		},
	}

	for k, testcase := range testcases {
		// Cleanup
		if err := etcdHelper.RecursiveDelete(useretcd.EtcdPrefix, true); err != nil && !etcdstorage.IsEtcdNotFound(err) {
			t.Fatalf("Could not clean up users: %v", err)
		}
		if err := etcdHelper.RecursiveDelete(identityetcd.EtcdPrefix, true); err != nil && !etcdstorage.IsEtcdNotFound(err) {
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

			}()
		}
		wg.Wait()
	}
}
