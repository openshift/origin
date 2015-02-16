package kubernetes

import (
	"net/url"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/util/httpproxy"
)

type ProxyConfig struct {
	KubernetesAddr *url.URL
	ClientConfig   *kclient.Config
}

func (c *ProxyConfig) InstallAPI(container *restful.Container) []string {
	proxy, err := httpproxy.NewUpgradeAwareSingleHostReverseProxy(c.ClientConfig, c.KubernetesAddr)
	if err != nil {
		glog.Fatalf("Unable to initialize the Kubernetes proxy: %v", err)
	}

	container.Handle("/api/", proxy)

	return []string{
		"Started Kubernetes proxy at %s/api/",
	}
}
