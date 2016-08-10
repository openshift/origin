package hybrid

import (
	"time"

	"github.com/openshift/origin/pkg/proxy/userspace"
	unidlingapi "github.com/openshift/origin/pkg/unidling/api"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/proxy"
	"k8s.io/kubernetes/pkg/proxy/config"
	"k8s.io/kubernetes/pkg/types"

	"github.com/golang/glog"
)

type HybridProxier struct {
	unidlingProxy        *userspace.Proxier
	unidlingLoadBalancer config.EndpointsConfigHandler
	mainProxy            proxy.ProxyProvider
	mainLoadBalancer     config.EndpointsConfigHandler

	serviceConfig *config.ServiceConfig

	// TODO(directxman12): figure out a good way to avoid duplicating this information
	// (it's saved in the individual proxies as well)
	usingUserspace map[types.NamespacedName]struct{}

	syncPeriod time.Duration
}

func NewHybridProxier(unidlingLoadBalancer config.EndpointsConfigHandler, unidlingProxy *userspace.Proxier, mainLoadBalancer config.EndpointsConfigHandler, mainProxy proxy.ProxyProvider, syncPeriod time.Duration, serviceConfig *config.ServiceConfig) (*HybridProxier, error) {
	return &HybridProxier{
		unidlingProxy:        unidlingProxy,
		mainProxy:            mainProxy,
		unidlingLoadBalancer: unidlingLoadBalancer,
		mainLoadBalancer:     mainLoadBalancer,

		serviceConfig: serviceConfig,

		usingUserspace: nil,

		syncPeriod: syncPeriod,
	}, nil
}

func (p *HybridProxier) OnServiceUpdate(services []api.Service) {
	forIPTables := make([]api.Service, 0, len(services))
	forUserspace := []api.Service{}

	for _, service := range services {
		if !api.IsServiceIPSet(&service) {
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
	p.mainProxy.OnServiceUpdate(forIPTables)
}

func (p *HybridProxier) updateUsingUserspace(endpoints []api.Endpoints) {
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

func (p *HybridProxier) OnEndpointsUpdate(endpoints []api.Endpoints) {
	p.updateUsingUserspace(endpoints)

	forIPTables := []api.Endpoints{}

	for _, endpoint := range endpoints {
		svcName := types.NamespacedName{
			Namespace: endpoint.Namespace,
			Name:      endpoint.Name,
		}
		if _, ok := p.usingUserspace[svcName]; !ok {
			forIPTables = append(forIPTables, endpoint)
		}
	}

	p.unidlingLoadBalancer.OnEndpointsUpdate(endpoints)
	p.mainLoadBalancer.OnEndpointsUpdate(forIPTables)

	p.OnServiceUpdate(p.serviceConfig.Config())
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
