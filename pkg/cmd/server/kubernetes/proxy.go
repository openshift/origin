package kubernetes

import (
	"net/url"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"
	kclient "k8s.io/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/util/httpproxy"
)

type ProxyConfig struct {
	ClientConfig *kclient.Config
}

func (c *ProxyConfig) InstallAPI(container *restful.Container) []string {
	kubeAddr, err := url.Parse(c.ClientConfig.Host)
	if err != nil {
		glog.Fatal(err)
	}

	proxy, err := httpproxy.NewUpgradeAwareSingleHostReverseProxy(c.ClientConfig, kubeAddr)
	if err != nil {
		glog.Fatalf("Unable to initialize the Kubernetes proxy: %v", err)
	}

	container.Handle("/api/", proxy)

	return []string{
		"Started Kubernetes proxy at %s/api/",
	}
}
