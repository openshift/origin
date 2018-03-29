package origin

import (
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/util/httprequest"
)

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func withAssetServerRedirect(handler http.Handler, configMapGetter coreclientv1.ConfigMapsGetter, stopCh <-chan struct{}) http.Handler {
	webConsolePublicURLAccessor := &webConsolePublicURLAccessor{
		configMapGetter: configMapGetter,
	}
	go webConsolePublicURLAccessor.run(stopCh)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			handler.ServeHTTP(w, req)
			return
		}
		if !httprequest.PrefersHTML(req) {
			handler.ServeHTTP(w, req)
			return
		}

		webconsolePublicURL := webConsolePublicURLAccessor.getPublicConsoleURL()
		if len(webconsolePublicURL) == 0 {
			handler.ServeHTTP(w, req)
			return
		}

		http.Redirect(w, req, webconsolePublicURL, http.StatusFound)
	})
}

func (c *MasterConfig) withConsoleRedirection(handler, assetServerHandler http.Handler, webconsolePublicURL string) http.Handler {
	if len(webconsolePublicURL) == 0 {
		return handler
	}

	publicURL, err := url.Parse(webconsolePublicURL)
	if err != nil {
		// fails validation before here
		glog.Fatal(err)
	}
	// path always ends in a slash or the
	prefix := publicURL.Path
	lastIndex := len(publicURL.Path) - 1
	if publicURL.Path[lastIndex] == '/' {
		prefix = publicURL.Path[0:lastIndex]
	}

	return WithPatternPrefixHandler(handler, assetServerHandler, prefix)
}

type webConsolePublicURLAccessor struct {
	publicURL       atomic.Value
	configMapGetter coreclientv1.ConfigMapsGetter
}

func (a *webConsolePublicURLAccessor) getPublicConsoleURL() string {
	currValue, ok := a.publicURL.Load().(string)
	if ok && len(currValue) > 0 {
		return currValue
	}

	// if we aren't already set, try a live update
	return a.updatePublicConsoleURL()
}

func (a *webConsolePublicURLAccessor) updatePublicConsoleURL() string {
	// check to see if someone already did our work
	currValue, ok := a.publicURL.Load().(string)
	if ok && len(currValue) > 0 {
		return currValue
	}

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

	return newValue
}

func (a *webConsolePublicURLAccessor) run(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case <-time.After(10 * time.Second):
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
