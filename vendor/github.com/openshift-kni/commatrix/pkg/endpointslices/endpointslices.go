package endpointslices

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/types"
)

type EndpointSlicesInfo struct {
	EndpointSlice discoveryv1.EndpointSlice
	Service       corev1.Service
	Pods          []corev1.Pod
}

type EndpointSlicesExporter struct {
	*client.ClientSet
	nodeToRole map[string]string
	sliceInfo  []EndpointSlicesInfo
}

type NoOwnerRefErr struct {
	name      string
	namespace string
}

func (e *NoOwnerRefErr) Error() string {
	return fmt.Sprintf("empty OwnerReferences in EndpointSlice %s/%s. skipping", e.namespace, e.name)
}

func New(cs *client.ClientSet) (*EndpointSlicesExporter, error) {
	nodeList := &corev1.NodeList{}
	err := cs.List(context.TODO(), nodeList)
	if err != nil {
		return nil, err
	}

	nodeToRole := map[string]string{}
	for _, node := range nodeList.Items {
		nodeToRole[node.Name], err = types.GetNodeRole(&node)
		if err != nil {
			return nil, err
		}
		// Info-level log to trace node to role mapping during initialization
		log.Infof("node role mapping: node=%s role=%s", node.Name, nodeToRole[node.Name])
	}

	return &EndpointSlicesExporter{cs, nodeToRole, []EndpointSlicesInfo{}}, nil
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

		label = labels.SelectorFromSet(service.Spec.Selector)
		pods := &corev1.PodList{}
		err = ep.List(context.TODO(), pods, &rtclient.ListOptions{Namespace: service.Namespace, LabelSelector: label})
		if err != nil {
			return fmt.Errorf("failed to list pods, %v", err)
		}

		if len(pods.Items) == 0 {
			log.Debug("no pods found for service name", service.Name)
			continue
		}

		ports := epl.Items[0].Ports
		// 	Check if all pod ports are exposed, otherwise, keep only ports linked to an EndpointSlice and hostPort.
		if !isExposedService(service) && !isHostNetworked(pods.Items[0]) {
			epsPortsInfo := getEndpointSlicePortsFromPod(pods.Items[0], epl.Items[0].Ports)
			ports = filterEndpointPortsByPodHostPort(epsPortsInfo)
		}
		if len(ports) == 0 {
			continue
		}
		epl.Items[0].Ports = ports

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
		cds, err := epSliceInfo.toComDetails(ep.nodeToRole)
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

// getEndpointSliceNodeRoles gets endpointslice Info struct and returns which node roles the services are on.
func (ei *EndpointSlicesInfo) getEndpointSliceNodeRoles(nodesRoles map[string]string) []string {
	// map to prevent duplications
	rolesMap := make(map[string]bool)
	// Dump the full node->role map for troubleshooting empty roles
	log.Infof("node->role map entries=%d content=%v", len(nodesRoles), nodesRoles)
	for _, endpoint := range ei.EndpointSlice.Endpoints {
		var nodeName string
		if endpoint.NodeName == nil {
			log.Warnf("EndpointSlice %s/%s: endpoint has nil NodeName; service=%s", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, ei.Service.Name)
			if dump, err := json.MarshalIndent(ei.EndpointSlice, "", "  "); err == nil {
				log.Infof("EndpointSlice dump %s/%s:\n%s", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, string(dump))
			} else {
				log.Warnf("failed to marshal EndpointSlice %s/%s: %v", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, err)
			}
			nodeName = ""
		} else {
			nodeName = *endpoint.NodeName
		}
		role, ok := nodesRoles[nodeName]
		if !ok {
			log.Warnf("EndpointSlice %s/%s: node %s not found in node->role map; service=%s", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, nodeName, ei.Service.Name)
		}
		if role == "" {
			log.Warnf("EndpointSlice %s/%s: empty role for node %s; service=%s", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, nodeName, ei.Service.Name)
		}
		rolesMap[role] = true
	}

	roles := []string{}
	for k := range rolesMap {
		roles = append(roles, k)
	}

	return roles
}

func (ei *EndpointSlicesInfo) toComDetails(nodesRoles map[string]string) ([]types.ComDetails, error) {
	if len(ei.EndpointSlice.OwnerReferences) == 0 {
		return nil, &NoOwnerRefErr{name: ei.EndpointSlice.Name, namespace: ei.EndpointSlice.Namespace}
	}

	res := make([]types.ComDetails, 0)

	// Dump the entire EndpointSlice for visibility
	if dump, err := json.MarshalIndent(ei.EndpointSlice, "", "  "); err == nil {
		log.Infof("EndpointSlice full dump %s/%s:\n%s", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, string(dump))
	} else {
		log.Warnf("failed to marshal EndpointSlice %s/%s: %v", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, err)
	}

	// Get the Namespace and Pod's name from the service.
	namespace := ei.Service.Namespace
	name, err := extractControllerName(&ei.Pods[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get pod name for endpointslice %s: %w", ei.EndpointSlice.Name, err)
	}

	// Get the node roles of this endpointslice.
	roles := ei.getEndpointSliceNodeRoles(nodesRoles)
	if len(roles) == 0 {
		log.Warnf("EndpointSlice %s/%s: no node roles resolved; service=%s", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name, ei.Service.Name)
	}

	epSlice := ei.EndpointSlice
	optional := isOptional(epSlice)

	for _, port := range epSlice.Ports {
		containerName, err := getContainerName(int(*port.Port), ei.Pods)
		if err != nil {
			log.Warningf("failed to get container name for EndpointSlice %s/%s: %s", namespace, name, err)
			containerName = "" // Default to an empty string if the container is not found.
		}

		for _, role := range roles {
			if role == "" {
				log.Warnf("EndpointSlice %s/%s: creating ComDetails with empty node role; service=%s port=%d protocol=%s pod=%s", namespace, ei.EndpointSlice.Name, ei.Service.Name, int(*port.Port), string(*port.Protocol), name)
			}
			res = append(res, types.ComDetails{
				Direction: consts.IngressLabel,
				Protocol:  string(*port.Protocol),
				Port:      int(*port.Port),
				Namespace: namespace,
				Pod:       name,
				Container: containerName,
				NodeRole:  role,
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
