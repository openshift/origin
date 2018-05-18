package identitymapper

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	clienttesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/api/user/v1"
	userv1fakeclient "github.com/openshift/client-go/user/clientset/versioned/fake"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
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
	ExistingUsers []runtime.Object

	// Expectations
	ValidateActions    func(t *testing.T, actions []clienttesting.Action)
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
	t.Run(k, func(t *testing.T) {
		fakeClient := userv1fakeclient.NewSimpleClientset(tc.ExistingUsers...)

		testInit := &testInitializer{}
		strategy := tc.MakeStrategy(fakeClient.UserV1().Users(), testInit)

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

		tc.ValidateActions(t, fakeClient.Actions())
	})
}
