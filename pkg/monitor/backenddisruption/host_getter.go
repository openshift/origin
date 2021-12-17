package backenddisruption

import (
	"context"
	"fmt"
	"sync"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type HostGetter interface {
	GetHost() (string, error)
}

type SimpleHostGetter struct {
	lock sync.Mutex
	host string
}

func NewSimpleHostGetter(host string) *SimpleHostGetter {
	return &SimpleHostGetter{
		host: host,
	}
}

func (g *SimpleHostGetter) GetHost() (string, error) {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.host, nil
}

func (g *SimpleHostGetter) SetHost(host string) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.host = host
}

type kubeAPIHostGetter struct {
	clientConfig *rest.Config
}

func NewKubeAPIHostGetter(clientConfig *rest.Config) HostGetter {
	return &kubeAPIHostGetter{
		clientConfig: clientConfig,
	}
}

func (g *kubeAPIHostGetter) GetHost() (string, error) {
	return g.clientConfig.Host, nil
}

type routeHostGetter struct {
	clientConfig   *rest.Config
	routeNamespace string
	routeName      string

	// initializeHost is used to ensure we only look up the route once instead of on every request
	initializeHost sync.Once
	// host is the https://host:port part of the URL
	host string
	// hostErr is the error (if we got one) from initializeHost.  This easier than retrying if we fail and probably
	// good enough for CI.
	hostErr error
}

func NewRouteHostGetter(clientConfig *rest.Config, routeNamespace string, routeName string) HostGetter {
	return &routeHostGetter{
		clientConfig:   clientConfig,
		routeNamespace: routeNamespace,
		routeName:      routeName,
	}
}

func (g *routeHostGetter) GetHost() (string, error) {
	g.initializeHost.Do(func() {
		client, err := routeclientset.NewForConfig(g.clientConfig)
		if err != nil {
			g.hostErr = err
			return
		}
		route, err := client.RouteV1().Routes(g.routeNamespace).Get(context.Background(), g.routeName, metav1.GetOptions{})
		if err != nil {
			g.hostErr = err
			return
		}
		for _, ingress := range route.Status.Ingress {
			if len(ingress.Host) > 0 {
				g.host = fmt.Sprintf("https://%s", ingress.Host)
				break
			}
		}
	})

	if g.hostErr != nil {
		return "", g.hostErr
	}
	if len(g.host) == 0 {
		return "", fmt.Errorf("missing URL")
	}
	return g.host, nil
}
