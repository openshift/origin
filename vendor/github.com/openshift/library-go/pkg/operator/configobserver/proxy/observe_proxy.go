package proxy

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
)

type ProxyLister interface {
	ProxyLister() configlistersv1.ProxyLister
}

func NewProxyObserveFunc(configPath []string) configobserver.ObserveConfigFunc {
	return (&observeProxyFlags{
		configPath: configPath,
	}).ObserveProxyConfig
}

type observeProxyFlags struct {
	configPath []string
}

// ObserveProxyConfig observes the proxy.config.openshift.io/cluster object and writes
// its content to an unstructured object in a string map at the path from the constructor
func (f *observeProxyFlags) ObserveProxyConfig(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	proxyLister := genericListers.(ProxyLister)

	errs := []error{}
	prevObservedProxyConfig := map[string]interface{}{}

	// grab the current Proxy config to later check whether it was updated
	currentProxyMap, _, err := unstructured.NestedStringMap(existingConfig, f.configPath...)
	if err != nil {
		return prevObservedProxyConfig, append(errs, err)
	}

	if len(currentProxyMap) > 0 {
		unstructured.SetNestedStringMap(prevObservedProxyConfig, currentProxyMap, f.configPath...)
	}

	observedConfig := map[string]interface{}{}
	proxyConfig, err := proxyLister.ProxyLister().Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("ObserveProxyConfig", "proxy.%s/cluster not found", configv1.GroupName)
		return observedConfig, errs
	}
	if err != nil {
		errs = append(errs, err)
		return existingConfig, errs
	}

	newProxyMap := proxyToMap(proxyConfig)
	if newProxyMap != nil {
		if err := unstructured.SetNestedStringMap(observedConfig, newProxyMap, f.configPath...); err != nil {
			errs = append(errs, err)
		}
	}

	if !reflect.DeepEqual(currentProxyMap, newProxyMap) {
		recorder.Eventf("ObserveProxyConfig", "proxy changed to %q", newProxyMap)
	}

	return observedConfig, errs
}

func proxyToMap(proxy *configv1.Proxy) map[string]string {
	proxyMap := map[string]string{}

	if noProxy := proxy.Status.NoProxy; len(noProxy) > 0 {
		proxyMap["NO_PROXY"] = noProxy
	}

	if httpProxy := proxy.Status.HTTPProxy; len(httpProxy) > 0 {
		proxyMap["HTTP_PROXY"] = httpProxy
	}

	if httpsProxy := proxy.Status.HTTPSProxy; len(httpsProxy) > 0 {
		proxyMap["HTTPS_PROXY"] = httpsProxy
	}

	if len(proxyMap) == 0 {
		return nil
	}

	return proxyMap
}
