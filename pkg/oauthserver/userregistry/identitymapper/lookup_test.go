package identitymapper

import (
	"context"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	userapi "github.com/openshift/api/user/v1"
	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	userapiinternal "github.com/openshift/origin/pkg/user/apis/user"
	userapiconversion "github.com/openshift/origin/pkg/user/apis/user/v1"
	"github.com/openshift/origin/pkg/user/registry/test"
	mappingregistry "github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

// TODO this is actually testing the user identity mapping registry
func TestLookup(t *testing.T) {
	testcases := map[string]struct {
		ProviderName     string
		ProviderUserName string

		ExistingIdentity *userapi.Identity
		ExistingUser     *userapi.User

		ExpectedActions  []test.Action
		ExpectedError    bool
		ExpectedUserName string
	}{
		"no identity": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,

			ExpectedActions: []test.Action{
				{Name: "GetIdentity", Object: "idp:bob"},
			},
			ExpectedError: true,
		},

		"existing identity, no user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "", ""),
			ExistingUser:     nil,

			ExpectedActions: []test.Action{
				{Name: "GetIdentity", Object: "idp:bob"},
			},
			ExpectedError: true,
		},
		"existing identity, missing user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:     nil,

			ExpectedActions: []test.Action{
				{Name: "GetIdentity", Object: "idp:bob"},
				{Name: "GetUser", Object: "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, invalid user UID reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUIDInvalid", "bob"),
			ExistingUser:     makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{Name: "GetIdentity", Object: "idp:bob"},
				{Name: "GetUser", Object: "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, user reference without identity backreference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:     makeUser("bobUserUID", "bob" /*, "idp:bob"*/),

			ExpectedActions: []test.Action{
				{Name: "GetIdentity", Object: "idp:bob"},
				{Name: "GetUser", Object: "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:     makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{Name: "GetIdentity", Object: "idp:bob"},
				{Name: "GetUser", Object: "bob"},
				{Name: "GetUser", Object: "bob"}, // extra request is for group lookup
			},
			ExpectedUserName: "bob",
		},
	}

	for k, tc := range testcases {
		actions := []test.Action{}
		identityRegistry := &test.IdentityRegistry{
			GetIdentities: map[string]*userapi.Identity{},
			Actions:       &actions,
		}
		userRegistry := &test.UserRegistry{
			GetUsers: map[string]*userapi.User{},
			Actions:  &actions,
		}
		if tc.ExistingIdentity != nil {
			identityRegistry.GetIdentities[tc.ExistingIdentity.Name] = tc.ExistingIdentity
		}
		if tc.ExistingUser != nil {
			userRegistry.GetUsers[tc.ExistingUser.Name] = tc.ExistingUser
		}

		mappingStorage := mappingregistry.NewREST(userRegistry, identityRegistry)
		mappingRegistry := NewRegistry(mappingStorage)

		lookupMapper := &lookupIdentityMapper{
			mappings: &FakeMappingClient{mappingRegistry},
			users:    userRegistry,
		}

		identity := authapi.NewDefaultUserIdentityInfo(tc.ProviderName, tc.ProviderUserName)
		user, err := lookupMapper.UserFor(identity)
		if tc.ExpectedError != (err != nil) {
			t.Errorf("%s: Expected error=%v, got %v", k, tc.ExpectedError, err)
			continue
		}
		if !tc.ExpectedError && user.GetName() != tc.ExpectedUserName {
			t.Errorf("%s: Expected username %v, got %v", k, tc.ExpectedUserName, user.GetName())
			continue
		}

		for i, action := range actions {
			if len(tc.ExpectedActions) <= i {
				t.Fatalf("%s: expected %d actions, got extras: %#v", k, len(tc.ExpectedActions), actions[i:])
				continue
			}
			expectedAction := tc.ExpectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Fatalf("%s: expected\n\t%s %#v\nGot\n\t%s %#v", k, expectedAction.Name, expectedAction.Object, action.Name, action.Object)
				continue
			}
		}
		if len(actions) < len(tc.ExpectedActions) {
			t.Errorf("Missing %d additional actions:\n\t%#v", len(tc.ExpectedActions)-len(actions), tc.ExpectedActions[len(actions):])
		}
	}
}

type FakeMappingClient struct {
	mappingRegistry mappingRegistry
}

func (f *FakeMappingClient) Create(obj *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	return f.mappingRegistry.CreateUserIdentityMapping(context.TODO(), obj)
}
func (f *FakeMappingClient) Update(obj *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	return f.mappingRegistry.UpdateUserIdentityMapping(context.TODO(), obj)
}
func (f *FakeMappingClient) Delete(name string, options *v1.DeleteOptions) error {
	return f.mappingRegistry.DeleteUserIdentityMapping(context.TODO(), name)
}
func (f *FakeMappingClient) Get(name string, options v1.GetOptions) (*userapi.UserIdentityMapping, error) {
	return f.mappingRegistry.GetUserIdentityMapping(context.TODO(), name, &options)
}

// Registry is an interface implemented by things that know how to store UserIdentityMapping objects.
type mappingRegistry interface {
	// GetUserIdentityMapping returns a UserIdentityMapping for the named identity
	GetUserIdentityMapping(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.UserIdentityMapping, error)
	// CreateUserIdentityMapping associates a user and an identity
	CreateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error)
	// UpdateUserIdentityMapping updates an associated user and identity
	UpdateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error)
	// DeleteUserIdentityMapping removes the user association for the named identity
	DeleteUserIdentityMapping(ctx apirequest.Context, name string) error
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type mappingStorage interface {
	rest.Getter
	rest.Deleter

	Create(ctx apirequest.Context, obj runtime.Object, createValidate rest.ValidateObjectFunc, _ bool) (runtime.Object, error)
	Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidate rest.ValidateObjectFunc, updateValidate rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error)
}

// storage puts strong typing around storage calls
type storage struct {
	mappingStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s mappingStorage) mappingRegistry {
	return &storage{s}
}

func (s *storage) GetUserIdentityMapping(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.UserIdentityMapping, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	out := &userapi.UserIdentityMapping{}
	if err := userapiconversion.Convert_user_UserIdentityMapping_To_v1_UserIdentityMapping(obj.(*userapiinternal.UserIdentityMapping), out, nil); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *storage) CreateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := s.Create(ctx, mapping, rest.ValidateAllObjectFunc, false)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.UserIdentityMapping), nil
}

func (s *storage) UpdateUserIdentityMapping(ctx apirequest.Context, mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, _, err := s.Update(ctx, mapping.Name, rest.DefaultUpdatedObjectInfo(mapping), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.UserIdentityMapping), nil
}

//
func (s *storage) DeleteUserIdentityMapping(ctx apirequest.Context, name string) error {
	_, err := s.Delete(ctx, name)
	return err
}
