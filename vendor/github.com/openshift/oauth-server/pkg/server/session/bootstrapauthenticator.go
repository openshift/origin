package session

import (
	"net/http"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"

	bootstrap "github.com/openshift/library-go/pkg/authentication/bootstrapauthenticator"
)

func NewBootstrapAuthenticator(delegate SessionAuthenticator, getter bootstrap.BootstrapUserDataGetter, store Store) SessionAuthenticator {
	return &bootstrapAuthenticator{
		delegate: delegate,
		getter:   getter,
		store:    store,
	}
}

type bootstrapAuthenticator struct {
	delegate SessionAuthenticator
	getter   bootstrap.BootstrapUserDataGetter
	store    Store
}

func (b *bootstrapAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	authResponse, ok, err := b.delegate.AuthenticateRequest(req)
	if err != nil || !ok || authResponse.User.GetName() != bootstrap.BootstrapUser {
		return authResponse, ok, err
	}

	// make sure that the password has not changed since this cookie was issued
	// note that this is not really for security - it is so that we do not annoy the user
	// by letting them log in successfully only to have a token that does not work
	data, ok, err := b.getter.Get()
	if err != nil || !ok {
		return nil, ok, err
	}
	if data.UID != authResponse.User.GetUID() {
		return nil, false, nil
	}

	return authResponse, true, nil
}

func (b *bootstrapAuthenticator) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	if user.GetName() != bootstrap.BootstrapUser {
		return b.delegate.AuthenticationSucceeded(user, state, w, req)
	}

	// since osin is the IDP for this user, we increase the length
	// of the session to allow for transitions between components
	// this means the user could stay authenticated for one hour + OAuth access token lifetime
	return false, putUser(b.store, w, user, time.Hour)
}

func (b *bootstrapAuthenticator) InvalidateAuthentication(w http.ResponseWriter, user user.Info) error {
	if user.GetName() != bootstrap.BootstrapUser {
		return b.delegate.InvalidateAuthentication(w, user)
	}

	// the IDP is responsible for maintaining the user's session
	// since osin is the IDP for the bootstrap user, we do not invalidate its session
	// this is safe to do because we tie the cookie and token to the password hash
	return nil
}
