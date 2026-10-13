/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/cloud-provider-azure/pkg/consts"
	"sigs.k8s.io/cloud-provider-azure/pkg/log"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer/fnutil"
)

func filterServicesByIngressIPs(services []*v1.Service, ips []netip.Addr) []*v1.Service {
	targetIPs := fnutil.Map(func(ip netip.Addr) string { return ip.String() }, ips)

	return fnutil.Filter(func(svc *v1.Service) bool {

		ingressIPs := fnutil.Map(func(ing v1.LoadBalancerIngress) string { return ing.IP }, svc.Status.LoadBalancer.Ingress)

		ingressIPs = fnutil.Filter(func(ip string) bool { return ip != "" }, ingressIPs)

		return len(fnutil.Intersection(ingressIPs, targetIPs)) > 0
	}, services)
}

func filterServicesByDisableFloatingIP(services []*v1.Service) []*v1.Service {
	return fnutil.Filter(func(svc *v1.Service) bool {
		return consts.IsK8sServiceDisableLoadBalancerFloatingIP(svc)
	}, services)
}

// listSharedIPPortMapping lists the shared IP port mapping for the service excluding the service itself.
// There are scenarios where multiple services share the same public IP,
// and in order to clean up the security rules, we need to know the port mapping of the shared IP.
func (az *Cloud) listSharedIPPortMapping(
	ctx context.Context,
	svc *v1.Service,
	ingressIPs []netip.Addr,
) (map[network.SecurityRuleProtocol][]int32, error) {
	var (
		logger = log.FromContextOrBackground(ctx).WithName("listSharedIPPortMapping")
		rv     = make(map[network.SecurityRuleProtocol][]int32)
	)

	var services []*v1.Service
	{
		var err error
		logger.V(5).Info("Listing all services")
		services, err = az.serviceLister.List(labels.Everything())
		if err != nil {
			logger.Error(err, "Failed to list all services")
			return nil, fmt.Errorf("list all services: %w", err)
		}
		logger.V(5).Info("Listed all services", "num-all-services", len(services))

		// Filter services by ingress IPs or backend node pool IPs (when disable floating IP)
		if consts.IsK8sServiceDisableLoadBalancerFloatingIP(svc) {
			logger.V(5).Info("Filter service by disableFloatingIP")
			services = filterServicesByDisableFloatingIP(services)
		} else {
			logger.V(5).Info("Filter service by external IPs")
			services = filterServicesByIngressIPs(services, ingressIPs)
		}
	}
	logger.V(5).Info("Filtered services", "num-filtered-services", len(services))

	for _, s := range services {
		logger.V(5).Info("Iterating service", "service", s.Name, "namespace", s.Namespace)
		if svc.Namespace == s.Namespace && svc.Name == s.Name {
			// skip the service itself
			continue
		}

		portsByProtocol, err := loadbalancer.SecurityRuleDestinationPortsByProtocol(s)
		if err != nil {
			return nil, fmt.Errorf("fetch security rule dst ports for %s: %w", s.Name, err)
		}

		for protocol, ports := range portsByProtocol {
			rv[protocol] = append(rv[protocol], ports...)
		}
	}

	logger.V(5).Info("Retain port mapping", "port-mapping", rv)

	return rv, nil
}
