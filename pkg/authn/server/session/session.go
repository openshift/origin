package session

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
)

type store struct {
	store sessions.Store
}

func NewStore(secrets ...string) Store {
	values := [][]byte{}
	for _, secret := range secrets {
		values = append(values, []byte(secret))
	}
	cookie := sessions.NewCookieStore(values...)
	return store{cookie}
}

func (s store) Get(req *http.Request, name string) (Session, error) {
	session, err := s.store.Get(req, name)
	return sessionWrapper{session}, err
}

func (s store) Save(w http.ResponseWriter, req *http.Request) error {
	return sessions.Save(req, w)
}

func (s store) Wrap(h http.Handler) http.Handler {
	return context.ClearHandler(h)
}

type sessionWrapper struct {
	session *sessions.Session
}

func (s sessionWrapper) Values() map[interface{}]interface{} {
	if s.session == nil {
		return map[interface{}]interface{}{}
	}
	return s.session.Values
}
