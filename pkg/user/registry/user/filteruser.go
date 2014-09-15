package user

import (
	"net/http"
	"path"
)

type UserInfo interface {
	GetName() string
	GetUID() string
}

type UserContext interface {
	Get(*http.Request) (UserInfo, bool)
}

type UserContextFunc func(*http.Request) (UserInfo, bool)

func (f UserContextFunc) Get(req *http.Request) (UserInfo, bool) {
	return f(req)
}

func NewCurrentUserContextFilter(requestPath string, context UserContext, handler http.Handler) http.Handler {
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
