package templaterouter

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	api "k8s.io/kubernetes/pkg/apis/core"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"

	idlingutil "github.com/openshift/origin/pkg/idling"
)

// IdlingCache stores and retrives information related to idling,
// such as whether a given endpoint is part of an idler, and it's
// "normal" (non-idled) set of endpoint subsets.
type IdlingCache interface {
	// IsIdled checks to see if the given endpoints object is idled, and, if so,
	// what its corresponding service's cluster IP is (if present), and the relevant ports.
	IsIdled(endpoints types.NamespacedName) (isIdled bool, serviceIP string, ports []kapi.EndpointPort)
	// NormalEndpoints retrieves the "normal" version of the given endpoints, as would
	// be seen by the router if not for idling.
	NormalEndpoints(endpoints types.NamespacedName) (*api.Endpoints, error)
}

func NewIdlingCache(svcLister kcorelisters.ServiceLister, epLister kcorelisters.EndpointsLister, idlerLookup idlingutil.IdlerServiceLookup) IdlingCache {
	return &idlingCache{
		svcLister:   svcLister,
		epLister:    epLister,
		idlerLookup: idlerLookup,
	}
}

type idlingCache struct {
	svcLister   kcorelisters.ServiceLister
	epLister    kcorelisters.EndpointsLister
	idlerLookup idlingutil.IdlerServiceLookup
}

func (c *idlingCache) IsIdled(svcName types.NamespacedName) (bool, string, []kapi.EndpointPort) {
	idler, hadIdler, err := c.idlerLookup.IdlerByService(svcName)
	if err != nil {
		utilruntime.HandleError(err)
		return false, "", nil
	}
	if !hadIdler || !idler.Status.Idled {
		return false, "", nil
	}

	svc, err := c.svcLister.Services(svcName.Namespace).Get(svcName.Name)
	if err != nil {
		utilruntime.HandleError(err)
		return true, "", nil
	}

	// ignore headless services, too (they have a ClusterIP of "None")
	if !kapihelper.IsServiceIPSet(svc) {
		return true, "", nil
	}

	ports := make([]kapi.EndpointPort, len(svc.Spec.Ports))
	for i, port := range svc.Spec.Ports {
		ports[i] = kapi.EndpointPort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: port.Protocol,
		}
	}

	return true, svc.Spec.ClusterIP, ports
}
func (c *idlingCache) NormalEndpoints(endpoints types.NamespacedName) (*api.Endpoints, error) {
	return c.epLister.Endpoints(endpoints.Namespace).Get(endpoints.Name)
}

func NewNoOpIdlingCache() IdlingCache {
	return &noOpIdlingCache{}
}

type noOpIdlingCache struct{}

func (c *noOpIdlingCache) IsIdled(endpoints types.NamespacedName) (isIdled bool, serviceIP string, ports []kapi.EndpointPort) {
	return false, "", nil
}

func (c *noOpIdlingCache) NormalEndpoints(endpoints types.NamespacedName) (*api.Endpoints, error) {
	return nil, fmt.Errorf("tried to fetch endpoints %s on a no-op idler cache", endpoints.String())
}
