package unidler

import (
	"net"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/tools/record"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/proxy/userspace"
	"k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
)

type NeedPodsSignaler interface {
	// NeedPods signals that endpoint addresses are needed in order to
	// service a traffic coming to the given service and port
	NeedPods(serviceName types.NamespacedName, port string) error
}

type eventSignaler struct {
	recorder record.EventRecorder
}

func (sig *eventSignaler) NeedPods(serviceName types.NamespacedName, port string) error {
	// TODO: we need to fake this since upstream removed our handle to the ObjectReference
	// This *should* be sufficient for the unidling controller
	serviceRef := v1.ObjectReference{
		Kind:      "Service",
		Namespace: serviceName.Namespace,
		Name:      serviceName.Name,
	}

	// HACK: make the message different to prevent event aggregation
	sig.recorder.Eventf(&serviceRef, v1.EventTypeNormal, unidlingapi.NeedPodsReason, "The service-port %s:%s needs pods.", serviceRef.Name, port)

	return nil
}

// NewEventSignaler constructs a NeedPodsSignaler which signals by recording
// an event for the service with the "NeedPods" reason.
func NewEventSignaler(eventRecorder record.EventRecorder) NeedPodsSignaler {
	return &eventSignaler{
		recorder: eventRecorder,
	}
}

// NewUnidlerProxier creates a new Proxier for the given LoadBalancer and address which fires off
// unidling signals connections and traffic.  It is intended to be used as one half of a HybridProxier.
func NewUnidlerProxier(loadBalancer userspace.LoadBalancer, listenIP net.IP, iptables iptables.Interface, exec utilexec.Interface, pr utilnet.PortRange, syncPeriod, minSyncPeriod, udpIdleTimeout time.Duration, signaler NeedPodsSignaler) (*userspace.Proxier, error) {
	newFunc := func(protocol api.Protocol, ip net.IP, port int) (userspace.ProxySocket, error) {
		return newUnidlerSocket(protocol, ip, port, signaler)
	}
	return userspace.NewCustomProxier(loadBalancer, listenIP, iptables, exec, pr, syncPeriod, minSyncPeriod, udpIdleTimeout, newFunc)
}
