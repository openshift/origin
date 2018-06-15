package csrf

import (
	"net/http"

	"github.com/pborman/uuid"

	"github.com/openshift/origin/pkg/oauthserver/server/session"
)

const CSRFKey = "csrf"

type sessionCsrf struct {
	store session.Store
	name  string
}

// NewSessionCSRF stores CSRF tokens in a session with the given name.
// Empty CSRF tokens or tokens that do not match the value in the session are rejected.
func NewSessionCSRF(store session.Store, name string) CSRF {
	return &sessionCsrf{
		store: store,
		name:  name,
	}
}

// Generate implements the CSRF interface
func (c *sessionCsrf) Generate(w http.ResponseWriter, req *http.Request) (string, error) {
	session, err := c.store.Get(req, c.name)
	if err != nil {
		return "", err
	}

	values := session.Values()
	csrfString, ok := values[CSRFKey].(string)
	if ok && csrfString != "" {
		return csrfString, nil
	}

	csrfString = uuid.NewUUID().String()
	values[CSRFKey] = csrfString

	// TODO: defer save until response is written?
	if err = c.store.Save(w, req); err != nil {
		return "", err
	}

	return csrfString, nil
}

// Check implements the CSRF interface
func (c *sessionCsrf) Check(req *http.Request, value string) (bool, error) {
	if len(value) == 0 {
		return false, nil
	}

	session, err := c.store.Get(req, c.name)
	if err != nil {
		return false, err
	}

	values := session.Values()
	csrfString, ok := values[CSRFKey].(string)
	if ok && csrfString == value {
		return true, nil
	}

	return false, nil
}
