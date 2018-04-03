package hybrid

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	"k8s.io/kubernetes/pkg/proxy"
	proxyconfig "k8s.io/kubernetes/pkg/proxy/config"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"

	idlingutil "github.com/openshift/origin/pkg/idling"
)

// WaitIndicator knows how to mark that a condition is false in one location,
// and then wait until it's true in another.
type WaitIndicator interface {
	// Reset resets the condition to be unmet.
	Reset()
	// Wait waits until the condition has been met.
	Wait()
}

// healthzCondWaitIndicator's condition is that the timestamp on a healthz checker is updated
type healthzWaitIndicator struct {
	// forwardTo forwards healthz checks to another updater
	forwardTo healthcheck.HealthzUpdater

	condMet bool
	cond    *sync.Cond
	mu      *sync.RWMutex
}

func (i *healthzWaitIndicator) UpdateTimestamp() {
	if i.forwardTo != nil {
		i.forwardTo.UpdateTimestamp()
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	i.condMet = true
	i.cond.Broadcast()
}
func (i *healthzWaitIndicator) Reset() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.condMet = false
}

func (i *healthzWaitIndicator) Wait() {
	i.cond.L.Lock()
	for !i.condMet {
		i.cond.Wait()
	}
	i.cond.L.Unlock()
}

func NewHealthzWaitIndicator(forwardTo healthcheck.HealthzUpdater) (WaitIndicator, healthcheck.HealthzUpdater) {
	ind := &healthzWaitIndicator{
		forwardTo: forwardTo,
		mu:        new(sync.RWMutex),
	}
	ind.cond = sync.NewCond(ind.mu.RLocker())
	return ind, ind
}

// HybridProxier runs an unidling proxy and a primary proxy at the same time,
// delegating idled services to the unidling proxy and other services to the
// primary proxy.  It should be registered as handler on an idler informer.
type HybridProxier struct {
	unidlingServiceHandler proxyconfig.ServiceHandler
	mainEndpointsHandler   proxyconfig.EndpointsHandler
	mainServiceHandler     proxyconfig.ServiceHandler
	mainProxy              proxy.ProxyProvider
	syncPeriod             time.Duration
	lookup                 idlingutil.IdlerServiceLookup
}

func NewHybridProxier(
	unidlingServiceHandler proxyconfig.ServiceHandler,
	mainEndpointsHandler proxyconfig.EndpointsHandler,
	mainServiceHandler proxyconfig.ServiceHandler,
	mainProxy proxy.ProxyProvider,
	syncPeriod time.Duration,
	lookup idlingutil.IdlerServiceLookup,
) (*HybridProxier, error) {
	return &HybridProxier{
		unidlingServiceHandler: unidlingServiceHandler,
		mainEndpointsHandler:   mainEndpointsHandler,
		mainServiceHandler:     mainServiceHandler,
		mainProxy:              mainProxy,
		syncPeriod:             syncPeriod,
		lookup:                 lookup,
	}, nil
}

func (p *HybridProxier) isIdled(svc types.NamespacedName) bool {
	idler, present, err := p.lookup.IdlerByService(svc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to check for idlers for service %s: %v", svc.String(), err))
	}
	if !present {
		return false
	}

	return idler.Status.Idled
}

// NB: there's no need to remove the service from the main proxy when idling,
// since it will naturally have no endpoints, when idled.  Not performing
// the switch makes the code a bit simpler, and leads to a faster turn-around
// time when we get endpoints back.

func (p *HybridProxier) OnServiceAdd(service *api.Service) {
	useUnidling := p.isIdled(types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	})

	glog.V(6).Infof("hybrid proxy: (always) add svc %s/%s in main proxy", service.Namespace, service.Name)
	p.mainServiceHandler.OnServiceAdd(service)

	if useUnidling {
		glog.V(6).Infof("hybrid proxy: add svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingServiceHandler.OnServiceAdd(service)
	}
}

func (p *HybridProxier) OnServiceUpdate(oldService, service *api.Service) {
	useUnidling := p.isIdled(types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	})
	glog.V(6).Infof("hybrid proxy: (always) update svc %s/%s in main proxy", service.Namespace, service.Name)
	p.mainServiceHandler.OnServiceUpdate(oldService, service)

	// NB: useUnidling can only change in the Idler handler,
	// so that should deal with removing services from the idling proxy.
	if useUnidling {
		glog.V(6).Infof("hybrid proxy: update svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingServiceHandler.OnServiceUpdate(oldService, service)
	}
}

func (p *HybridProxier) OnServiceDelete(service *api.Service) {
	useUnidling := p.isIdled(types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	})

	glog.V(6).Infof("hybrid proxy: (always) del svc %s/%s in main proxy", service.Namespace, service.Name)
	p.mainServiceHandler.OnServiceDelete(service)

	if useUnidling {
		glog.V(6).Infof("hybrid proxy: del svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingServiceHandler.OnServiceDelete(service)
	}
}

func (p *HybridProxier) OnServiceSynced() {
	p.unidlingServiceHandler.OnServiceSynced()
	p.mainServiceHandler.OnServiceSynced()
	glog.V(6).Infof("hybrid proxy: services synced")
}

func (p *HybridProxier) OnEndpointsAdd(endpoints *api.Endpoints) {
	useUnidling := p.isIdled(types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	})

	if !useUnidling {
		glog.V(6).Infof("hybrid proxy: add ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsAdd(endpoints)
	}
	// we never need to add endpoints to the unidler proxy, since it doesn't care
}

func (p *HybridProxier) OnEndpointsUpdate(oldEndpoints, endpoints *api.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	useUnidling := p.isIdled(types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	})

	if !useUnidling {
		glog.V(6).Infof("hybrid proxy: update ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsUpdate(oldEndpoints, endpoints)
	}
	// we never need to update endpoints in the unidler proxy, since it doesn't care
}

func (p *HybridProxier) OnEndpointsDelete(endpoints *api.Endpoints) {
	useUnidling := p.isIdled(types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	})

	if !useUnidling {
		glog.V(6).Infof("hybrid proxy: del ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainEndpointsHandler.OnEndpointsDelete(endpoints)
	}
	// we never need to delete endpoints in the unidler proxy, since it doesn't care
}

func (p *HybridProxier) OnEndpointsSynced() {
	p.mainEndpointsHandler.OnEndpointsSynced()
	glog.V(6).Infof("hybrid proxy: endpoints synced")
}

// Sync is called to immediately synchronize the proxier state to iptables
func (p *HybridProxier) Sync() {
	p.mainProxy.Sync()
	glog.V(6).Infof("hybrid proxy: proxies synced")
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (p *HybridProxier) SyncLoop() {
	// the iptables proxier now lies about how it works.  sync doesn't actually sync now --
	// it just adds to a queue that's processed by a loop launched by SyncLoop, so we
	// *must* start the sync loops, and not just use our own...
	go p.mainProxy.SyncLoop()

	select {}
}

func (p *HybridProxier) IdlerEventHandlers(serviceLister kcorelisters.ServiceLister, endpointsLister kcorelisters.EndpointsLister, waiter WaitIndicator) cache.ResourceEventHandler {
	return &idlerChangeHandler{
		mainEndpointsHandler:   p.mainEndpointsHandler,
		mainServiceHandler:     p.mainServiceHandler,
		mainProxy:              p.mainProxy,
		unidlingServiceHandler: p.unidlingServiceHandler,

		serviceLister:   serviceLister,
		endpointsLister: endpointsLister,

		waitIndicator: waiter,
	}
}

type idlerChangeHandler struct {
	mainEndpointsHandler   proxyconfig.EndpointsHandler
	mainServiceHandler     proxyconfig.ServiceHandler
	mainProxy              proxy.ProxyProvider
	unidlingServiceHandler proxyconfig.ServiceHandler

	serviceLister   kcorelisters.ServiceLister
	endpointsLister kcorelisters.EndpointsLister

	waitIndicator WaitIndicator
}

func (h *idlerChangeHandler) OnAdd(obj interface{}) {
	// the addition of an idler is always going to be either a no-op
	// (if it's considered not idled), or a switch on (if it's considered idled)
	idler := obj.(*idling.Idler)
	if !idler.Status.Idled {
		glog.V(6).Infof("hybrid proxy: ignore unidled idler %s/%s on add", idler.Namespace, idler.Name)
		// a non-idled idler does nothing
		return
	}

	// an idled idler forces updates to its services
	h.switchIdledOn(idler.Namespace, idler.Spec.TriggerServiceNames)
}

// TODO(directxman12): deal with the case where someone removes a trigger service
// while idled more proactively

func (h *idlerChangeHandler) OnUpdate(oldObj, newObj interface{}) {
	oldIdler := oldObj.(*idling.Idler)
	newIdler := newObj.(*idling.Idler)

	if oldIdler.Status.Idled == newIdler.Status.Idled {
		// don't do anything when we didn't switch our idled state
		glog.V(8).Infof("hybrid proxy: ignore unchanged idled state for %s/%s on update", newIdler.Namespace, newIdler.Name)
		return
	}
	// TODO(directxman12): deal with the case where someone adds a trigger service during idling

	if newIdler.Status.Idled {
		// we're switching to idled
		h.switchIdledOn(newIdler.Namespace, newIdler.Spec.TriggerServiceNames)
	} else {
		// we're switching to unidled
		h.switchIdledOff(newIdler.Namespace, newIdler.Spec.TriggerServiceNames)
	}
}
func (h *idlerChangeHandler) OnDelete(obj interface{}) {
	// the delete of an idler is always going to be either a no-op
	// (if not idled) or a switch to unidled (if idled)
	idler := obj.(*idling.Idler)
	if !idler.Status.Idled {
		// a non-idled idler does nothing
		glog.V(6).Infof("hybrid proxy: ignore unidled idler %s/%s on delete", idler.Namespace, idler.Name)
		return
	}

	// an idled idler forces updates to its services
	h.switchIdledOff(idler.Namespace, idler.Spec.TriggerServiceNames)
}

func (h *idlerChangeHandler) switchIdledOn(namespace string, svcNames []string) {
	glog.V(6).Infof("hybrid proxy: switch idling on for %s/%v", namespace, svcNames)
	for _, svcName := range svcNames {
		// make sure this is before the endpoints, so that if we have
		// missing endpoints, we can still switch services on and off
		svc, err := h.serviceLister.Services(namespace).Get(svcName)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to fetch service %s/%s while handling idler: %v", namespace, svcName, err))
			continue
		}

		// enable the service in the unidling proxy
		h.unidlingServiceHandler.OnServiceAdd(svc)

		eps, err := h.endpointsLister.Endpoints(namespace).Get(svcName)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to fetch endpoints %s/%s while handling idler: %v", namespace, svcName, err))
			continue
		}

		// ensure that the endpoints are removed from the main handler
		h.mainEndpointsHandler.OnEndpointsDelete(eps)
	}
}

func (h *idlerChangeHandler) switchIdledOff(namespace string, svcNames []string) {
	glog.V(6).Infof("hybrid proxy: switch idling off for %s/%v", namespace, svcNames)
	for _, svcName := range svcNames {
		svc, err := h.serviceLister.Services(namespace).Get(svcName)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to fetch service %s/%s while handling idler: %v", namespace, svcName, err))
			continue
		}

		eps, err := h.endpointsLister.Endpoints(namespace).Get(svcName)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to fetch endpoints %s/%s while handling idler: %v", namespace, svcName, err))
			continue
		}

		// ensure that the endpoints are added from the main handler
		h.mainEndpointsHandler.OnEndpointsAdd(eps)

		h.waitIndicator.Reset()
		// force a sync so that our iptables rules are all in place when we switch idling off
		h.mainProxy.Sync()

		// actually release our services
		h.unidlingServiceHandler.OnServiceDelete(svc)
	}
}
