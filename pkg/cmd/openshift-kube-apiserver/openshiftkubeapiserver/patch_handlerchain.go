package openshiftkubeapiserver

import (
	"net/http"
	"strings"

	"io/ioutil"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/oauth/urls"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
)

const (
	openShiftOAuthAPIPrefix      = "/oauth"
	openShiftLoginPrefix         = "/login"
	openShiftOAuthCallbackPrefix = "/oauth2callback"
)

// TODO switch back to taking a kubeapiserver config.  For now make it obviously safe for 3.11
func BuildHandlerChain(genericConfig *genericapiserver.Config, kubeInformers informers.SharedInformerFactory, legacyServiceServingCertSignerCABundle string, oauthConfig *configapi.OAuthConfig, userAgentMatchingConfig configapi.UserAgentMatchingConfig) (func(apiHandler http.Handler, kc *genericapiserver.Config) http.Handler, map[string]genericapiserver.PostStartHookFunc, error) {
	extraPostStartHooks := map[string]genericapiserver.PostStartHookFunc{}

	webconsoleProxyHandler, err := newWebConsoleProxy(kubeInformers, legacyServiceServingCertSignerCABundle)
	if err != nil {
		return nil, nil, err
	}
	oauthServerHandler, newPostStartHooks, err := NewOAuthServerHandler(genericConfig, oauthConfig)
	if err != nil {
		return nil, nil, err
	}
	for name, fn := range newPostStartHooks {
		extraPostStartHooks[name] = fn
	}

	return func(apiHandler http.Handler, genericConfig *genericapiserver.Config) http.Handler {
			// Machinery that let's use discover the Web Console Public URL
			accessor := newWebConsolePublicURLAccessor(genericConfig.LoopbackClientConfig)
			// the webconsole is proxied through the API server.  This starts a small controller that keeps track of where to proxy.
			// TODO stop proxying the webconsole. Should happen in a future release.
			extraPostStartHooks["openshift.io-webconsolepublicurl"] = func(context genericapiserver.PostStartHookContext) error {
				go accessor.Run(context.StopCh)
				return nil
			}

			// these are after the kube handler
			handler := versionSkewFilter(apiHandler, userAgentMatchingConfig)

			// this is the normal kube handler chain
			handler = genericapiserver.DefaultBuildHandlerChain(handler, genericConfig)

			// these handlers are all before the normal kube chain
			handler = translateLegacyScopeImpersonation(handler)
			handler = configprocessing.WithCacheControl(handler, "no-store") // protected endpoints should not be cached

			// redirects from / to /console if you're using a browser
			handler = withAssetServerRedirect(handler, accessor)

			// these handlers are actually separate API servers which have their own handler chains.
			// our server embeds these
			handler = withConsoleRedirection(handler, webconsoleProxyHandler, accessor)
			handler = withOAuthRedirection(oauthConfig, handler, oauthServerHandler)

			return handler
		},
		extraPostStartHooks,
		nil
}

func newWebConsoleProxy(kubeInformers informers.SharedInformerFactory, legacyServiceServingCertSignerCABundle string) (http.Handler, error) {
	caBundle, err := ioutil.ReadFile(legacyServiceServingCertSignerCABundle)
	if err != nil {
		return nil, err
	}
	proxyHandler, err := newServiceProxyHandler("webconsole", "openshift-web-console", aggregatorapiserver.NewClusterIPServiceResolver(kubeInformers.Core().V1().Services().Lister()), caBundle, "OpenShift web console")
	if err != nil {
		return nil, err
	}
	return proxyHandler, nil
}

func withOAuthRedirection(oauthConfig *configapi.OAuthConfig, handler, oauthServerHandler http.Handler) http.Handler {
	if oauthConfig == nil {
		return handler
	}

	glog.Infof("Starting OAuth2 API at %s", urls.OpenShiftOAuthAPIPrefix)
	return WithPatternPrefixHandler(handler, oauthServerHandler, openShiftOAuthAPIPrefix, openShiftLoginPrefix, openShiftOAuthCallbackPrefix)
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
