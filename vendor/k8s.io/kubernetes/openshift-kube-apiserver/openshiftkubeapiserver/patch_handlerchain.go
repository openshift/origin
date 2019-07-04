package openshiftkubeapiserver

import (
	"net/http"
	"strings"

	"k8s.io/klog"

	genericapiserver "k8s.io/apiserver/pkg/server"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/apiserver/apiserverconfig"
	"github.com/openshift/library-go/pkg/apiserver/httprequest"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

const (
	openShiftOAuthAPIPrefix      = "/oauth"
	openShiftLoginPrefix         = "/login"
	openShiftLogoutPrefix        = "/logout"
	openShiftOAuthCallbackPrefix = "/oauth2callback"
)

// TODO switch back to taking a kubeapiserver config.  For now make it obviously safe for 3.11
func BuildHandlerChain(genericConfig *genericapiserver.Config, oauthConfig *osinv1.OAuthConfig, authConfig kubecontrolplanev1.MasterAuthConfig, userAgentMatchingConfig kubecontrolplanev1.UserAgentMatchingConfig, consolePublicURL string) (func(apiHandler http.Handler, kc *genericapiserver.Config) http.Handler, error) {
	// ignore oauthConfig if we have a valid OAuth metadata file
	// this prevents us from running the internal OAuth server when we are honoring an external one
	if oauthMetadataFile := authConfig.OAuthMetadataFile; len(oauthMetadataFile) > 0 {
		if _, _, err := loadOAuthMetadataFile(oauthMetadataFile); err == nil {
			oauthConfig = nil // simplest way to keep existing code paths working
		}
	}

	var oauthServerHandler http.Handler
	if oauthConfig != nil {
		var err error
		oauthServerHandler, err = NewOAuthServerHandler(genericConfig, oauthConfig)
		if err != nil {
			return nil, err
		}
	}

	return func(apiHandler http.Handler, genericConfig *genericapiserver.Config) http.Handler {
			// these are after the kube handler
			handler := versionSkewFilter(apiHandler, userAgentMatchingConfig)

			// this is the normal kube handler chain
			handler = genericapiserver.DefaultBuildHandlerChain(handler, genericConfig)

			// these handlers are all before the normal kube chain
			handler = translateLegacyScopeImpersonation(handler)
			handler = apiserverconfig.WithCacheControl(handler, "no-store") // protected endpoints should not be cached

			// redirects from / and /console to consolePublicURL if you're using a browser
			handler = withConsoleRedirect(handler, consolePublicURL)

			if oauthConfig != nil {
				// these handlers are actually separate API servers which have their own handler chains.
				// our server embeds these
				handler = withOAuthRedirection(oauthConfig, handler, oauthServerHandler)
			}

			return handler
		},
		nil
}

func withOAuthRedirection(oauthConfig *osinv1.OAuthConfig, handler, oauthServerHandler http.Handler) http.Handler {
	if oauthConfig == nil {
		return handler
	}

	klog.Infof("Starting OAuth2 API at %s", oauthdiscovery.OpenShiftOAuthAPIPrefix)
	return WithPatternPrefixHandler(handler, oauthServerHandler, openShiftOAuthAPIPrefix, openShiftLoginPrefix, openShiftLogoutPrefix, openShiftOAuthCallbackPrefix)
}

func WithPatternPrefixHandler(handler http.Handler, patternHandler http.Handler, prefixes ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, p := range prefixes {
			if strings.HasPrefix(req.URL.Path, p) {
				patternHandler.ServeHTTP(w, req)
				return
			}
		}
		handler.ServeHTTP(w, req)
	})
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
