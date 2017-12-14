package hybrid

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	api "k8s.io/kubernetes/pkg/apis/core"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	"k8s.io/kubernetes/pkg/proxy"
	proxyconfig "k8s.io/kubernetes/pkg/proxy/config"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
)

// HybridProxier runs an unidling proxy and a primary proxy at the same time,
// delegating idled services to the unidling proxy and other services to the
// primary proxy.
type HybridProxier struct {
	unidlingServiceHandler   proxyconfig.ServiceHandler
	unidlingEndpointsHandler proxyconfig.EndpointsHandler
	mainEndpointsHandler     proxyconfig.EndpointsHandler
	mainServicesHandler      proxyconfig.ServiceHandler
	mainProxy                proxy.ProxyProvider
	unidlingProxy            proxy.ProxyProvider
	syncPeriod               time.Duration
	serviceLister            kcorelisters.ServiceLister

	// TODO(directxman12): figure out a good way to avoid duplicating this information
	// (it's saved in the individual proxies as well)
	// usingUserspace is *NOT* a set -- we care about the value, and use it to keep track of
	// when we need to delete from an existing proxier when adding to a new one.
	usingUserspace     map[types.NamespacedName]bool
	usingUserspaceLock sync.Mutex
}

func NewHybridProxier(
	unidlingEndpointsHandler proxyconfig.EndpointsHandler,
	unidlingServiceHandler proxyconfig.ServiceHandler,
	mainEndpointsHandler proxyconfig.EndpointsHandler,
	mainServicesHandler proxyconfig.ServiceHandler,
	mainProxy proxy.ProxyProvider,
	unidlingProxy proxy.ProxyProvider,
	syncPeriod time.Duration,
	serviceLister kcorelisters.ServiceLister,
) (*HybridProxier, error) {
	return &HybridProxier{
		unidlingEndpointsHandler: unidlingEndpointsHandler,
		unidlingServiceHandler:   unidlingServiceHandler,
		mainEndpointsHandler:     mainEndpointsHandler,
		mainServicesHandler:      mainServicesHandler,
		mainProxy:                mainProxy,
		unidlingProxy:            unidlingProxy,
		syncPeriod:               syncPeriod,
		serviceLister:            serviceLister,

		usingUserspace: make(map[types.NamespacedName]bool),
	}, nil
}

func (p *HybridProxier) OnServiceAdd(service *api.Service) {
	svcName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	// since this is an Add, we know the service isn't already in another
	// proxy, so don't bother trying to remove like on an update
	if isUsingUserspace, ok := p.usingUserspace[svcName]; ok && isUsingUserspace {
		glog.V(6).Infof("hybrid proxy: add svc %s in unidling proxy", service.Name)
		p.unidlingServiceHandler.OnServiceAdd(service)
	} else {
		glog.V(6).Infof("hybrid proxy: add svc %s in main proxy", service.Name)
		p.mainServicesHandler.OnServiceAdd(service)
	}
}

func (p *HybridProxier) OnServiceUpdate(oldService, service *api.Service) {
	svcName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	// NB: usingUserspace can only change in the endpoints handler,
	// so that should deal with calling OnServiceDelete on switches
	if isUsingUserspace, ok := p.usingUserspace[svcName]; ok && isUsingUserspace {
		glog.V(6).Infof("hybrid proxy: update svc %s in unidling proxy", service.Name)
		p.unidlingServiceHandler.OnServiceUpdate(oldService, service)
	} else {
		glog.V(6).Infof("hybrid proxy: update svc %s in main proxy", service.Name)
		p.mainServicesHandler.OnServiceUpdate(oldService, service)
	}
}

func (p *HybridProxier) OnServiceDelete(service *api.Service) {
	svcName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	if isUsingUserspace, ok := p.usingUserspace[svcName]; ok && isUsingUserspace {
		glog.V(6).Infof("hybrid proxy: del svc %s in unidling proxy", service.Name)
		p.unidlingServiceHandler.OnServiceDelete(service)
	} else {
		glog.V(6).Infof("hybrid proxy: del svc %s in main proxy", service.Name)
		p.mainServicesHandler.OnServiceDelete(service)
	}
}

func (p *HybridProxier) OnServiceSynced() {
	p.unidlingServiceHandler.OnServiceSynced()
	p.mainServicesHandler.OnServiceSynced()
	glog.V(6).Infof("hybrid proxy: services synced")
}

// shouldEndpointsUseUserspace checks to see if the given endpoints have the correct
// annotations and size to use the unidling proxy.
func (p *HybridProxier) shouldEndpointsUseUserspace(endpoints *api.Endpoints) bool {
	hasEndpoints := false
	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			hasEndpoints = true
			break
		}
	}

	if !hasEndpoints {
		if _, ok := endpoints.Annotations[unidlingapi.IdledAtAnnotation]; ok {
			return true
		}
	}

	return false
}

func (p *HybridProxier) switchService(name types.NamespacedName) {
	svc, err := p.serviceLister.Services(name.Namespace).Get(name.Name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error while getting service %s from cache: %v", name.String(), err))
		return
	}

	if p.usingUserspace[name] {
		glog.V(6).Infof("hybrid proxy: switching svc %s to unidling proxy", svc.Name)
		p.unidlingServiceHandler.OnServiceAdd(svc)
		p.mainServicesHandler.OnServiceDelete(svc)
	} else {
		glog.V(6).Infof("hybrid proxy: switching svc %s to main proxy", svc.Name)
		p.mainServicesHandler.OnServiceAdd(svc)
		p.unidlingServiceHandler.OnServiceDelete(svc)
	}
}

func (p *HybridProxier) OnEndpointsAdd(endpoints *api.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	glog.V(6).Infof("hybrid proxy: (always) add ep %s in unidling proxy", endpoints.Name)
	p.unidlingEndpointsHandler.OnEndpointsAdd(endpoints)

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	svcName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	wasUsingUserspace, knownEndpoints := p.usingUserspace[svcName]
	p.usingUserspace[svcName] = p.shouldEndpointsUseUserspace(endpoints)

	if !p.usingUserspace[svcName] {
		glog.V(6).Infof("hybrid proxy: add ep %s in main proxy", endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsAdd(endpoints)
	}

	// a service could appear before endpoints, so we have to treat this as a potential
	// state modification for services, and not just an addition (since we could flip proxies).
	if knownEndpoints && wasUsingUserspace != p.usingUserspace[svcName] {
		p.switchService(svcName)
	}
}

func (p *HybridProxier) OnEndpointsUpdate(oldEndpoints, endpoints *api.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	glog.V(6).Infof("hybrid proxy: (always) update ep %s in unidling proxy", endpoints.Name)
	p.unidlingEndpointsHandler.OnEndpointsUpdate(oldEndpoints, endpoints)

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	svcName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	wasUsingUserspace, knownEndpoints := p.usingUserspace[svcName]
	p.usingUserspace[svcName] = p.shouldEndpointsUseUserspace(endpoints)

	if !knownEndpoints {
		utilruntime.HandleError(fmt.Errorf("received update for unknown endpoints %s", svcName.String()))
		return
	}

	isSwitch := wasUsingUserspace != p.usingUserspace[svcName]

	if !isSwitch && !p.usingUserspace[svcName] {
		glog.V(6).Infof("hybrid proxy: update ep %s in main proxy", endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsUpdate(oldEndpoints, endpoints)
		return
	}

	if p.usingUserspace[svcName] {
		glog.V(6).Infof("hybrid proxy: del ep %s in main proxy", endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsDelete(oldEndpoints)
	} else {
		glog.V(6).Infof("hybrid proxy: add ep %s in main proxy", endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsAdd(endpoints)
	}

	p.switchService(svcName)
}

func (p *HybridProxier) OnEndpointsDelete(endpoints *api.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	glog.V(6).Infof("hybrid proxy: (always) del ep %s in unidling proxy", endpoints.Name)
	p.unidlingEndpointsHandler.OnEndpointsDelete(endpoints)

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	svcName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	usingUserspace, knownEndpoints := p.usingUserspace[svcName]

	if !knownEndpoints {
		utilruntime.HandleError(fmt.Errorf("received delete for unknown endpoints %s", svcName.String()))
		return
	}

	if !usingUserspace {
		glog.V(6).Infof("hybrid proxy: del ep %s in main proxy", endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsDelete(endpoints)
	}

	delete(p.usingUserspace, svcName)
}

func (p *HybridProxier) OnEndpointsSynced() {
	p.unidlingEndpointsHandler.OnEndpointsSynced()
	p.mainEndpointsHandler.OnEndpointsSynced()
	glog.V(6).Infof("hybrid proxy: endpoints synced")
}

// Sync is called to immediately synchronize the proxier state to iptables
func (p *HybridProxier) Sync() {
	p.mainProxy.Sync()
	p.unidlingProxy.Sync()
	glog.V(6).Infof("hybrid proxy: proxies synced")
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (p *HybridProxier) SyncLoop() {
	// the iptables proxier now lies about how it works.  sync doesn't actually sync now --
	// it just adds to a queue that's processed by a loop launched by SyncLoop, so we
	// *must* start the sync loops, and not just use our own...
	go p.mainProxy.SyncLoop()
	go p.unidlingProxy.SyncLoop()

	select {}
}
