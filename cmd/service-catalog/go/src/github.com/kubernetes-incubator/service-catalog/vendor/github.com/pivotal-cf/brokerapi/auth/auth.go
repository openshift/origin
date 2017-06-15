package auth

import "net/http"

type Wrapper struct {
	username string
	password string
}

func NewWrapper(username, password string) *Wrapper {
	return &Wrapper{
		username: username,
		password: password,
	}
}

const notAuthorized = "Not Authorized"

func (wrapper *Wrapper) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(wrapper, r) {
			http.Error(w, notAuthorized, http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func (wrapper *Wrapper) WrapFunc(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(wrapper, r) {
			http.Error(w, notAuthorized, http.StatusUnauthorized)
			return
		}

		handlerFunc(w, r)
	})
}

func authorized(wrapper *Wrapper, r *http.Request) bool {
	username, password, isOk := r.BasicAuth()
	return isOk && username == wrapper.username && password == wrapper.password
}
