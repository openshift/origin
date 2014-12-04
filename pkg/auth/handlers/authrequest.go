package handlers

import (
	"net/http"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

type RequestContext interface {
	Set(*http.Request, interface{})
	Remove(*http.Request)
}

func NewRequestAuthenticator(context RequestContext, auth authenticator.Request, failed http.Handler, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, ok, err := auth.AuthenticateRequest(req)
		if err != nil || !ok {
			failed.ServeHTTP(w, req)
			return
		}
		glog.V(1).Infof("Found user, %v, when accessing %v", user, req.URL)

		context.Set(req, user)
		defer context.Remove(req)

		handler.ServeHTTP(w, req)
	})
}
