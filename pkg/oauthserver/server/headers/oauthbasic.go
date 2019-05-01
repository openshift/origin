package headers

import "net/http"

const (
	authzHeader     = "Authorization"
	copyAuthzHeader = "oauth.openshift.io:" + authzHeader // will never conflict because : is not a valid header key
)

func WithPreserveAuthorizationHeader(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vv, ok := r.Header[authzHeader]; ok {
			r.Header[copyAuthzHeader] = vv // capture the values before they are deleted
		}

		handler.ServeHTTP(w, r)
	})
}

func WithRestoreAuthorizationHeader(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vv, ok := r.Header[copyAuthzHeader]; ok {
			r.Header[authzHeader] = vv // add them back afterwards for use in OAuth flows
			delete(r.Header, copyAuthzHeader)
		}

		handler.ServeHTTP(w, r)
	})
}
