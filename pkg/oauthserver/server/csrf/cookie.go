package csrf

import (
	"net/http"

	"github.com/openshift/origin/pkg/oauthserver/server/crypto"
)

type cookieCsrf struct {
	name   string
	path   string
	domain string
	secure bool
}

// NewCookieCSRF stores random CSRF tokens in a cookie created with the given options.
// Empty CSRF tokens or tokens that do not match the value of the cookie on the request
// are rejected.
func NewCookieCSRF(name, path, domain string, secure bool) CSRF {
	return &cookieCsrf{
		name:   name,
		path:   path,
		domain: domain,
		secure: secure,
	}
}

// Generate implements the CSRF interface
func (c *cookieCsrf) Generate(w http.ResponseWriter, req *http.Request) string {
	// reuse the session cookie if we already have one
	// this makes us more tolerant of multiple clicks against the login page
	cookie, err := req.Cookie(c.name)
	if err == nil && len(cookie.Value) > 0 {
		return cookie.Value
	}

	// do not set Expires or MaxAge to make this a session cookie
	cookie = &http.Cookie{
		Name:     c.name,
		Value:    crypto.Random256BitsString(),
		Path:     c.path,
		Domain:   c.domain,
		Secure:   c.secure,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	return cookie.Value
}

// Check implements the CSRF interface
func (c *cookieCsrf) Check(req *http.Request, value string) bool {
	if len(value) == 0 {
		return false
	}

	cookie, err := req.Cookie(c.name)
	if err != nil { // the only error returned here is ErrNoCookie
		return false
	}

	return crypto.IsEqualConstantTime(cookie.Value, value)
}
