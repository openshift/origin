package backenddisruption

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

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

	hostGetterLock sync.Mutex
	// host is the https://host:port part of the URL
	host atomic.Value
}

func NewRouteHostGetter(clientConfig *rest.Config, routeNamespace string, routeName string) HostGetter {
	return &routeHostGetter{
		clientConfig:   clientConfig,
		routeNamespace: routeNamespace,
		routeName:      routeName,
	}
}

func (g *routeHostGetter) GetHost() (string, error) {
	existingHost := g.host.Load()
	if existingHost != nil {
		host := existingHost.(string)
		if len(host) > 0 {
			return host, nil
		}
	}
	g.hostGetterLock.Lock()
	defer g.hostGetterLock.Unlock()
	client, err := routeclientset.NewForConfig(g.clientConfig)
	if err != nil {
		return "", err
	}
	route, err := client.RouteV1().Routes(g.routeNamespace).Get(context.Background(), g.routeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, ingress := range route.Status.Ingress {
		if len(ingress.Host) > 0 {
			host := fmt.Sprintf("https://%s", ingress.Host)
			g.host.Store(host)
			return host, nil
		}
	}

	return "", fmt.Errorf("missing in route")
}
