package identitymapper

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	userapi "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/pkg/user/registry/test"
)

type testInitializer struct {
	called bool
}

func (t *testInitializer) InitializeUser(identity *userapi.Identity, user *userapi.User) error {
	t.called = true
	return nil
}

type strategyTestCase struct {
	MakeStrategy func(user userclient.UserInterface, initializer Initializer) UserForNewIdentityGetter

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
		User: corev1.ObjectReference{
			UID:  types.UID(userUID),
			Name: userName,
		},
		Extra: map[string]string{},
	}
}
func makeUserIdentityMapping(identityUID string, providerName string, providerUserName string, userUID string, userName string) *userapi.UserIdentityMapping {
	return &userapi.UserIdentityMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: providerName + ":" + providerUserName,
		},
		User: corev1.ObjectReference{
			UID:  types.UID(userUID),
			Name: userName,
		},
		Identity: corev1.ObjectReference{
			UID:  types.UID(identityUID),
			Name: "stockvalue",
		},
	}
}

func (tc strategyTestCase) run(k string, t *testing.T) {
	actions := []test.Action{}
	userRegistry := &test.UserRegistry{
		GetUsers: map[string]*userapi.User{},
		Actions:  &actions,
	}
	for _, u := range tc.ExistingUsers {
		userRegistry.GetUsers[u.Name] = u
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
