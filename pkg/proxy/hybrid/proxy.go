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
	"k8s.io/kubernetes/pkg/util/async"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
)

// RunnableProxy is an extra interface we layer on top of ProxyProvider that
// lets us control exactly when the proxy is synced.
type RunnableProxy interface {
	proxy.ProxyProvider

	SyncProxyRules()
	SetSyncRunner(b *async.BoundedFrequencyRunner)
}

// HybridProxier runs an unidling proxy and a primary proxy at the same time,
// delegating idled services to the unidling proxy and other services to the
// primary proxy.
type HybridProxier struct {
	mainProxy     RunnableProxy
	unidlingProxy RunnableProxy
	syncPeriod    time.Duration
	minSyncPeriod time.Duration
	serviceLister kcorelisters.ServiceLister

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

	// A gate that prevents iptables from being exhausted by proxy calls.
	syncRunner *async.BoundedFrequencyRunner
}

func NewHybridProxier(
	mainProxy RunnableProxy,
	unidlingProxy RunnableProxy,
	syncPeriod time.Duration,
	minSyncPeriod time.Duration,
	serviceLister kcorelisters.ServiceLister,
) (*HybridProxier, error) {
	p := &HybridProxier{
		mainProxy:     mainProxy,
		unidlingProxy: unidlingProxy,
		syncPeriod:    syncPeriod,
		minSyncPeriod: minSyncPeriod,
		serviceLister: serviceLister,

		usingUserspace:      make(map[types.NamespacedName]bool),
		switchedToUserspace: make(map[types.NamespacedName]bool),
	}

	p.syncRunner = async.NewBoundedFrequencyRunner("sync-runner", p.syncProxyRules, minSyncPeriod, syncPeriod, 4)

	// Hackery abound: we want to make sure that changes are applied
	// to both proxies at approximately the same time. That means that we
	// need to stop the two proxy's independent loops and take them over.
	mainProxy.SetSyncRunner(p.syncRunner)
	unidlingProxy.SetSyncRunner(p.syncRunner)

	return p, nil
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
		glog.V(6).Infof("hybrid proxy: add svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingProxy.OnServiceAdd(service)
	} else {
		glog.V(6).Infof("hybrid proxy: add svc %s/%s in main proxy", service.Namespace, service.Name)
		p.mainProxy.OnServiceAdd(service)
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
		glog.V(6).Infof("hybrid proxy: update svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingProxy.OnServiceUpdate(oldService, service)
	} else {
		glog.V(6).Infof("hybrid proxy: update svc %s/%s in main proxy", service.Namespace, service.Name)
		p.mainProxy.OnServiceUpdate(oldService, service)
	}
}

func (p *HybridProxier) OnServiceDelete(service *api.Service) {
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
		glog.V(6).Infof("hybrid proxy: del svc %s/%s in unidling proxy", service.Namespace, service.Name)
		p.unidlingProxy.OnServiceDelete(service)
	} else {
		glog.V(6).Infof("hybrid proxy: del svc %s/%s in main proxy", service.Namespace, service.Name)
		p.mainProxy.OnServiceDelete(service)
	}

	delete(p.switchedToUserspace, svcName)
}

func (p *HybridProxier) OnServiceSynced() {
	p.unidlingProxy.OnServiceSynced()
	p.mainProxy.OnServiceSynced()
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
		glog.V(6).Infof("hybrid proxy: ignoring duplicate switchService(%s/%s)", name.Namespace, name.Name)
		return
	}

	svc, err := p.serviceLister.Services(name.Namespace).Get(name.Name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error while getting service %s/%s from cache: %v", name.Namespace, name.String(), err))
		return
	}

	if p.usingUserspace[name] {
		glog.V(6).Infof("hybrid proxy: switching svc %s/%s to unidling proxy", svc.Namespace, svc.Name)
		p.unidlingProxy.OnServiceAdd(svc)
		p.mainProxy.OnServiceDelete(svc)
	} else {
		glog.V(6).Infof("hybrid proxy: switching svc %s/%s to main proxy", svc.Namespace, svc.Name)
		p.mainProxy.OnServiceAdd(svc)
		p.unidlingProxy.OnServiceDelete(svc)
	}

	p.switchedToUserspace[name] = p.usingUserspace[name]
}

func (p *HybridProxier) OnEndpointsAdd(endpoints *api.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	glog.V(6).Infof("hybrid proxy: (always) add ep %s/%s in unidling proxy", endpoints.Namespace, endpoints.Name)
	p.unidlingProxy.OnEndpointsAdd(endpoints)

	p.usingUserspaceLock.Lock()
	defer p.usingUserspaceLock.Unlock()

	svcName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	wasUsingUserspace, knownEndpoints := p.usingUserspace[svcName]
	p.usingUserspace[svcName] = p.shouldEndpointsUseUserspace(endpoints)

	if !p.usingUserspace[svcName] {
		glog.V(6).Infof("hybrid proxy: add ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainProxy.OnEndpointsAdd(endpoints)
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
	glog.V(6).Infof("hybrid proxy: (always) update ep %s/%s in unidling proxy", endpoints.Namespace, endpoints.Name)
	p.unidlingProxy.OnEndpointsUpdate(oldEndpoints, endpoints)

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
		glog.V(6).Infof("hybrid proxy: update ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainProxy.OnEndpointsUpdate(oldEndpoints, endpoints)
		return
	}

	if p.usingUserspace[svcName] {
		glog.V(6).Infof("hybrid proxy: del ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainProxy.OnEndpointsDelete(oldEndpoints)
	} else {
		glog.V(6).Infof("hybrid proxy: add ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainProxy.OnEndpointsAdd(endpoints)
	}

	p.switchService(svcName)
}

func (p *HybridProxier) OnEndpointsDelete(endpoints *api.Endpoints) {
	// we track all endpoints in the unidling endpoints handler so that we can succesfully
	// detect when a service become unidling
	glog.V(6).Infof("hybrid proxy: (always) del ep %s/%s in unidling proxy", endpoints.Namespace, endpoints.Name)
	p.unidlingProxy.OnEndpointsDelete(endpoints)

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
		glog.V(6).Infof("hybrid proxy: del ep %s/%s in main proxy", endpoints.Namespace, endpoints.Name)
		p.mainProxy.OnEndpointsDelete(endpoints)
	}

	delete(p.usingUserspace, svcName)
}

func (p *HybridProxier) OnEndpointsSynced() {
	p.unidlingProxy.OnEndpointsSynced()
	p.mainProxy.OnEndpointsSynced()
	glog.V(6).Infof("hybrid proxy: endpoints synced")
}

// Sync is called to synchronize the proxier state to iptables
// this doesn't take immediate effect - rather, it requests that the
// BoundedFrequencyRunner call syncProxyRules()
func (p *HybridProxier) Sync() {
	p.syncRunner.Run()
}

// syncProxyRules actually applies the proxy rules to the node.
// It is called by our SyncRunner.
// We do this so that we can guarantee that changes are applied to both
// proxies, especially when unidling a newly-awoken service.
func (p *HybridProxier) syncProxyRules() {
	glog.V(4).Infof("hybrid proxy: syncProxyRules start")

	p.mainProxy.SyncProxyRules()
	p.unidlingProxy.SyncProxyRules()

	glog.V(4).Infof("hybrid proxy: syncProxyRules finished")
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (p *HybridProxier) SyncLoop() {
	// All this does is start our syncRunner, since we pass it *back* in to
	// the mainProxy
	p.mainProxy.SyncLoop()
}
