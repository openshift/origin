package endpointslices

import (
	"context"
	"fmt"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/mcp"
	"github.com/openshift-kni/commatrix/pkg/types"
)

type EndpointSlicesInfo struct {
	EndpointSlice discoveryv1.EndpointSlice
	Service       corev1.Service
	Pods          []corev1.Pod
}

type EndpointSlicesExporter struct {
	*client.ClientSet
	nodeToGroup map[string]string
	sliceInfo   []EndpointSlicesInfo
}

// NodeToGroup returns Node→Group mapping.
// - When MCPs exist: uses the MCP pool name.
// - When MCPs do not exist (e.g., HyperShift): prefers the 'hypershift.openshift.io/nodePool' label.
// - Otherwise: falls back to the node role.
func (ep *EndpointSlicesExporter) NodeToGroup() map[string]string {
	return ep.nodeToGroup
}

type NoOwnerRefErr struct {
	name      string
	namespace string
}

func (e *NoOwnerRefErr) Error() string {
	return fmt.Sprintf("empty OwnerReferences in EndpointSlice %s/%s. skipping", e.namespace, e.name)
}

func New(cs *client.ClientSet) (*EndpointSlicesExporter, error) {
	// Try MCP-based resolution first
	if nodeToPool, err := mcp.ResolveNodeToPool(cs); err == nil {
		return &EndpointSlicesExporter{ClientSet: cs, nodeToGroup: nodeToPool, sliceInfo: []EndpointSlicesInfo{}}, nil
	}

	// Fallback: build node->group map (HyperShift or clusters without MCP): prefer NodePool label, else role
	nodeToGroup, err := types.BuildNodeToGroupMap(cs)
	if err != nil {
		return nil, err
	}
	return &EndpointSlicesExporter{ClientSet: cs, nodeToGroup: nodeToGroup, sliceInfo: []EndpointSlicesInfo{}}, nil
}

// load endpoint slices for services from type loadbalancer and node port,
// or with port specified with hostNetwork or hostPort.
func (ep *EndpointSlicesExporter) LoadExposedEndpointSlicesInfo() error {
	// get all the services
	servicesList := &corev1.ServiceList{}
	err := ep.List(context.TODO(), servicesList, &rtclient.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}
	epsliceInfos := []EndpointSlicesInfo{}
	for _, service := range servicesList.Items {
		// get the endpoint slice for this object
		epl := &discoveryv1.EndpointSliceList{}
		label, err := labels.Parse(fmt.Sprintf("kubernetes.io/service-name=%s", service.Name))
		if err != nil {
			return fmt.Errorf("failed to create selector for endpoint slice, %v", err)
		}
		err = ep.List(context.TODO(), epl, &rtclient.ListOptions{Namespace: service.Namespace, LabelSelector: label})
		if err != nil {
			return fmt.Errorf("failed to list endpoint slice, %v", err)
		}

		if len(epl.Items) == 0 {
			log.Debug("no endpoint slice found for service name", service.Name)
			continue
		}

		pods := &corev1.PodList{}
		exposedService := isExposedService(service)
		if len(service.Spec.Selector) == 0 {
			// If an internal service has no selector, ports can't be exposed, skip.
			if !exposedService {
				log.Debug("no pods found for internal service, selector wasn't defined", service.Name)
				continue
			}
		} else {
			label = labels.SelectorFromSet(service.Spec.Selector)
			err = ep.List(context.TODO(), pods, &rtclient.ListOptions{Namespace: service.Namespace, LabelSelector: label})
			if err != nil {
				return fmt.Errorf("failed to list pods, %v", err)
			}

			// If there are no pods found for the service, skip.
			if len(pods.Items) == 0 {
				log.Debug("no pods found for service name", service.Name)
				continue
			}

			ports := epl.Items[0].Ports
			// 	Check if all pod ports are exposed, otherwise, keep only ports linked to an EndpointSlice and hostPort.
			if !exposedService && !isHostNetworked(pods.Items[0]) {
				epsPortsInfo := getEndpointSlicePortsFromPod(pods.Items[0], epl.Items[0].Ports)
				ports = filterEndpointPortsByPodHostPort(epsPortsInfo)
			}
			if len(ports) == 0 {
				continue
			}
			epl.Items[0].Ports = ports
		}
		epsliceInfo := createEPSliceInfo(service, epl.Items[0], pods.Items)
		log.Debug("epsliceInfo created", epsliceInfo)
		epsliceInfos = append(epsliceInfos, epsliceInfo)
	}

	log.Debug("length of the created epsliceInfos slice: ", len(epsliceInfos))
	ep.sliceInfo = epsliceInfos
	return nil
}

func (ep *EndpointSlicesExporter) ToComDetails() ([]types.ComDetails, error) {
	comDetails := make([]types.ComDetails, 0)

	for _, epSliceInfo := range ep.sliceInfo {
		cds, err := epSliceInfo.toComDetailsWithGroups(ep.nodeToGroup)
		if err != nil {
			switch err.(type) {
			case *NoOwnerRefErr:
				log.Debug(err.Error())
				continue
			default:
				return nil, err
			}
		}
		comDetails = append(comDetails, cds...)
	}

	cleanedComDetails := removeDups(comDetails)
	return cleanedComDetails, nil
}

func createEPSliceInfo(service corev1.Service, ep discoveryv1.EndpointSlice, pods []corev1.Pod) EndpointSlicesInfo {
	return EndpointSlicesInfo{
		EndpointSlice: ep,
		Service:       service,
		Pods:          pods,
	}
}

func (ei *EndpointSlicesInfo) getEndpointSliceGroups(nodeToGroup map[string]string) []string {
	poolsMap := make(map[string]bool)
	for _, endpoint := range ei.EndpointSlice.Endpoints {
		if endpoint.NodeName == nil {
			continue
		}
		pool := nodeToGroup[*endpoint.NodeName]
		if pool == "" {
			continue
		}
		poolsMap[pool] = true
	}

	pools := []string{}
	for k := range poolsMap {
		pools = append(pools, k)
	}

	slices.Sort(pools)
	return pools
}

func (ei *EndpointSlicesInfo) toComDetailsWithGroups(nodeToGroup map[string]string) ([]types.ComDetails, error) {
	if len(ei.EndpointSlice.OwnerReferences) == 0 {
		return nil, &NoOwnerRefErr{name: ei.EndpointSlice.Name, namespace: ei.EndpointSlice.Namespace}
	}

	res := make([]types.ComDetails, 0)

	// Get the Namespace and Pod's name from the service.
	namespace := ei.Service.Namespace
	name, err := extractControllerName(&ei.Pods[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get pod name for endpointslice %s: %w", ei.EndpointSlice.Name, err)
	}

	// Get the groups backing this EndpointSlice (pool names or roles).
	pools := ei.getEndpointSliceGroups(nodeToGroup)

	epSlice := ei.EndpointSlice
	optional := isOptional(epSlice)

	for _, port := range epSlice.Ports {
		containerName, err := getContainerName(int(*port.Port), ei.Pods)
		if err != nil {
			log.Warningf("failed to get container name for EndpointSlice %s/%s: %s", namespace, name, err)
			containerName = ""
		}

		for _, pool := range pools {
			res = append(res, types.ComDetails{
				Direction: consts.IngressLabel,
				Protocol:  string(*port.Protocol),
				Port:      int(*port.Port),
				Namespace: namespace,
				Pod:       name,
				Container: containerName,
				NodeGroup: pool,
				Service:   ei.Service.Name,
				Optional:  optional,
			})
		}
	}

	return res, nil
}

func getContainerName(portNum int, pods []corev1.Pod) (string, error) {
	if len(pods) == 0 {
		return "", fmt.Errorf("got empty pods slice")
	}

	res := ""
	pod := pods[0]
	found := false

	for i := 0; i < len(pod.Spec.Containers); i++ {
		container := pod.Spec.Containers[i]

		if found {
			break
		}

		for _, port := range container.Ports {
			if port.ContainerPort == int32(portNum) {
				res = container.Name
				found = true
				break
			}
		}
	}

	if !found {
		return "", fmt.Errorf("couldn't find port %d in pods", portNum)
	}

	return res, nil
}

func extractControllerName(pod *corev1.Pod) (string, error) {
	if len(pod.OwnerReferences) == 0 {
		return pod.Name, nil
	}

	ownerRefName := pod.OwnerReferences[0].Name
	switch pod.OwnerReferences[0].Kind {
	case "Node":
		res, found := strings.CutSuffix(pod.Name, fmt.Sprintf("-%s", pod.Spec.NodeName))
		if !found {
			return "", fmt.Errorf("pod name %s is not ending with node name %s", pod.Name, pod.Spec.NodeName)
		}
		return res, nil
	case "ReplicaSet":
		return ownerRefName[:strings.LastIndex(ownerRefName, "-")], nil
	case "DaemonSet":
		return ownerRefName, nil
	case "StatefulSet":
		return ownerRefName, nil
	case "ReplicationController":
		return ownerRefName, nil
	}

	return "", fmt.Errorf("failed to extract pod name for %s", pod.Name)
}

func isOptional(epSlice discoveryv1.EndpointSlice) bool {
	optional := false
	if _, ok := epSlice.Labels[consts.OptionalLabel]; ok {
		optional = true
	}

	return optional
}

func removeDups(comDetails []types.ComDetails) []types.ComDetails {
	set := sets.New[types.ComDetails](comDetails...)
	res := set.UnsortedList()

	return res
}
