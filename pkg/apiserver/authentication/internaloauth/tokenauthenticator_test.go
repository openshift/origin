package internaloauth

import (
	"errors"
	"testing"
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	clienttesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/api/user/v1"
	userfake "github.com/openshift/client-go/user/clientset/versioned/fake"
	oapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthfake "github.com/openshift/origin/pkg/oauth/generated/internalclientset/fake"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
)

func TestAuthenticateTokenInvalidUID(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset(
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token", CreationTimestamp: metav1.Time{Time: time.Now()}},
			ExpiresIn:  600, // 10 minutes
			UserName:   "foo",
			UserUID:    string("bar1"),
		},
	)
	fakeUserClient := userfake.NewSimpleClientset(&userapi.User{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "bar2"}})

	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), fakeUserClient.UserV1().Users(), NoopGroupMapper{}, NewUIDValidator())

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err.Error() != "user.UID (bar2) does not match token.userUID (bar1)" {
		t.Errorf("Unexpected error: %v", err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}

func TestAuthenticateTokenNotFound(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset()
	fakeUserClient := userfake.NewSimpleClientset()
	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), fakeUserClient.UserV1().Users(), NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err == nil {
		t.Error("Expected not found error")
	}
	if !apierrs.IsNotFound(err) {
		t.Error("Expected not found error")
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}

func TestAuthenticateTokenOtherGetError(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset()
	fakeOAuthClient.PrependReactor("get", "oauthaccesstokens", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("get error")
	})
	fakeUserClient := userfake.NewSimpleClientset()
	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), fakeUserClient.UserV1().Users(), NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err == nil {
		t.Error("Expected error is missing!")
	}
	if err.Error() != "get error" {
		t.Errorf("Expected error %v, but got error %v", "get error", err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}

func TestAuthenticateTokenTimeout(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	testClock := &fakeTicker{clock: clock.NewFakeClock(time.Time{})}

	defaultTimeout := int32(30) // 30 seconds
	clientTimeout := int32(15)  // 15 seconds
	minTimeout := int32(10)     // 10 seconds -> 10/3 = a tick per 3 seconds

	testClient := oapi.OAuthClient{
		ObjectMeta:                          metav1.ObjectMeta{Name: "testClient"},
		AccessTokenInactivityTimeoutSeconds: &clientTimeout,
	}
	quickClient := oapi.OAuthClient{
		ObjectMeta:                          metav1.ObjectMeta{Name: "quickClient"},
		AccessTokenInactivityTimeoutSeconds: &minTimeout,
	}
	slowClient := oapi.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: "slowClient"},
	}
	testToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "testToken", CreationTimestamp: metav1.Time{Time: testClock.Now()}},
		ClientName:               "testClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: clientTimeout,
	}
	quickToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "quickToken", CreationTimestamp: metav1.Time{Time: testClock.Now()}},
		ClientName:               "quickClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: minTimeout,
	}
	slowToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "slowToken", CreationTimestamp: metav1.Time{Time: testClock.Now()}},
		ClientName:               "slowClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: defaultTimeout,
	}
	emergToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "emergToken", CreationTimestamp: metav1.Time{Time: testClock.Now()}},
		ClientName:               "quickClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: 5, // super short timeout
	}
	fakeOAuthClient := oauthfake.NewSimpleClientset(&testToken, &quickToken, &slowToken, &emergToken, &testClient, &quickClient, &slowClient)
	fakeUserClient := userfake.NewSimpleClientset(&userapi.User{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "bar"}})
	accessTokenGetter := fakeOAuthClient.Oauth().OAuthAccessTokens()
	oauthClients := fakeOAuthClient.Oauth().OAuthClients()
	lister := &fakeOAuthClientLister{
		clients: oauthClients,
	}

	timeouts := NewTimeoutValidator(accessTokenGetter, lister, defaultTimeout, minTimeout)

	// inject fake clock, which has some interesting properties
	// 1. A sleep will cause at most one ticker event, regardless of how long the sleep was
	// 2. The clock will hold one tick event and will drop the next one if something does not consume it first
	timeouts.ticker = testClock

	// decorate flush
	// The fake clock 1. and 2. require that we issue a wait(t, timeoutsSync) after each testClock.Sleep that causes a tick
	originalFlush := timeouts.flushHandler
	timeoutsSync := make(chan struct{})
	timeouts.flushHandler = func(flushHorizon time.Time) {
		originalFlush(flushHorizon)
		timeoutsSync <- struct{}{} // signal that flush is complete so we never race against it
	}

	// decorate putToken
	// We must issue a wait(t, putTokenSync) after each call to checkToken that should be successful
	originalPutToken := timeouts.putTokenHandler
	putTokenSync := make(chan struct{})
	timeouts.putTokenHandler = func(td *tokenData) {
		originalPutToken(td)
		putTokenSync <- struct{}{} // signal that putToken is complete so we never race against it
	}

	// add some padding to all sleep invocations to make sure we are not failing on any boundary values
	buffer := time.Nanosecond

	tokenAuthenticator := NewTokenAuthenticator(accessTokenGetter, fakeUserClient.UserV1().Users(), NoopGroupMapper{}, timeouts)

	go timeouts.Run(stopCh)

	// TIME: 0 seconds have passed here

	// first time should succeed for all
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	checkToken(t, "quickToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync) // from emergency flush because quickToken has a short enough timeout

	checkToken(t, "slowToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	// this should cause an emergency flush, if not the next auth will fail,
	// as the token will be timed out
	checkToken(t, "emergToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync) // from emergency flush because emergToken has a super short timeout

	// wait 6 seconds
	testClock.Sleep(5*time.Second + buffer)

	// a tick happens every 3 seconds
	wait(t, timeoutsSync)

	// TIME: 6th second

	// See if emergency flush happened
	checkToken(t, "emergToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync) // from emergency flush because emergToken has a super short timeout

	// wait for timeout (minTimeout + 1 - the previously waited 6 seconds)
	testClock.Sleep(time.Duration(minTimeout-5)*time.Second + buffer)
	wait(t, timeoutsSync)

	// TIME: 11th second

	// now we change the testClient and see if the testToken will still be
	// valid instead of timing out
	changeClient, ret := oauthClients.Get("testClient", metav1.GetOptions{})
	if ret != nil {
		t.Error("Failed to get testClient")
	} else {
		longTimeout := int32(20)
		changeClient.AccessTokenInactivityTimeoutSeconds = &longTimeout
		_, ret = oauthClients.Update(changeClient)
		if ret != nil {
			t.Error("Failed to update testClient")
		}
	}

	// this should fail, thus no call to wait(t, putTokenSync)
	checkToken(t, "quickToken", tokenAuthenticator, accessTokenGetter, testClock, false)

	// while this should get updated
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync)

	// wait for timeout
	testClock.Sleep(time.Duration(clientTimeout+1)*time.Second + buffer)

	// 16 seconds equals 5 more flushes, but the fake clock will only tick once during this time
	wait(t, timeoutsSync)

	// TIME: 27th second

	// this should get updated
	checkToken(t, "slowToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync)

	// while this should not fail
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync)
	// and should be updated to last at least till the 31st second
	token, err := accessTokenGetter.Get("testToken", metav1.GetOptions{})
	if err != nil {
		t.Error("Failed to get testToken")
	} else {
		if token.InactivityTimeoutSeconds < 31 {
			t.Errorf("Expected timeout in more than 31 seconds, found: %d", token.InactivityTimeoutSeconds)
		}
	}

	//now change testClient again, so that tokens do not expire anymore
	changeclient, ret := oauthClients.Get("testClient", metav1.GetOptions{})
	if ret != nil {
		t.Error("Failed to get testClient")
	} else {
		changeclient.AccessTokenInactivityTimeoutSeconds = new(int32)
		_, ret = oauthClients.Update(changeclient)
		if ret != nil {
			t.Error("Failed to update testClient")
		}
	}

	// and wait until test token should time out, and has been flushed for sure
	testClock.Sleep(time.Duration(minTimeout)*time.Second + buffer)
	wait(t, timeoutsSync)

	// while this should not fail
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, testClock, true)
	wait(t, putTokenSync)

	wait(t, timeoutsSync)

	// and should be updated to have a ZERO timeout
	token, err = accessTokenGetter.Get("testToken", metav1.GetOptions{})
	if err != nil {
		t.Error("Failed to get testToken")
	} else {
		if token.InactivityTimeoutSeconds != 0 {
			t.Errorf("Expected timeout of 0 seconds, found: %d", token.InactivityTimeoutSeconds)
		}
	}
}

type fakeOAuthClientLister struct {
	clients oauthclient.OAuthClientInterface
}

func (f fakeOAuthClientLister) Get(name string) (*oapi.OAuthClient, error) {
	return f.clients.Get(name, metav1.GetOptions{})
}

func (f fakeOAuthClientLister) List(selector labels.Selector) ([]*oapi.OAuthClient, error) {
	panic("not used")
}

type fakeTicker struct {
	clock *clock.FakeClock
	ch    <-chan time.Time
}

func (t *fakeTicker) Now() time.Time {
	return t.clock.Now()
}

func (t *fakeTicker) C() <-chan time.Time {
	return t.ch
}

func (t *fakeTicker) Stop() {}

func (t *fakeTicker) NewTicker(d time.Duration) {
	t.ch = t.clock.Tick(d)
}

func (t *fakeTicker) Sleep(d time.Duration) {
	t.clock.Sleep(d)
}

func checkToken(t *testing.T, name string, authf authenticator.Token, tokens oauthclient.OAuthAccessTokenInterface, current *fakeTicker, present bool) {
	t.Helper()
	userInfo, found, err := authf.AuthenticateToken(name)
	if present {
		if !found {
			t.Errorf("Did not find token %s!", name)
		}
		if err != nil {
			t.Errorf("Unexpected error checking for token %s: %v", name, err)
		}
		if userInfo == nil {
			t.Errorf("Did not get a user for token %s!", name)
		}
	} else {
		if found {
			token, tokenErr := tokens.Get(name, metav1.GetOptions{})
			if tokenErr != nil {
				t.Fatal(tokenErr)
			}
			t.Errorf("Found token (created=%s, timeout=%di, now=%s), but it should be gone!",
				token.CreationTimestamp, token.InactivityTimeoutSeconds, current.Now())
		}
		if err != errTimedout {
			t.Errorf("Unexpected error checking absence of token %s: %v", name, err)
		}
		if userInfo != nil {
			t.Errorf("Unexpected user checking absence of token %s: %v", name, userInfo)
		}
	}
}

func wait(t *testing.T, c chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(30 * time.Second):
		t.Fatal("failed to see channel event")
	}
}
