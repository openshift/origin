package internaloauth

import (
	"testing"
	"time"

	userapi "github.com/openshift/api/user/v1"
	userfake "github.com/openshift/client-go/user/clientset/versioned/fake"
	oapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthfake "github.com/openshift/origin/pkg/oauth/generated/internalclientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuthenticateTokenExpired(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset(
		// expired token that had a lifetime of 10 minutes
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token1", CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)}},
			ExpiresIn:  600,
			UserName:   "foo",
		},
		// non-expired token that has a lifetime of 10 minutes, but has a non-nil deletion timestamp
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token2", CreationTimestamp: metav1.Time{Time: time.Now()}, DeletionTimestamp: &metav1.Time{}},
			ExpiresIn:  600,
			UserName:   "foo",
		},
	)
	fakeUserClient := userfake.NewSimpleClientset(&userapi.User{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "bar"}})

	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), fakeUserClient.UserV1().Users(), NoopGroupMapper{}, NewExpirationValidator())

	for _, tokenName := range []string{"token1", "token2"} {
		userInfo, found, err := tokenAuthenticator.AuthenticateToken(tokenName)
		if found {
			t.Error("Found token, but it should be missing!")
		}
		if err != errExpired {
			t.Errorf("Unexpected error: %v", err)
		}
		if userInfo != nil {
			t.Errorf("Unexpected user: %v", userInfo)
		}
	}
}

func TestAuthenticateTokenValidated(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset(
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token", CreationTimestamp: metav1.Time{Time: time.Now()}},
			ExpiresIn:  600, // 10 minutes
			UserName:   "foo",
			UserUID:    string("bar"),
		},
	)
	fakeUserClient := userfake.NewSimpleClientset(&userapi.User{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "bar"}})

	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), fakeUserClient.UserV1().Users(), NoopGroupMapper{}, NewExpirationValidator(), NewUIDValidator())

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if !found {
		t.Error("Did not find a token!")
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if userInfo == nil {
		t.Error("Did not get a user!")
	}
}
