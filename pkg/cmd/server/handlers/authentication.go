package handlers

import (
	"net/http"

	"github.com/golang/glog"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

// AuthenticationHandlerFilter creates a filter object that will enforce authentication directly
func AuthenticationHandlerFilter(handler http.Handler, authenticator authenticator.Request, contextMapper apirequest.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, ok, err := authenticator.AuthenticateRequest(req)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}
		if err := contextMapper.Update(req, apirequest.WithUser(ctx, user)); err != nil {
			glog.V(4).Infof("Error setting authenticated context: %v", err)
			http.Error(w, "Unable to set authenticated request context", http.StatusInternalServerError)
			return
		}

		handler.ServeHTTP(w, req)
	})
}
