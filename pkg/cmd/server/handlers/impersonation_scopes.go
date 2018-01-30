package handlers

import (
	"net/http"

	authenticationv1 "k8s.io/api/authentication/v1"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authenticationapi "github.com/openshift/origin/pkg/oauthserver/api"
)

// TranslateLegacyScopeImpersonation is a filter that will translates user scope impersonation for openshift into the equivalent kube headers.
func TranslateLegacyScopeImpersonation(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, scope := range req.Header[authenticationapi.ImpersonateUserScopeHeader] {
			req.Header[authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationapi.ScopesKey] =
				append(req.Header[authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationapi.ScopesKey], scope)
		}

		handler.ServeHTTP(w, req)
	})
}
