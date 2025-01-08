/*
Copyright 2016 The Kubernetes Authors.

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

package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epodoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	e2erc "k8s.io/kubernetes/test/e2e/framework/rc"
	testutils "k8s.io/kubernetes/test/utils"
	imageutils "k8s.io/kubernetes/test/utils/image"
	netutils "k8s.io/utils/net"
)

// NodePortRange should match whatever the default/configured range is
var NodePortRange = utilnet.PortRange{Base: 30000, Size: 2768}

// It is copied from "k8s.io/kubernetes/pkg/registry/core/service/portallocator"
var errAllocated = errors.New("provided port is already allocated")

// TestJig is a test jig to help service testing.
type TestJig struct {
	Client    clientset.Interface
	Namespace string
	Name      string
	ID        string
	Labels    map[string]string
	// ExternalIPs should be false for Conformance test
	// Don't check nodeport on external addrs in conformance test, but in e2e test.
	ExternalIPs bool
}

// NewTestJig allocates and inits a new TestJig.
func NewTestJig(client clientset.Interface, namespace, name string) *TestJig {
	j := &TestJig{}
	j.Client = client
	j.Namespace = namespace
	j.Name = name
	j.ID = j.Name + "-" + string(uuid.NewUUID())
	j.Labels = map[string]string{"testid": j.ID}

	return j
}

// newServiceTemplate returns the default v1.Service template for this j, but
// does not actually create the Service.  The default Service has the same name
// as the j and exposes the given port.
func (j *TestJig) newServiceTemplate(proto v1.Protocol, port int32) *v1.Service {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: j.Namespace,
			Name:      j.Name,
			Labels:    j.Labels,
		},
		Spec: v1.ServiceSpec{
			Selector: j.Labels,
			Ports: []v1.ServicePort{
				{
					Protocol: proto,
					Port:     port,
				},
			},
		},
	}
	return service
}

// CreateTCPServiceWithPort creates a new TCP Service with given port based on the
// j's defaults. Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateTCPServiceWithPort(ctx context.Context, tweak func(svc *v1.Service), port int32) (*v1.Service, error) {
	svc := j.newServiceTemplate(v1.ProtocolTCP, port)
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP Service %q: %w", svc.Name, err)
	}
	return j.sanityCheckService(result, svc.Spec.Type)
}

// CreateTCPService creates a new TCP Service based on the j's
// defaults.  Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateTCPService(ctx context.Context, tweak func(svc *v1.Service)) (*v1.Service, error) {
	return j.CreateTCPServiceWithPort(ctx, tweak, 80)
}

// CreateUDPService creates a new UDP Service based on the j's
// defaults.  Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateUDPService(ctx context.Context, tweak func(svc *v1.Service)) (*v1.Service, error) {
	svc := j.newServiceTemplate(v1.ProtocolUDP, 80)
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP Service %q: %w", svc.Name, err)
	}
	return j.sanityCheckService(result, svc.Spec.Type)
}

// CreateExternalNameService creates a new ExternalName type Service based on the j's defaults.
// Callers can provide a function to tweak the Service object before it is created.
func (j *TestJig) CreateExternalNameService(ctx context.Context, tweak func(svc *v1.Service)) (*v1.Service, error) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: j.Namespace,
			Name:      j.Name,
			Labels:    j.Labels,
		},
		Spec: v1.ServiceSpec{
			Selector:     j.Labels,
			ExternalName: "foo.example.com",
			Type:         v1.ServiceTypeExternalName,
		},
	}
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create ExternalName Service %q: %w", svc.Name, err)
	}
	return j.sanityCheckService(result, svc.Spec.Type)
}

// ChangeServiceType updates the given service's ServiceType to the given newType.
func (j *TestJig) ChangeServiceType(ctx context.Context, newType v1.ServiceType, timeout time.Duration) error {
	ingressIP := ""
	svc, err := j.UpdateService(ctx, func(s *v1.Service) {
		for _, ing := range s.Status.LoadBalancer.Ingress {
			if ing.IP != "" {
				ingressIP = ing.IP
			}
		}
		s.Spec.Type = newType
		s.Spec.Ports[0].NodePort = 0
	})
	if err != nil {
		return err
	}
	if ingressIP != "" {
		_, err = j.WaitForLoadBalancerDestroy(ctx, ingressIP, int(svc.Spec.Ports[0].Port), timeout)
	}
	return err
}

// CreateOnlyLocalNodePortService creates a NodePort service with
// ExternalTrafficPolicy set to Local and sanity checks its nodePort.
// If createPod is true, it also creates an RC with 1 replica of
// the standard netexec container used everywhere in this test.
func (j *TestJig) CreateOnlyLocalNodePortService(ctx context.Context, createPod bool) (*v1.Service, error) {
	ginkgo.By("creating a service " + j.Namespace + "/" + j.Name + " with type=NodePort and ExternalTrafficPolicy=Local")
	svc, err := j.CreateTCPService(ctx, func(svc *v1.Service) {
		svc.Spec.Type = v1.ServiceTypeNodePort
		svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyLocal
		svc.Spec.Ports = []v1.ServicePort{{Protocol: v1.ProtocolTCP, Port: 80}}
	})
	if err != nil {
		return nil, err
	}

	if createPod {
		ginkgo.By("creating a pod to be part of the service " + j.Name)
		_, err = j.Run(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	return svc, nil
}

// CreateOnlyLocalLoadBalancerService creates a loadbalancer service with
// ExternalTrafficPolicy set to Local and waits for it to acquire an ingress IP.
// If createPod is true, it also creates an RC with 1 replica of
// the standard netexec container used everywhere in this test.
func (j *TestJig) CreateOnlyLocalLoadBalancerService(ctx context.Context, timeout time.Duration, createPod bool,
	tweak func(svc *v1.Service)) (*v1.Service, error) {
	_, err := j.CreateLoadBalancerService(ctx, timeout, func(svc *v1.Service) {
		ginkgo.By("setting ExternalTrafficPolicy=Local")
		svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyLocal
		if tweak != nil {
			tweak(svc)
		}
	})
	if err != nil {
		return nil, err
	}

	if createPod {
		ginkgo.By("creating a pod to be part of the service " + j.Name)
		_, err = j.Run(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	ginkgo.By("waiting for loadbalancer for service " + j.Namespace + "/" + j.Name)
	return j.WaitForLoadBalancer(ctx, timeout)
}

// CreateLoadBalancerService creates a loadbalancer service and waits
// for it to acquire an ingress IP.
func (j *TestJig) CreateLoadBalancerService(ctx context.Context, timeout time.Duration, tweak func(svc *v1.Service)) (*v1.Service, error) {
	ginkgo.By("creating a service " + j.Namespace + "/" + j.Name + " with type=LoadBalancer")
	svc := j.newServiceTemplate(v1.ProtocolTCP, 80)
	svc.Spec.Type = v1.ServiceTypeLoadBalancer
	// We need to turn affinity off for our LB distribution tests
	svc.Spec.SessionAffinity = v1.ServiceAffinityNone
	if tweak != nil {
		tweak(svc)
	}
	_, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create LoadBalancer Service %q: %w", svc.Name, err)
	}

	ginkgo.By("waiting for loadbalancer for service " + j.Namespace + "/" + j.Name)
	return j.WaitForLoadBalancer(ctx, timeout)
}

// GetEndpointNodes returns a map of nodenames:external-ip on which the
// endpoints of the Service are running.
func (j *TestJig) GetEndpointNodes(ctx context.Context) (map[string][]string, error) {
	return j.GetEndpointNodesWithIP(ctx, v1.NodeExternalIP)
}

// GetEndpointNodesWithIP returns a map of nodenames:<ip of given type> on which the
// endpoints of the Service are running.
func (j *TestJig) GetEndpointNodesWithIP(ctx context.Context, addressType v1.NodeAddressType) (map[string][]string, error) {
	nodes, err := j.ListNodesWithEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	nodeMap := map[string][]string{}
	for _, node := range nodes {
		nodeMap[node.Name] = e2enode.GetAddresses(&node, addressType)
	}
	return nodeMap, nil
}

// ListNodesWithEndpoint returns a list of nodes on which the
// endpoints of the given Service are running.
func (j *TestJig) ListNodesWithEndpoint(ctx context.Context) ([]v1.Node, error) {
	nodeNames, err := j.GetEndpointNodeNames(ctx)
	if err != nil {
		return nil, err
	}
	allNodes, err := j.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	epNodes := make([]v1.Node, 0, nodeNames.Len())
	for _, node := range allNodes.Items {
		if nodeNames.Has(node.Name) {
			epNodes = append(epNodes, node)
		}
	}
	return epNodes, nil
}

// GetEndpointNodeNames returns a string set of node names on which the
// endpoints of the given Service are running.
func (j *TestJig) GetEndpointNodeNames(ctx context.Context) (sets.String, error) {
	err := j.waitForAvailableEndpoint(ctx, ServiceEndpointsTimeout)
	if err != nil {
		return nil, err
	}
	endpoints, err := j.Client.CoreV1().Endpoints(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get endpoints for service %s/%s failed (%s)", j.Namespace, j.Name, err)
	}
	if len(endpoints.Subsets) == 0 {
		return nil, fmt.Errorf("endpoint has no subsets, cannot determine node addresses")
	}
	epNodes := sets.NewString()
	for _, ss := range endpoints.Subsets {
		for _, e := range ss.Addresses {
			if e.NodeName != nil {
				epNodes.Insert(*e.NodeName)
			}
		}
	}
	return epNodes, nil
}

// WaitForEndpointOnNode waits for a service endpoint on the given node.
func (j *TestJig) WaitForEndpointOnNode(ctx context.Context, nodeName string) error {
	return wait.PollUntilContextTimeout(ctx, framework.Poll, KubeProxyLagTimeout, true, func(ctx context.Context) (bool, error) {
		endpoints, err := j.Client.CoreV1().Endpoints(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Get endpoints for service %s/%s failed (%s)", j.Namespace, j.Name, err)
			return false, nil
		}
		if len(endpoints.Subsets) == 0 {
			framework.Logf("Expect endpoints with subsets, got none.")
			return false, nil
		}
		// TODO: Handle multiple endpoints
		if len(endpoints.Subsets[0].Addresses) == 0 {
			framework.Logf("Expected Ready endpoints - found none")
			return false, nil
		}
		epHostName := *endpoints.Subsets[0].Addresses[0].NodeName
		framework.Logf("Pod for service %s/%s is on node %s", j.Namespace, j.Name, epHostName)
		if epHostName != nodeName {
			framework.Logf("Found endpoint on wrong node, expected %v, got %v", nodeName, epHostName)
			return false, nil
		}
		return true, nil
	})
}

// waitForAvailableEndpoint waits for at least 1 endpoint to be available till timeout
func (j *TestJig) waitForAvailableEndpoint(ctx context.Context, timeout time.Duration) error {
	//Wait for endpoints to be created, this may take longer time if service backing pods are taking longer time to run
	endpointSelector := fields.OneTermEqualSelector("metadata.name", j.Name)
	stopCh := make(chan struct{})
	endpointAvailable := false
	endpointSliceAvailable := false

	var controller cache.Controller
	_, controller = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = endpointSelector.String()
				obj, err := j.Client.CoreV1().Endpoints(j.Namespace).List(ctx, options)
				return runtime.Object(obj), err
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = endpointSelector.String()
				return j.Client.CoreV1().Endpoints(j.Namespace).Watch(ctx, options)
			},
		},
		&v1.Endpoints{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if e, ok := obj.(*v1.Endpoints); ok {
					if len(e.Subsets) > 0 && len(e.Subsets[0].Addresses) > 0 {
						endpointAvailable = true
					}
				}
			},
			UpdateFunc: func(old, cur interface{}) {
				if e, ok := cur.(*v1.Endpoints); ok {
					if len(e.Subsets) > 0 && len(e.Subsets[0].Addresses) > 0 {
						endpointAvailable = true
					}
				}
			},
		},
	)
	defer func() {
		close(stopCh)
	}()

	go controller.Run(stopCh)

	var esController cache.Controller
	_, esController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = "kubernetes.io/service-name=" + j.Name
				obj, err := j.Client.DiscoveryV1().EndpointSlices(j.Namespace).List(ctx, options)
				return runtime.Object(obj), err
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = "kubernetes.io/service-name=" + j.Name
				return j.Client.DiscoveryV1().EndpointSlices(j.Namespace).Watch(ctx, options)
			},
		},
		&discoveryv1.EndpointSlice{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if es, ok := obj.(*discoveryv1.EndpointSlice); ok {
					// TODO: currently we only consider addresses in 1 slice, but services with
					// a large number of endpoints (>1000) may have multiple slices. Some slices
					// with only a few addresses. We should check the addresses in all slices.
					if len(es.Endpoints) > 0 && len(es.Endpoints[0].Addresses) > 0 {
						endpointSliceAvailable = true
					}
				}
			},
			UpdateFunc: func(old, cur interface{}) {
				if es, ok := cur.(*discoveryv1.EndpointSlice); ok {
					// TODO: currently we only consider addresses in 1 slice, but services with
					// a large number of endpoints (>1000) may have multiple slices. Some slices
					// with only a few addresses. We should check the addresses in all slices.
					if len(es.Endpoints) > 0 && len(es.Endpoints[0].Addresses) > 0 {
						endpointSliceAvailable = true
					}
				}
			},
		},
	)

	go esController.Run(stopCh)

	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		return endpointAvailable && endpointSliceAvailable, nil
	})
	if err != nil {
		return fmt.Errorf("no subset of available IP address found for the endpoint %s within timeout %v", j.Name, timeout)
	}
	return nil
}

// sanityCheckService performs sanity checks on the given service; in particular, ensuring
// that creating/updating a service allocates IPs, ports, etc, as needed. It does not
// check for ingress assignment as that happens asynchronously after the Service is created.
func (j *TestJig) sanityCheckService(svc *v1.Service, svcType v1.ServiceType) (*v1.Service, error) {
	if svcType == "" {
		svcType = v1.ServiceTypeClusterIP
	}
	if svc.Spec.Type != svcType {
		return nil, fmt.Errorf("unexpected Spec.Type (%s) for service, expected %s", svc.Spec.Type, svcType)
	}

	if svcType != v1.ServiceTypeExternalName {
		if svc.Spec.ExternalName != "" {
			return nil, fmt.Errorf("unexpected Spec.ExternalName (%s) for service, expected empty", svc.Spec.ExternalName)
		}
		if svc.Spec.ClusterIP == "" {
			return nil, fmt.Errorf("didn't get ClusterIP for non-ExternalName service")
		}
	} else {
		if svc.Spec.ClusterIP != "" {
			return nil, fmt.Errorf("unexpected Spec.ClusterIP (%s) for ExternalName service, expected empty", svc.Spec.ClusterIP)
		}
	}

	expectNodePorts := needsNodePorts(svc)
	for i, port := range svc.Spec.Ports {
		hasNodePort := port.NodePort != 0
		if hasNodePort != expectNodePorts {
			return nil, fmt.Errorf("unexpected Spec.Ports[%d].NodePort (%d) for service", i, port.NodePort)
		}
		if hasNodePort {
			if !NodePortRange.Contains(int(port.NodePort)) {
				return nil, fmt.Errorf("out-of-range nodePort (%d) for service", port.NodePort)
			}
		}
	}

	// FIXME: this fails for tests that were changed from LoadBalancer to ClusterIP.
	// if svcType != v1.ServiceTypeLoadBalancer {
	// 	if len(svc.Status.LoadBalancer.Ingress) != 0 {
	// 		return nil, fmt.Errorf("unexpected Status.LoadBalancer.Ingress on non-LoadBalancer service")
	// 	}
	// }

	return svc, nil
}

func needsNodePorts(svc *v1.Service) bool {
	if svc == nil {
		return false
	}
	// Type NodePort
	if svc.Spec.Type == v1.ServiceTypeNodePort {
		return true
	}
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		return false
	}
	// Type LoadBalancer
	if svc.Spec.AllocateLoadBalancerNodePorts == nil {
		return true //back-compat
	}
	return *svc.Spec.AllocateLoadBalancerNodePorts
}

// UpdateService fetches a service, calls the update function on it, and
// then attempts to send the updated service. It tries up to 3 times in the
// face of timeouts and conflicts.
func (j *TestJig) UpdateService(ctx context.Context, update func(*v1.Service)) (*v1.Service, error) {
	for i := 0; i < 3; i++ {
		service, err := j.Client.CoreV1().Services(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get Service %q: %w", j.Name, err)
		}
		update(service)
		result, err := j.Client.CoreV1().Services(j.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err == nil {
			return j.sanityCheckService(result, service.Spec.Type)
		}
		if !apierrors.IsConflict(err) && !apierrors.IsServerTimeout(err) {
			return nil, fmt.Errorf("failed to update Service %q: %w", j.Name, err)
		}
	}
	return nil, fmt.Errorf("too many retries updating Service %q", j.Name)
}

// WaitForNewIngressIP waits for the given service to get a new ingress IP, or returns an error after the given timeout
func (j *TestJig) WaitForNewIngressIP(ctx context.Context, existingIP string, timeout time.Duration) (*v1.Service, error) {
	framework.Logf("Waiting up to %v for service %q to get a new ingress IP", timeout, j.Name)
	service, err := j.waitForCondition(ctx, timeout, "have a new ingress IP", func(svc *v1.Service) bool {
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return false
		}
		ip := svc.Status.LoadBalancer.Ingress[0].IP
		if ip == "" || ip == existingIP {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return j.sanityCheckService(service, v1.ServiceTypeLoadBalancer)
}

// ChangeServiceNodePort changes node ports of the given service.
func (j *TestJig) ChangeServiceNodePort(ctx context.Context, initial int) (*v1.Service, error) {
	var err error
	var service *v1.Service
	for i := 1; i < NodePortRange.Size; i++ {
		offs1 := initial - NodePortRange.Base
		offs2 := (offs1 + i) % NodePortRange.Size
		newPort := NodePortRange.Base + offs2
		service, err = j.UpdateService(ctx, func(s *v1.Service) {
			s.Spec.Ports[0].NodePort = int32(newPort)
		})
		if err != nil && strings.Contains(err.Error(), errAllocated.Error()) {
			framework.Logf("tried nodePort %d, but it is in use, will try another", newPort)
			continue
		}
		// Otherwise err was nil or err was a real error
		break
	}
	return service, err
}

// WaitForLoadBalancer waits the given service to have a LoadBalancer, or returns an error after the given timeout
func (j *TestJig) WaitForLoadBalancer(ctx context.Context, timeout time.Duration) (*v1.Service, error) {
	framework.Logf("Waiting up to %v for service %q to have a LoadBalancer", timeout, j.Name)
	service, err := j.waitForCondition(ctx, timeout, "have a load balancer", func(svc *v1.Service) bool {
		return len(svc.Status.LoadBalancer.Ingress) > 0
	})
	if err != nil {
		return nil, err
	}

	for i, ing := range service.Status.LoadBalancer.Ingress {
		if ing.IP == "" && ing.Hostname == "" {
			return nil, fmt.Errorf("unexpected Status.LoadBalancer.Ingress[%d] for service: %#v", i, ing)
		}
	}

	return j.sanityCheckService(service, v1.ServiceTypeLoadBalancer)
}

// WaitForLoadBalancerDestroy waits the given service to destroy a LoadBalancer, or returns an error after the given timeout
func (j *TestJig) WaitForLoadBalancerDestroy(ctx context.Context, ip string, port int, timeout time.Duration) (*v1.Service, error) {
	// TODO: once support ticket 21807001 is resolved, reduce this timeout back to something reasonable
	defer func() {
		if err := framework.EnsureLoadBalancerResourcesDeleted(ctx, ip, strconv.Itoa(port)); err != nil {
			framework.Logf("Failed to delete cloud resources for service: %s %d (%v)", ip, port, err)
		}
	}()

	framework.Logf("Waiting up to %v for service %q to have no LoadBalancer", timeout, j.Name)
	service, err := j.waitForCondition(ctx, timeout, "have no load balancer", func(svc *v1.Service) bool {
		return len(svc.Status.LoadBalancer.Ingress) == 0
	})
	if err != nil {
		return nil, err
	}
	return j.sanityCheckService(service, service.Spec.Type)
}

func (j *TestJig) waitForCondition(ctx context.Context, timeout time.Duration, message string, conditionFn func(*v1.Service) bool) (*v1.Service, error) {
	var service *v1.Service
	pollFunc := func(ctx context.Context) (bool, error) {
		svc, err := j.Client.CoreV1().Services(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Retrying .... error trying to get Service %s: %v", j.Name, err)
			return false, nil
		}
		if conditionFn(svc) {
			service = svc
			return true, nil
		}
		return false, nil
	}
	if err := wait.PollUntilContextTimeout(ctx, framework.Poll, timeout, true, pollFunc); err != nil {
		return nil, fmt.Errorf("timed out waiting for service %q to %s: %w", j.Name, message, err)
	}
	return service, nil
}

// newRCTemplate returns the default v1.ReplicationController object for
// this j, but does not actually create the RC.  The default RC has the same
// name as the j and runs the "netexec" container.
func (j *TestJig) newRCTemplate() *v1.ReplicationController {
	var replicas int32 = 1
	var grace int64 = 3 // so we don't race with kube-proxy when scaling up/down

	rc := &v1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: j.Namespace,
			Name:      j.Name,
			Labels:    j.Labels,
		},
		Spec: v1.ReplicationControllerSpec{
			Replicas: &replicas,
			Selector: j.Labels,
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: j.Labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "netexec",
							Image: imageutils.GetE2EImage(imageutils.Agnhost),
							Args:  []string{"netexec", "--http-port=80", "--udp-port=80"},
							ReadinessProbe: &v1.Probe{
								PeriodSeconds: 3,
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Port: intstr.FromInt32(80),
										Path: "/hostName",
									},
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &grace,
				},
			},
		},
	}
	return rc
}

// AddRCAntiAffinity adds AntiAffinity to the given ReplicationController.
func (j *TestJig) AddRCAntiAffinity(rc *v1.ReplicationController) {
	var replicas int32 = 2

	rc.Spec.Replicas = &replicas
	if rc.Spec.Template.Spec.Affinity == nil {
		rc.Spec.Template.Spec.Affinity = &v1.Affinity{}
	}
	if rc.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
		rc.Spec.Template.Spec.Affinity.PodAntiAffinity = &v1.PodAntiAffinity{}
	}
	rc.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
		rc.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		v1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{MatchLabels: j.Labels},
			Namespaces:    nil,
			TopologyKey:   "kubernetes.io/hostname",
		})
}

// CreatePDB returns a PodDisruptionBudget for the given ReplicationController, or returns an error if a PodDisruptionBudget isn't ready
func (j *TestJig) CreatePDB(ctx context.Context, rc *v1.ReplicationController) (*policyv1.PodDisruptionBudget, error) {
	pdb := j.newPDBTemplate(rc)
	newPdb, err := j.Client.PolicyV1().PodDisruptionBudgets(j.Namespace).Create(ctx, pdb, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create PDB %q %v", pdb.Name, err)
	}
	if err := j.waitForPdbReady(ctx); err != nil {
		return nil, fmt.Errorf("failed waiting for PDB to be ready: %w", err)
	}

	return newPdb, nil
}

// newPDBTemplate returns the default policyv1.PodDisruptionBudget object for
// this j, but does not actually create the PDB.  The default PDB specifies a
// MinAvailable of N-1 and matches the pods created by the RC.
func (j *TestJig) newPDBTemplate(rc *v1.ReplicationController) *policyv1.PodDisruptionBudget {
	minAvailable := intstr.FromInt32(*rc.Spec.Replicas - 1)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: j.Namespace,
			Name:      j.Name,
			Labels:    j.Labels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector:     &metav1.LabelSelector{MatchLabels: j.Labels},
		},
	}

	return pdb
}

// Run creates a ReplicationController and Pod(s) and waits for the
// Pod(s) to be running. Callers can provide a function to tweak the RC object
// before it is created.
func (j *TestJig) Run(ctx context.Context, tweak func(rc *v1.ReplicationController)) (*v1.ReplicationController, error) {
	rc := j.newRCTemplate()
	if tweak != nil {
		tweak(rc)
	}
	result, err := j.Client.CoreV1().ReplicationControllers(j.Namespace).Create(ctx, rc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create RC %q: %w", rc.Name, err)
	}
	pods, err := j.waitForPodsCreated(ctx, int(*(rc.Spec.Replicas)))
	if err != nil {
		return nil, fmt.Errorf("failed to create pods: %w", err)
	}
	if err := j.waitForPodsReady(ctx, pods); err != nil {
		return nil, fmt.Errorf("failed waiting for pods to be running: %w", err)
	}
	return result, nil
}

// Scale scales pods to the given replicas
func (j *TestJig) Scale(ctx context.Context, replicas int) error {
	rc := j.Name
	scale, err := j.Client.CoreV1().ReplicationControllers(j.Namespace).GetScale(ctx, rc, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get scale for RC %q: %w", rc, err)
	}

	scale.ResourceVersion = "" // indicate the scale update should be unconditional
	scale.Spec.Replicas = int32(replicas)
	_, err = j.Client.CoreV1().ReplicationControllers(j.Namespace).UpdateScale(ctx, rc, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale RC %q: %w", rc, err)
	}
	pods, err := j.waitForPodsCreated(ctx, replicas)
	if err != nil {
		return fmt.Errorf("failed waiting for pods: %w", err)
	}
	if err := j.waitForPodsReady(ctx, pods); err != nil {
		return fmt.Errorf("failed waiting for pods to be running: %w", err)
	}
	return nil
}

func (j *TestJig) waitForPdbReady(ctx context.Context) error {
	timeout := 2 * time.Minute
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(2 * time.Second) {
		pdb, err := j.Client.PolicyV1().PodDisruptionBudgets(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pdb.Status.DisruptionsAllowed > 0 {
			return nil
		}
	}

	return fmt.Errorf("timeout waiting for PDB %q to be ready", j.Name)
}

func (j *TestJig) waitForPodsCreated(ctx context.Context, replicas int) ([]string, error) {
	// TODO (pohly): replace with gomega.Eventually
	timeout := 2 * time.Minute
	// List the pods, making sure we observe all the replicas.
	label := labels.SelectorFromSet(labels.Set(j.Labels))
	framework.Logf("Waiting up to %v for %d pods to be created", timeout, replicas)
	for start := time.Now(); time.Since(start) < timeout && ctx.Err() == nil; time.Sleep(2 * time.Second) {
		options := metav1.ListOptions{LabelSelector: label.String()}
		pods, err := j.Client.CoreV1().Pods(j.Namespace).List(ctx, options)
		if err != nil {
			return nil, err
		}

		found := []string{}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			found = append(found, pod.Name)
		}
		if len(found) == replicas {
			framework.Logf("Found all %d pods", replicas)
			return found, nil
		}
		framework.Logf("Found %d/%d pods - will retry", len(found), replicas)
	}
	return nil, fmt.Errorf("timeout waiting for %d pods to be created", replicas)
}

func (j *TestJig) waitForPodsReady(ctx context.Context, pods []string) error {
	timeout := 2 * time.Minute
	if !e2epod.CheckPodsRunningReady(ctx, j.Client, j.Namespace, pods, timeout) {
		return fmt.Errorf("timeout waiting for %d pods to be ready", len(pods))
	}
	return nil
}

func testReachabilityOverServiceName(ctx context.Context, serviceName string, sp v1.ServicePort, execPod *v1.Pod) error {
	return testEndpointReachability(ctx, serviceName, sp.Port, sp.Protocol, execPod)
}

func testReachabilityOverClusterIP(ctx context.Context, clusterIP string, sp v1.ServicePort, execPod *v1.Pod) error {
	// If .spec.clusterIP is set to "" or "None" for service, ClusterIP is not created, so reachability can not be tested over clusterIP:servicePort
	if netutils.ParseIPSloppy(clusterIP) == nil {
		return fmt.Errorf("unable to parse ClusterIP: %s", clusterIP)
	}
	return testEndpointReachability(ctx, clusterIP, sp.Port, sp.Protocol, execPod)
}

func testReachabilityOverExternalIP(ctx context.Context, externalIP string, sp v1.ServicePort, execPod *v1.Pod) error {
	return testEndpointReachability(ctx, externalIP, sp.Port, sp.Protocol, execPod)
}

func testReachabilityOverNodePorts(ctx context.Context, nodes *v1.NodeList, sp v1.ServicePort, pod *v1.Pod, clusterIP string, externalIPs bool) error {
	internalAddrs := e2enode.CollectAddresses(nodes, v1.NodeInternalIP)
	isClusterIPV4 := netutils.IsIPv4String(clusterIP)

	for _, internalAddr := range internalAddrs {
		// If the node's internal address points to localhost, then we are not
		// able to test the service reachability via that address
		if isInvalidOrLocalhostAddress(internalAddr) {
			framework.Logf("skipping testEndpointReachability() for internal address %s", internalAddr)
			continue
		}
		// Check service reachability on the node internalIP which is same family as clusterIP
		if isClusterIPV4 != netutils.IsIPv4String(internalAddr) {
			framework.Logf("skipping testEndpointReachability() for internal address %s as it does not match clusterIP (%s) family", internalAddr, clusterIP)
			continue
		}

		err := testEndpointReachability(ctx, internalAddr, sp.NodePort, sp.Protocol, pod)
		if err != nil {
			return err
		}
	}
	if externalIPs {
		externalAddrs := e2enode.CollectAddresses(nodes, v1.NodeExternalIP)
		for _, externalAddr := range externalAddrs {
			if isClusterIPV4 != netutils.IsIPv4String(externalAddr) {
				framework.Logf("skipping testEndpointReachability() for external address %s as it does not match clusterIP (%s) family", externalAddr, clusterIP)
				continue
			}
			err := testEndpointReachability(ctx, externalAddr, sp.NodePort, sp.Protocol, pod)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// isInvalidOrLocalhostAddress returns `true` if the provided `ip` is either not
// parsable or the loopback address. Otherwise it will return `false`.
func isInvalidOrLocalhostAddress(ip string) bool {
	parsedIP := netutils.ParseIPSloppy(ip)
	if parsedIP == nil || parsedIP.IsLoopback() {
		return true
	}
	return false
}

// testEndpointReachability tests reachability to endpoints (i.e. IP, ServiceName) and ports. Test request is initiated from specified execPod.
// TCP and UDP protocol based service are supported at this moment
// TODO: add support to test SCTP Protocol based services.
func testEndpointReachability(ctx context.Context, endpoint string, port int32, protocol v1.Protocol, execPod *v1.Pod) error {
	ep := net.JoinHostPort(endpoint, strconv.Itoa(int(port)))
	cmd := ""
	switch protocol {
	case v1.ProtocolTCP:
		cmd = fmt.Sprintf("echo hostName | nc -v -t -w 2 %s %v", endpoint, port)
	case v1.ProtocolUDP:
		cmd = fmt.Sprintf("echo hostName | nc -v -u -w 2 %s %v", endpoint, port)
	default:
		return fmt.Errorf("service reachability check is not supported for %v", protocol)
	}

	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, ServiceReachabilityShortPollTimeout, true, func(ctx context.Context) (bool, error) {
		stdout, err := e2epodoutput.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
		if err != nil {
			framework.Logf("Service reachability failing with error: %v\nRetrying...", err)
			return false, nil
		}
		trimmed := strings.TrimSpace(stdout)
		if trimmed != "" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("service is not reachable within %v timeout on endpoint %s over %s protocol", ServiceReachabilityShortPollTimeout, ep, protocol)
	}
	return nil
}

// checkClusterIPServiceReachability ensures that service of type ClusterIP is reachable over
// - ServiceName:ServicePort, ClusterIP:ServicePort
func (j *TestJig) checkClusterIPServiceReachability(ctx context.Context, svc *v1.Service, pod *v1.Pod) error {
	clusterIP := svc.Spec.ClusterIP
	servicePorts := svc.Spec.Ports
	externalIPs := svc.Spec.ExternalIPs

	err := j.waitForAvailableEndpoint(ctx, ServiceEndpointsTimeout)
	if err != nil {
		return err
	}

	for _, servicePort := range servicePorts {
		err = testReachabilityOverServiceName(ctx, svc.Name, servicePort, pod)
		if err != nil {
			return err
		}
		err = testReachabilityOverClusterIP(ctx, clusterIP, servicePort, pod)
		if err != nil {
			return err
		}
		if len(externalIPs) > 0 {
			for _, externalIP := range externalIPs {
				err = testReachabilityOverExternalIP(ctx, externalIP, servicePort, pod)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkNodePortServiceReachability ensures that service of type nodePort are reachable
//   - Internal clients should be reachable to service over -
//     ServiceName:ServicePort, ClusterIP:ServicePort and NodeInternalIPs:NodePort
//   - External clients should be reachable to service over -
//     NodePublicIPs:NodePort
func (j *TestJig) checkNodePortServiceReachability(ctx context.Context, svc *v1.Service, pod *v1.Pod) error {
	clusterIP := svc.Spec.ClusterIP
	servicePorts := svc.Spec.Ports

	// Consider only 2 nodes for testing
	nodes, err := e2enode.GetBoundedReadySchedulableNodes(ctx, j.Client, 2)
	if err != nil {
		return err
	}

	err = j.waitForAvailableEndpoint(ctx, ServiceEndpointsTimeout)
	if err != nil {
		return err
	}

	for _, servicePort := range servicePorts {
		err = testReachabilityOverServiceName(ctx, svc.Name, servicePort, pod)
		if err != nil {
			return err
		}
		err = testReachabilityOverClusterIP(ctx, clusterIP, servicePort, pod)
		if err != nil {
			return err
		}
		err = testReachabilityOverNodePorts(ctx, nodes, servicePort, pod, clusterIP, j.ExternalIPs)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkExternalServiceReachability ensures service of type externalName resolves to IP address and no fake externalName is set
// FQDN of kubernetes is used as externalName(for air tight platforms).
func (j *TestJig) checkExternalServiceReachability(ctx context.Context, svc *v1.Service, pod *v1.Pod) error {
	// NOTE(claudiub): Windows does not support PQDN.
	svcName := fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, framework.TestContext.ClusterDNSDomain)
	// Service must resolve to IP
	cmd := fmt.Sprintf("nslookup %s", svcName)
	return wait.PollUntilContextTimeout(ctx, framework.Poll, ServiceReachabilityShortPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		_, stderr, err := e2epodoutput.RunHostCmdWithFullOutput(pod.Namespace, pod.Name, cmd)
		// NOTE(claudiub): nslookup may return 0 on Windows, even though the DNS name was not found. In this case,
		// we can check stderr for the error.
		if err != nil || (framework.NodeOSDistroIs("windows") && strings.Contains(stderr, fmt.Sprintf("can't find %s", svcName))) {
			framework.Logf("ExternalName service %q failed to resolve to IP", pod.Namespace+"/"+pod.Name)
			return false, nil
		}
		return true, nil
	})
}

// CheckServiceReachability ensures that request are served by the services. Only supports Services with type ClusterIP, NodePort and ExternalName.
func (j *TestJig) CheckServiceReachability(ctx context.Context, svc *v1.Service, pod *v1.Pod) error {
	svcType := svc.Spec.Type

	_, err := j.sanityCheckService(svc, svcType)
	if err != nil {
		return err
	}

	switch svcType {
	case v1.ServiceTypeClusterIP:
		return j.checkClusterIPServiceReachability(ctx, svc, pod)
	case v1.ServiceTypeNodePort:
		return j.checkNodePortServiceReachability(ctx, svc, pod)
	case v1.ServiceTypeExternalName:
		return j.checkExternalServiceReachability(ctx, svc, pod)
	case v1.ServiceTypeLoadBalancer:
		return j.checkClusterIPServiceReachability(ctx, svc, pod)
	default:
		return fmt.Errorf("unsupported service type \"%s\" to verify service reachability for \"%s\" service. This may due to diverse implementation of the service type", svcType, svc.Name)
	}
}

// CreateServicePods creates a replication controller with the label same as service. Service listens to TCP and UDP.
func (j *TestJig) CreateServicePods(ctx context.Context, replica int) error {
	config := testutils.RCConfig{
		Client:       j.Client,
		Name:         j.Name,
		Image:        imageutils.GetE2EImage(imageutils.Agnhost),
		Command:      []string{"/agnhost", "serve-hostname", "--http=false", "--tcp", "--udp"},
		Namespace:    j.Namespace,
		Labels:       j.Labels,
		PollInterval: 3 * time.Second,
		Timeout:      framework.PodReadyBeforeTimeout,
		Replicas:     replica,
	}
	return e2erc.RunRC(ctx, config)
}

// CreateSCTPServiceWithPort creates a new SCTP Service with given port based on the
// j's defaults. Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateSCTPServiceWithPort(ctx context.Context, tweak func(svc *v1.Service), port int32) (*v1.Service, error) {
	svc := j.newServiceTemplate(v1.ProtocolSCTP, port)
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create SCTP Service %q: %w", svc.Name, err)
	}
	return j.sanityCheckService(result, svc.Spec.Type)
}

// CreateLoadBalancerServiceWaitForClusterIPOnly creates a loadbalancer service and waits
// for it to acquire a cluster IP
func (j *TestJig) CreateLoadBalancerServiceWaitForClusterIPOnly(tweak func(svc *v1.Service)) (*v1.Service, error) {
	ginkgo.By("creating a service " + j.Namespace + "/" + j.Name + " with type=LoadBalancer")
	svc := j.newServiceTemplate(v1.ProtocolTCP, 80)
	svc.Spec.Type = v1.ServiceTypeLoadBalancer
	// We need to turn affinity off for our LB distribution tests
	svc.Spec.SessionAffinity = v1.ServiceAffinityNone
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create LoadBalancer Service %q: %w", svc.Name, err)
	}

	return j.sanityCheckService(result, v1.ServiceTypeLoadBalancer)
}
