package builder

import (
	"os"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/build/api"
)

// A KeyValue can be used to build ordered lists of key-value pairs.
type KeyValue struct {
	Key   string
	Value string
}

// buildInfo returns a slice of KeyValue pairs with build metadata to be
// inserted into Docker images produced by build.
func buildInfo(build *api.Build) []KeyValue {
	kv := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", build.Name},
		{"OPENSHIFT_BUILD_NAMESPACE", build.Namespace},
	}
	if build.Spec.Source.Git != nil {
		kv = append(kv, KeyValue{"OPENSHIFT_BUILD_SOURCE", build.Spec.Source.Git.URI})
		if build.Spec.Source.Git.Ref != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_REFERENCE", build.Spec.Source.Git.Ref})
		}
		if build.Spec.Revision != nil && build.Spec.Revision.Git != nil && build.Spec.Revision.Git.Commit != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_COMMIT", build.Spec.Revision.Git.Commit})
		}
	}
	if build.Spec.Strategy.Type == api.SourceBuildStrategyType {
		env := build.Spec.Strategy.SourceStrategy.Env
		for _, e := range env {
			kv = append(kv, KeyValue{e.Name, e.Value})
		}
	}
	return kv
}

// setHTTPProxy sets the system's environment variables to define the HTTP and
// HTTPS proxies to be used by commands that respect those variables, e.g., git
// clone performed to fetch source code to be built. It returns the original
// values of all environment variables that were set.
func setHTTPProxy(httpProxy, httpsProxy string) map[string]string {
	originalProxies := make(map[string]string, 4)
	if httpProxy != "" {
		glog.V(2).Infof("Setting HTTP_PROXY to %s", httpProxy)
		originalProxies["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		originalProxies["http_proxy"] = os.Getenv("http_proxy")
		os.Setenv("HTTP_PROXY", httpProxy)
		os.Setenv("http_proxy", httpProxy)
	}
	if httpsProxy != "" {
		glog.V(2).Infof("Setting HTTPS_PROXY to %s", httpsProxy)
		originalProxies["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		originalProxies["https_proxy"] = os.Getenv("https_proxy")
		os.Setenv("HTTPS_PROXY", httpsProxy)
		os.Setenv("https_proxy", httpsProxy)
	}
	return originalProxies
}

// resetHTTPProxy sets the system's environment variables defined in
// originalProxies. It should be used to undo the changes made by setHTTPProxy.
func resetHTTPProxy(originalProxies map[string]string) {
	if proxy, ok := originalProxies["HTTP_PROXY"]; ok {
		glog.V(4).Infof("Resetting HTTP_PROXY to %s", proxy)
		os.Setenv("HTTP_PROXY", proxy)
	}
	if proxy, ok := originalProxies["http_proxy"]; ok {
		glog.V(4).Infof("Resetting http_proxy to %s", proxy)
		os.Setenv("http_proxy", proxy)
	}
	if proxy, ok := originalProxies["HTTPS_PROXY"]; ok {
		glog.V(4).Infof("Resetting HTTPS_PROXY to %s", proxy)
		os.Setenv("HTTPS_PROXY", proxy)
	}
	if proxy, ok := originalProxies["https_proxy"]; ok {
		glog.V(4).Infof("Resetting https_proxy to %s", proxy)
		os.Setenv("https_proxy", proxy)
	}
}
