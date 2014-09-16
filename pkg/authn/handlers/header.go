package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/authn/api"
	"github.com/openshift/origin/pkg/authn/usercontext"
)

type HeaderConfig struct {
	UserHeaderName   string
	ForbiddenHandler http.Handler
}

func NewDefaultHeaderConfig() *HeaderConfig {
	return &HeaderConfig{
		UserHeaderName:   "X-Remote-User",
		ForbiddenHandler: forbidden,
	}
}

func AuthenticateFromHeaders(config *HeaderConfig, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name, ok := req.Header.Get(config.UserHeaderName)
		if !ok {
			config.ForbiddenHandler.ServeHTTP(w, req)
			return
		}
		user := &api.DefaultUserInfo{
			Name: name,
		}
		usercontext.With(req, user, func() {
			handler.ServeHTTP(w, req)
		})
	})
}
