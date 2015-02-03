package kubernetes

import (
	"net/http/httputil"
	"net/url"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"
)

type ProxyConfig struct {
	KubernetesAddr *url.URL
	ClientConfig   *kclient.Config
}

func (c *ProxyConfig) InstallAPI(container *restful.Container) []string {
	transport, err := kclient.TransportFor(c.ClientConfig)
	if err != nil {
		glog.Fatalf("Unable to initialize the Kubernetes proxy: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(c.KubernetesAddr)
	proxy.Transport = transport
	container.Handle("/api/", proxy)

	return []string{
		"Started Kubernetes proxy at %s/api/",
	}
}
