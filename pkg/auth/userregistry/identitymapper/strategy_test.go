package identitymapper

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/user"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/registry/test"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
)

type testInitializer struct {
	called bool
}

func (t *testInitializer) InitializeUser(identity *userapi.Identity, user *userapi.User) error {
	t.called = true
	return nil
}

type strategyTestCase struct {
	MakeStrategy func(user userregistry.Registry, initializer user.Initializer) UserForNewIdentityGetter

	// Inputs
	PreferredUsername string
	Identity          *userapi.Identity

	// User registry setup
	ExistingUsers  []*userapi.User
	CreateResponse *userapi.User
	UpdateResponse *userapi.User

	// Expectations
	ExpectedActions    []test.Action
	ExpectedError      bool
	ExpectedUserName   string
	ExpectedInitialize bool
}

func makeUser(uid string, name string, identities ...string) *userapi.User {
	return &userapi.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(uid),
		},
		Identities: identities,
	}
}
func makeIdentity(uid string, providerName string, providerUserName string, userUID string, userName string) *userapi.Identity {
	return &userapi.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: providerName + ":" + providerUserName,
			UID:  types.UID(uid),
		},
		ProviderName:     providerName,
		ProviderUserName: providerUserName,
		User: kapi.ObjectReference{
			UID:  types.UID(userUID),
			Name: userName,
		},
		Extra: map[string]string{},
	}
}

func (tc strategyTestCase) run(k string, t *testing.T) {
	actions := []test.Action{}
	userRegistry := &test.UserRegistry{
		Get:     map[string]*userapi.User{},
		Actions: &actions,
	}
	for _, u := range tc.ExistingUsers {
		userRegistry.Get[u.Name] = u
	}

	testInit := &testInitializer{}
	strategy := tc.MakeStrategy(userRegistry, testInit)

	user, err := strategy.UserForNewIdentity(apirequest.NewContext(), tc.PreferredUsername, tc.Identity)
	if tc.ExpectedError != (err != nil) {
		t.Errorf("%s: Expected error=%v, got %v", k, tc.ExpectedError, err)
		return
	}
	if !tc.ExpectedError && user.Name != tc.ExpectedUserName {
		t.Errorf("%s: Expected username %v, got %v", k, tc.ExpectedUserName, user.Name)
		return
	}

	if tc.ExpectedInitialize != testInit.called {
		t.Errorf("%s: Expected initialize=%v, got initialize=%v", k, tc.ExpectedInitialize, testInit.called)
	}

	for i, action := range actions {
		if len(tc.ExpectedActions) <= i {
			t.Errorf("%s: expected %d actions, got extras: %#v", k, len(tc.ExpectedActions), actions[i:])
			return
		}
		expectedAction := tc.ExpectedActions[i]
		if !reflect.DeepEqual(expectedAction, action) {
			t.Errorf("%s: expected\n\t%s %#v\nGot\n\t%s %#v", k, expectedAction.Name, expectedAction.Object, action.Name, action.Object)
			continue
		}
	}
	if len(actions) < len(tc.ExpectedActions) {
		t.Errorf("Missing %d additional actions:\n\t%#v", len(tc.ExpectedActions)-len(actions), tc.ExpectedActions[len(actions):])
	}
}
