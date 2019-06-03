package hybrid

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/proxy"
	proxyconfig "k8s.io/kubernetes/pkg/proxy/config"

	unidlingapi "github.com/openshift/api/unidling/v1alpha1"
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
	serviceLister            corev1listers.ServiceLister

	// TODO(directxman12): figure out a good way to avoid duplicating this information
	// (it's saved in the individual proxies as well)
	// usingUserspace is *NOT* a set -- we care about the value, and use it to keep track of
	// when we need to delete from an existing proxier when adding to a new one.
	usingUserspace     map[types.NamespacedName]bool
	usingUserspaceLock sync.Mutex

	// There are some bugs where we can call switchService() multiple times
	// even though we don't actually want to switch. This calls OnServiceDelete()
	// multiple times for the underlying proxies, which causes bugs.
	// See bz 1635330
	// So, add an additional state store to ensure we only switch once
	switchedToUserspace     map[types.NamespacedName]bool
	switchedToUserspaceLock sync.Mutex
}

func NewHybridProxier(
	unidlingEndpointsHandler proxyconfig.EndpointsHandler,
	unidlingServiceHandler proxyconfig.ServiceHandler,
	mainEndpointsHandler proxyconfig.EndpointsHandler,
	mainServicesHandler proxyconfig.ServiceHandler,
	mainProxy proxy.ProxyProvider,
	unidlingProxy proxy.ProxyProvider,
	syncPeriod time.Duration,
	serviceLister corev1listers.ServiceLister,
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

		usingUserspace:      make(map[types.NamespacedName]bool),
		switchedToUserspace: make(map[types.NamespacedName]bool),
	}, nil
}

func (p *HybridProxier) OnServiceAdd(service *corev1.Service) {
	svcName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	// since this is an Add, we know the service isn't already in another
	// proxy, so don't bother trying to remove like on an update
	if isUsingUserspace, ok := p.usingUserspace[svcName]; ok && isUsingUserspace {
		klog.V(6).Infof("hybrid proxy: add svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingServiceHandler.OnServiceAdd(service)
	} else {
		klog.V(6).Infof("hybrid proxy: add svc %s/%s in main proxy", service.Namespace, service.Name)
		p.mainServicesHandler.OnServiceAdd(service)
	}
}

func (p *HybridProxier) OnServiceUpdate(oldService, service *corev1.Service) {
	svcName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	// NB: usingUserspace can only change in the endpoints handler,
	// so that should deal with calling OnServiceDelete on switches
	if isUsingUserspace, ok := p.usingUserspace[svcName]; ok && isUsingUserspace {
		klog.V(6).Infof("hybrid proxy: update svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingServiceHandler.OnServiceUpdate(oldService, service)
	} else {
		klog.V(6).Infof("hybrid proxy: update svc %s/%s in main proxy", service.Namespace, service.Name)
		p.mainServicesHandler.OnServiceUpdate(oldService, service)
	}
}

func (p *HybridProxier) OnServiceDelete(service *corev1.Service) {
	svcName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	// Careful, we always need to get this lock after usingUserspace, or else we could deadlock
	p.switchedToUserspaceLock.Lock()
	defer p.switchedToUserspaceLock.Unlock()

	if isUsingUserspace, ok := p.usingUserspace[svcName]; ok && isUsingUserspace {
		klog.V(6).Infof("hybrid proxy: del svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingServiceHandler.OnServiceDelete(service)
	} else {
		klog.V(6).Infof("hybrid proxy: del svc %s/%s in main proxy", service.Namespace, service.Name)
		p.mainServicesHandler.OnServiceDelete(service)
	}

	delete(p.switchedToUserspace, svcName)
}

func (p *HybridProxier) OnServiceSynced() {
	p.unidlingServiceHandler.OnServiceSynced()
	p.mainServicesHandler.OnServiceSynced()
	klog.V(6).Infof("hybrid proxy: services synced")
}

// shouldEndpointsUseUserspace checks to see if the given endpoints have the correct
// annotations and size to use the unidling proxy.
func (p *HybridProxier) shouldEndpointsUseUserspace(endpoints *corev1.Endpoints) bool {
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

// switchService moves a service between the unidling and main proxies.
func (p *HybridProxier) switchService(name types.NamespacedName) {
	// We shouldn't call switchService more than once (per switch), but there
	// are some logic bugs where this happens
	// So, cache the real state and don't allow this to be called twice.
	// This assumes the caller already holds usingUserspaceLock
	p.switchedToUserspaceLock.Lock()
	defer p.switchedToUserspaceLock.Unlock()

	switched, ok := p.switchedToUserspace[name]
	if ok && p.usingUserspace[name] == switched {
		klog.V(6).Infof("hybrid proxy: ignoring duplicate switchService(%s/%s)", name.Namespace, name.Name)
		return
	}

	svc, err := p.serviceLister.Services(name.Namespace).Get(name.Name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error while getting service %s/%s from cache: %v", name.Namespace, name.String(), err))
		return
	}

	if p.usingUserspace[name] {
		klog.V(6).Infof("hybrid proxy: switching svc %s/%s to unidling proxy", svc.Namespace, svc.Name)
		p.unidlingServiceHandler.OnServiceAdd(svc)
		p.mainServicesHandler.OnServiceDelete(svc)
	} else {
		klog.V(6).Infof("hybrid proxy: switching svc %s/%s to main proxy", svc.Namespace, svc.Name)
		p.mainServicesHandler.OnServiceAdd(svc)
		p.unidlingServiceHandler.OnServiceDelete(svc)
	}

	p.switchedToUserspace[name] = p.usingUserspace[name]
}

func (p *HybridProxier) OnEndpointsAdd(endpoints *corev1.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	klog.V(6).Infof("hybrid proxy: (always) add ep %s/%s in unidling proxy", endpoints.Namespace, endpoints.Name)
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
		klog.V(6).Infof("hybrid proxy: add ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsAdd(endpoints)
	}

	// a service could appear before endpoints, so we have to treat this as a potential
	// state modification for services, and not just an addition (since we could flip proxies).
	if knownEndpoints && wasUsingUserspace != p.usingUserspace[svcName] {
		p.switchService(svcName)
	}
}

func (p *HybridProxier) OnEndpointsUpdate(oldEndpoints, endpoints *corev1.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	klog.V(6).Infof("hybrid proxy: (always) update ep %s/%s in unidling proxy", endpoints.Namespace, endpoints.Name)
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
		klog.V(6).Infof("hybrid proxy: update ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsUpdate(oldEndpoints, endpoints)
		return
	}

	if p.usingUserspace[svcName] {
		klog.V(6).Infof("hybrid proxy: del ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsDelete(oldEndpoints)
	} else {
		klog.V(6).Infof("hybrid proxy: add ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsAdd(endpoints)
	}

	p.switchService(svcName)
}

func (p *HybridProxier) OnEndpointsDelete(endpoints *corev1.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	klog.V(6).Infof("hybrid proxy: (always) del ep %s/%s in unidling proxy", endpoints.Namespace, endpoints.Name)
	p.unidlingEndpointsHandler.OnEndpointsDelete(endpoints)

	// Careful - there is the potential for deadlocks here,
	// except that we always get usingUserspaceLock first, then
	// get switchedToUserspaceLock.
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
		klog.V(6).Infof("hybrid proxy: del ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsDelete(endpoints)
	}

	delete(p.usingUserspace, svcName)
}

func (p *HybridProxier) OnEndpointsSynced() {
	p.unidlingEndpointsHandler.OnEndpointsSynced()
	p.mainEndpointsHandler.OnEndpointsSynced()
	klog.V(6).Infof("hybrid proxy: endpoints synced")
}

// Sync is called to immediately synchronize the proxier state to iptables
func (p *HybridProxier) Sync() {
	p.mainProxy.Sync()
	p.unidlingProxy.Sync()
	klog.V(6).Infof("hybrid proxy: proxies synced")
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
