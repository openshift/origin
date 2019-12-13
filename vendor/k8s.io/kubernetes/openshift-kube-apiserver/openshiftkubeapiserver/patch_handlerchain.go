package openshiftkubeapiserver

import (
	"net/http"
	"strings"

	genericapiserver "k8s.io/apiserver/pkg/server"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/library-go/pkg/apiserver/httprequest"
)

// TODO switch back to taking a kubeapiserver config.  For now make it obviously safe for 3.11
func BuildHandlerChain(userAgentMatchingConfig kubecontrolplanev1.UserAgentMatchingConfig, consolePublicURL string) (func(apiHandler http.Handler, kc *genericapiserver.Config) http.Handler, error) {
	return func(apiHandler http.Handler, genericConfig *genericapiserver.Config) http.Handler {
			// these are after the kube handler
			handler := versionSkewFilter(apiHandler, userAgentMatchingConfig)

			// this is the normal kube handler chain
			handler = genericapiserver.DefaultBuildHandlerChain(handler, genericConfig)

			// these handlers are all before the normal kube chain
			handler = translateLegacyScopeImpersonation(handler)

			// redirects from / and /console to consolePublicURL if you're using a browser
			handler = withConsoleRedirect(handler, consolePublicURL)

			return handler
		},
		nil
}

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func withConsoleRedirect(handler http.Handler, consolePublicURL string) http.Handler {
	if len(consolePublicURL) == 0 {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/console") ||
			(req.URL.Path == "/" && httprequest.PrefersHTML(req)) {
			http.Redirect(w, req, consolePublicURL, http.StatusFound)
			return
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}
