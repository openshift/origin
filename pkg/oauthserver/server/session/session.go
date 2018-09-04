package session

import (
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/sessions"
)

type store struct {
	// name of the cookie used for session data
	name string
	// do not use store's Get method, it mucks with global state for caching purposes
	// decoding a single small cookie multiple times is not the end of the world
	// currently we do not have any single request paths that decode the cookie multiple times
	store sessions.Store
}

func NewStore(name string, secure bool, secrets ...[]byte) Store {
	cookie := sessions.NewCookieStore(secrets...)
	// we encode expiration information into the cookie data to avoid browser bugs
	// since we do not set the Expires or Max-Age attributes, all cookies created by this store are session cookies
	cookie.Options.MaxAge = 0
	cookie.Options.HttpOnly = true
	cookie.Options.Secure = secure
	return &store{name: name, store: cookie}
}

func (s *store) Get(r *http.Request) Values {
	// always use New to avoid global state
	session, err := s.store.New(r, s.name)
	if err != nil {
		// ignore all errors, this could occur from poorly handling key rotation.
		// depending on how keys are incorrectly rotated,
		// verification or decryption can fail with various different errors.
		// even with a malicious actor trying to mess with the cookie,
		// there does not seem to be much that we gain from erroring
		// instead of just ignoring the junk data and returning empty Values.
		// empty Values means the user has to reauthenticate instead of getting stuck
		// on an error page until their cookie expires or is removed.
		// we leak less state information using this approach.

		// log the error in case we ever need to know that it is occurring
		// we do not log the request as that could leak sensitive information such as the cookie
		glog.V(4).Infof("failed to decode secure session cookie %s: %v", s.name, err)

		return Values{}
	}
	return session.Values
}

func (s *store) Put(w http.ResponseWriter, v Values) error {
	// build a session from an empty request to avoid any decoding overhead
	// always use New to avoid global state
	r := &http.Request{}
	session, err := s.store.New(r, s.name)
	if err != nil {
		return err
	}

	// override the values for the session
	session.Values = v

	// write the encoded cookie, the request parameter is ignored
	return s.store.Save(r, w, session)
}
