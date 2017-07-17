package hybrid

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api"
	apihelper "k8s.io/kubernetes/pkg/api/helper"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	"k8s.io/kubernetes/pkg/proxy"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
	"k8s.io/kubernetes/pkg/proxy/userspace"
)

// EndpointsConfigHandler is an abstract interface of objects which receive update notifications for the set of endpoints.
type EndpointsConfigHandler interface {
	// OnEndpointsUpdate gets called when endpoints configuration is changed for a given
	// service on any of the configuration sources. An example is when a new
	// service comes up, or when containers come up or down for an existing service.
	OnEndpointsUpdate(endpoints []*api.Endpoints)
}

// ServiceConfigHandler is an abstract interface of objects which receive update notifications for the set of services.
type ServiceConfigHandler interface {
	// OnServiceUpdate gets called when a configuration has been changed by one of the sources.
	// This is the union of all the configuration sources.
	OnServiceUpdate(services []*api.Service)
}

type HybridProxier struct {
	unidlingProxy        *userspace.Proxier
	unidlingLoadBalancer EndpointsConfigHandler
	mainEndpointsHandler EndpointsConfigHandler
	mainServicesHandler  ServiceConfigHandler
	mainProxy            proxy.ProxyProvider
	syncPeriod           time.Duration
	serviceLister        kcorelisters.ServiceLister

	// TODO(directxman12): figure out a good way to avoid duplicating this information
	// (it's saved in the individual proxies as well)
	usingUserspace     map[types.NamespacedName]struct{}
	usingUserspaceLock sync.Mutex
}

func NewHybridProxier(
	unidlingLoadBalancer EndpointsConfigHandler,
	unidlingProxy *userspace.Proxier,
	mainEndpointsHandler EndpointsConfigHandler,
	mainProxy proxy.ProxyProvider,
	mainServicesHandler ServiceConfigHandler,
	syncPeriod time.Duration,
	serviceLister kcorelisters.ServiceLister,
) (*HybridProxier, error) {
	return &HybridProxier{
		unidlingLoadBalancer: unidlingLoadBalancer,
		unidlingProxy:        unidlingProxy,
		mainEndpointsHandler: mainEndpointsHandler,
		mainProxy:            mainProxy,
		mainServicesHandler:  mainServicesHandler,
		syncPeriod:           syncPeriod,
		serviceLister:        serviceLister,

		usingUserspace: nil,
	}, nil
}

func (p *HybridProxier) OnServiceUpdate(services []*api.Service) {
	forIPTables := make([]*api.Service, 0, len(services))
	forUserspace := []*api.Service{}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	for _, service := range services {
		if !apihelper.IsServiceIPSet(service) {
			// Skip service with no ClusterIP set
			continue
		}
		svcName := types.NamespacedName{
			Namespace: service.Namespace,
			Name:      service.Name,
		}
		if _, ok := p.usingUserspace[svcName]; ok {
			forUserspace = append(forUserspace, service)
		} else {
			forIPTables = append(forIPTables, service)
		}
	}

	p.unidlingProxy.OnServiceUpdate(forUserspace)
	p.mainServicesHandler.OnServiceUpdate(forIPTables)
}

func (p *HybridProxier) updateUsingUserspace(endpoints []*api.Endpoints) {
	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	p.usingUserspace = make(map[types.NamespacedName]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		hasEndpoints := false
		for _, subset := range endpoint.Subsets {
			if len(subset.Addresses) > 0 {
				hasEndpoints = true
				break
			}
		}

		if !hasEndpoints {
			if _, ok := endpoint.Annotations[unidlingapi.IdledAtAnnotation]; ok {
				svcName := types.NamespacedName{
					Namespace: endpoint.Namespace,
					Name:      endpoint.Name,
				}
				p.usingUserspace[svcName] = struct{}{}
			}
		}
	}
}

func (p *HybridProxier) getIPTablesEndpoints(endpoints []*api.Endpoints) []*api.Endpoints {
	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	forIPTables := []*api.Endpoints{}
	for _, endpoint := range endpoints {
		svcName := types.NamespacedName{
			Namespace: endpoint.Namespace,
			Name:      endpoint.Name,
		}
		if _, ok := p.usingUserspace[svcName]; !ok {
			forIPTables = append(forIPTables, endpoint)
		}
	}
	return forIPTables
}

func (p *HybridProxier) OnEndpointsUpdate(endpoints []*api.Endpoints) {
	p.updateUsingUserspace(endpoints)

	p.unidlingLoadBalancer.OnEndpointsUpdate(endpoints)

	forIPTables := p.getIPTablesEndpoints(endpoints)
	p.mainEndpointsHandler.OnEndpointsUpdate(forIPTables)

	services, err := p.serviceLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error while listing services from cache: %v", err))
		return
	}
	if services == nil {
		services = []*api.Service{}
	}
	p.OnServiceUpdate(services)
}

// Sync is called to immediately synchronize the proxier state to iptables
func (p *HybridProxier) Sync() {
	p.mainProxy.Sync()
	p.unidlingProxy.Sync()
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (p *HybridProxier) SyncLoop() {
	t := time.NewTicker(p.syncPeriod)
	defer t.Stop()
	for {
		<-t.C
		glog.V(6).Infof("Periodic sync")
		p.mainProxy.Sync()
		p.unidlingProxy.Sync()
	}
}
