package session

import (
	"net/http"
)

type Store interface {
	Get(r *http.Request, name string) (Session, error)
	Save(http.ResponseWriter, *http.Request) error
	Wrap(http.Handler) http.Handler
}

type Session interface {
	Values() map[interface{}]interface{}
}
