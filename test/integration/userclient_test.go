package integration

import (
	"path"
	"reflect"
	"sync"
	"testing"

	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"

	corev1 "k8s.io/api/core/v1"
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	userapi "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/userregistry/identitymapper"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func makeIdentityInfo(providerName, providerUserName string, extra map[string]string) authapi.UserIdentityInfo {
	info := authapi.NewDefaultUserIdentityInfo("idp", "bob")
	if extra != nil {
		info.Extra = extra
	}
	return info
}

func makeUser(name string, identities ...string) *userapi.User {
	return &userapi.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Identities: identities,
	}
}
func makeIdentity(providerName, providerUserName string) *userapi.Identity {
	return &userapi.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: providerName + ":" + providerUserName,
		},
		ProviderName:     providerName,
		ProviderUserName: providerUserName,
	}
}
func makeIdentityWithUserReference(providerName, providerUserName string, userName string, userUID types.UID) *userapi.Identity {
	identity := makeIdentity(providerName, providerUserName)
	identity.User.Name = userName
	identity.User.UID = userUID
	return identity
}
func makeMapping(user, identity string) *userapi.UserIdentityMapping {
	return &userapi.UserIdentityMapping{
		ObjectMeta: metav1.ObjectMeta{Name: identity},
		User:       corev1.ObjectReference{Name: user},
		Identity:   corev1.ObjectReference{Name: identity},
	}
}

func TestUserInitialization(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userClient, err := userclient.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lookup, err := identitymapper.NewIdentityUserMapper(userClient.Identities(), userClient.Users(), userClient.UserIdentityMappings(), identitymapper.MappingMethodLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	generate, err := identitymapper.NewIdentityUserMapper(userClient.Identities(), userClient.Users(), userClient.UserIdentityMappings(), identitymapper.MappingMethodGenerate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	add, err := identitymapper.NewIdentityUserMapper(userClient.Identities(), userClient.Users(), userClient.UserIdentityMappings(), identitymapper.MappingMethodAdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claim, err := identitymapper.NewIdentityUserMapper(userClient.Identities(), userClient.Users(), userClient.UserIdentityMappings(), identitymapper.MappingMethodClaim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testcases := map[string]struct {
		Identity authapi.UserIdentityInfo
		Mapper   authapi.UserIdentityMapper

		CreateIdentity *userapi.Identity
		CreateUser     *userapi.User
		CreateMapping  *userapi.UserIdentityMapping
		UpdateUser     *userapi.User

		ExpectedErr        error
		ExpectedUserName   string
		ExpectedFullName   string
		ExpectedIdentities []string
	}{
		"lookup missing identity": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   lookup,

			ExpectedErr: identitymapper.NewLookupError(makeIdentityInfo("idp", "bob", nil), kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob")),
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

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
		},
		"generate with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   generate,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr:        kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
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

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
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

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
		},
		"add with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
		},
		"add with existing mapped identity without user backreference": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   add,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),
			// Update user to a version which does not reference the identity
			UpdateUser: makeUser("mappeduser"),

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
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

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
		},
		"claim with existing mapped identity with invalid user UID": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentityWithUserReference("idp", "bob", "mappeduser", "invalidUID"),

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
		},
		"claim with existing mapped identity without user backreference": {
			Identity: makeIdentityInfo("idp", "bob", nil),
			Mapper:   claim,

			CreateUser:     makeUser("mappeduser"),
			CreateIdentity: makeIdentity("idp", "bob"),
			CreateMapping:  makeMapping("mappeduser", "idp:bob"),
			// Update user to a version which does not reference the identity
			UpdateUser: makeUser("mappeduser"),

			ExpectedErr: kerrs.NewNotFound(userapi.Resource("useridentitymapping"), "idp:bob"),
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

	client, err := etcd.MakeEtcdClientV3(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for k, testcase := range testcases {
		// Cleanup
		if _, err := client.KV.Delete(context.Background(), path.Join("/", masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix, "/users"), clientv3.WithPrefix()); err != nil {
			t.Fatalf("Could not clean up users: %v", err)
		}
		if _, err := client.KV.Delete(context.Background(), path.Join("/", masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix, "/useridentities"), clientv3.WithPrefix()); err != nil {
			t.Fatalf("Could not clean up identities: %v", err)
		}

		// Pre-create items
		if testcase.CreateUser != nil {
			_, err := userClient.Users().Create(testcase.CreateUser)
			if err != nil {
				t.Errorf("%s: Could not create user: %v", k, err)
				continue
			}
		}
		if testcase.CreateIdentity != nil {
			_, err := userClient.Identities().Create(testcase.CreateIdentity)
			if err != nil {
				t.Errorf("%s: Could not create identity: %v", k, err)
				continue
			}
		}
		if testcase.CreateMapping != nil {
			_, err := userClient.UserIdentityMappings().Update(testcase.CreateMapping)
			if err != nil {
				t.Errorf("%s: Could not create mapping: %v", k, err)
				continue
			}
		}
		if testcase.UpdateUser != nil {
			if testcase.UpdateUser.ResourceVersion == "" {
				existingUser, err := userClient.Users().Get(testcase.UpdateUser.Name, metav1.GetOptions{})
				if err != nil {
					t.Errorf("%s: Could not get user to update: %v", k, err)
					continue
				}
				testcase.UpdateUser.ResourceVersion = existingUser.ResourceVersion
			}
			_, err := userClient.Users().Update(testcase.UpdateUser)
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

				user, err := userClient.Users().Get(userInfo.GetName(), metav1.GetOptions{})
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
