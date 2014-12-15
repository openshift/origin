package user

import (
	"net/http"
	"path"
)

type Info interface {
	GetName() string
	GetUID() string
}

// mux is an object that can register http handlers.
type mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
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

// InstallThisUser registers the APIServer log support function into a mux.
func InstallThisUser(mux mux, endpoint string, requestsToUsers Context, apiHandler http.Handler) {
	mux.HandleFunc(endpoint,
		func(w http.ResponseWriter, req *http.Request) {
			user, found := requestsToUsers.Get(req)
			if !found {
				http.Error(w, "Need to be authorized to access this method", http.StatusUnauthorized)
				return
			}

			base := path.Dir(req.URL.Path)
			req.URL.Path = path.Join(base, user.GetName())
			apiHandler.ServeHTTP(w, req)
		},
	)
}
