package util

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/route/api"
)

// UnsecuredRoute will return a route with enough info so that it can direct traffic to
// the service provided by --service. Callers of this helper are responsible for providing
// tls configuration, path, and the hostname of the route.
func UnsecuredRoute(kc kclientset.Interface, namespace, routeName, serviceName, portString string) (*api.Route, error) {
	if len(routeName) == 0 {
		routeName = serviceName
	}

	svc, err := kc.Core().Services(namespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		if len(portString) == 0 {
			return nil, fmt.Errorf("you need to provide a route port via --port when exposing a non-existent service")
		}
		return &api.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name: routeName,
			},
			Spec: api.RouteSpec{
				To: api.RouteTargetReference{
					Name: serviceName,
				},
				Port: resolveRoutePort(portString),
			},
		}, nil
	}

	ok, port := supportsTCP(svc)
	if !ok {
		return nil, fmt.Errorf("service %q doesn't support TCP", svc.Name)
	}

	route := &api.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:   routeName,
			Labels: svc.Labels,
		},
		Spec: api.RouteSpec{
			To: api.RouteTargetReference{
				Name: serviceName,
			},
		},
	}

	// If the service has multiple ports and the user didn't specify --port,
	// then default the route port to a service port name.
	if len(port.Name) > 0 && len(portString) == 0 {
		route.Spec.Port = resolveRoutePort(port.Name)
	}
	// --port uber alles
	if len(portString) > 0 {
		route.Spec.Port = resolveRoutePort(portString)
	}

	return route, nil
}

func resolveRoutePort(portString string) *api.RoutePort {
	if len(portString) == 0 {
		return nil
	}
	var routePort intstr.IntOrString
	integer, err := strconv.Atoi(portString)
	if err != nil {
		routePort = intstr.FromString(portString)
	} else {
		routePort = intstr.FromInt(integer)
	}
	return &api.RoutePort{
		TargetPort: routePort,
	}
}

func supportsTCP(svc *kapi.Service) (bool, kapi.ServicePort) {
	for _, port := range svc.Spec.Ports {
		if port.Protocol == kapi.ProtocolTCP {
			return true, port
		}
	}
	return false, kapi.ServicePort{}
}
