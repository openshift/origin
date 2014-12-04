package user

import (
	"net/http"
	"path"
)

type Info interface {
	GetName() string
	GetUID() string
}

type Context interface {
	Get(*http.Request) (Info, bool)
}

type ContextFunc func(*http.Request) (Info, bool)

func (f ContextFunc) Get(req *http.Request) (Info, bool) {
	return f(req)
}

func NewCurrentContextFilter(requestPath string, context Context, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != requestPath {
			handler.ServeHTTP(w, req)
			return
		}

		user, found := context.Get(req)
		if !found {
			http.Error(w, "Need to be authorized to access this method", http.StatusUnauthorized)
			return
		}

		base := path.Dir(req.URL.Path)
		req.URL.Path = path.Join(base, user.GetName())
		handler.ServeHTTP(w, req)
	})
}
