package kubernetes

import (
	"fmt"
	"net/url"

	"k8s.io/kubernetes/pkg/client/restclient"

	restful "github.com/emicklei/go-restful"

	"github.com/openshift/origin/pkg/util/httpproxy"
)

type ProxyConfig struct {
	ClientConfig *restclient.Config
	Component    string
	Path         string
	URLRewriteFn func(*url.URL)
}

func (c *ProxyConfig) InstallAPI(container *restful.Container) ([]string, error) {
	kubeAddr, err := url.Parse(c.ClientConfig.Host)
	if err != nil {
		return nil, err
	}

	proxy, err := httpproxy.NewUpgradeAwareSingleHostReverseProxy(c.ClientConfig, kubeAddr, c.URLRewriteFn)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize the Kubernetes proxy: %v", err)
	}

	container.Handle(c.Path, proxy)

	return []string{
		fmt.Sprintf("Started %s proxy at %%s%s", c.Component, c.Path),
	}, nil
}
