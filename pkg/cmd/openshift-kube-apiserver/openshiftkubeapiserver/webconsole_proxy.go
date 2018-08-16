package openshiftkubeapiserver

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/util/httprequest"
)

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func withAssetServerRedirect(handler http.Handler, accessor *webConsolePublicURLAccessor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" && httprequest.PrefersHTML(req) {
			webconsolePublicURL := accessor.getPublicConsoleURL()
			if len(webconsolePublicURL) > 0 {
				http.Redirect(w, req, webconsolePublicURL, http.StatusFound)
				return
			}
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}

func withConsoleRedirection(handler, assetServerHandler http.Handler, accessor *webConsolePublicURLAccessor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// blacklist well known paths so we do not risk recursion deadlocks
		for _, prefix := range []string{"/apis", "/api", "/oapi", "/healthz", "/version"} {
			if req.URL.Path == prefix || strings.HasPrefix(req.URL.Path, prefix+"/") {
				// Dispatch to the next handler
				handler.ServeHTTP(w, req)
				return
			}
		}

		webconsolePublicURL := accessor.getPublicConsoleURL()
		if len(webconsolePublicURL) > 0 {
			publicURL, err := url.Parse(webconsolePublicURL)
			if err != nil {
				// fails validation before here
				glog.Fatal(err)
				// Dispatch to the next handler
				handler.ServeHTTP(w, req)
				return
			}

			prefix := publicURL.Path
			// prefix must not include a trailing '/'
			lastIndex := len(publicURL.Path) - 1
			if publicURL.Path[lastIndex] == '/' {
				prefix = publicURL.Path[0:lastIndex]
			}
			if req.URL.Path == prefix || strings.HasPrefix(req.URL.Path, prefix+"/") {
				assetServerHandler.ServeHTTP(w, req)
				return
			}
		}

		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}

type webConsolePublicURLAccessor struct {
	publicURL       atomic.Value
	configMapGetter coreclientv1.ConfigMapsGetter
	polling         time.Duration
}

func newWebConsolePublicURLAccessor(clientConfig *rest.Config) *webConsolePublicURLAccessor {
	accessor := &webConsolePublicURLAccessor{
		configMapGetter: coreclientv1.NewForConfigOrDie(clientConfig),
	}
	return accessor
}

func (a *webConsolePublicURLAccessor) getPublicConsoleURL() string {
	currValue, ok := a.publicURL.Load().(string)
	if ok && len(currValue) > 0 {
		return currValue
	}

	// if we aren't already set, try to update
	return a.updatePublicConsoleURL()
}

func (a *webConsolePublicURLAccessor) updatePublicConsoleURL() string {
	// TODO: best effort ratelimit maybe
	configMap, err := a.configMapGetter.ConfigMaps("openshift-web-console").Get("webconsole-config", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	config, ok := configMap.Data["webconsole-config.yaml"]
	if !ok {
		return ""
	}
	webConsoleConfig, err := readWebConsoleConfiguration(config)
	if err != nil {
		return ""
	}
	newValue := webConsoleConfig.ClusterInfo.ConsolePublicURL
	a.publicURL.Store(newValue)

	// Once we have a value we relax polling to once per minute
	a.polling = 1 * time.Minute

	return newValue
}

func (a *webConsolePublicURLAccessor) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	a.polling = 1 * time.Second
	for {
		select {
		case <-stopCh:
			return
		case <-time.After(a.polling):
		}

		a.updatePublicConsoleURL()
	}
}

var (
	webconsoleConfigScheme = runtime.NewScheme()
	webconsoleConfigCodecs = serializer.NewCodecFactory(webconsoleConfigScheme)
)

func init() {
	if err := webconsoleconfigv1.AddToScheme(webconsoleConfigScheme); err != nil {
		panic(err)
	}
}

func readWebConsoleConfiguration(objBytes string) (*webconsoleconfigv1.WebConsoleConfiguration, error) {
	defaultConfigObj, err := runtime.Decode(webconsoleConfigCodecs.UniversalDecoder(webconsoleconfigv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		return nil, err
	}
	ret, ok := defaultConfigObj.(*webconsoleconfigv1.WebConsoleConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected *webconsoleconfigv1.WebConsoleConfiguration, got %T", defaultConfigObj)
	}

	return ret, nil
}
