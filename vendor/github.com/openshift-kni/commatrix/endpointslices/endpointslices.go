package endpointslices

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-kni/commatrix/types"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/client"
	"github.com/openshift-kni/commatrix/consts"
	nodesutil "github.com/openshift-kni/commatrix/nodes"
)

type EndpointSlicesInfo struct {
	EndpointSlice discoveryv1.EndpointSlice
	Service       corev1.Service
	Pods          []corev1.Pod
}

func GetIngressEndpointSlicesInfo(cs *client.ClientSet) ([]EndpointSlicesInfo, error) {
	var epSlicesList discoveryv1.EndpointSliceList
	err := cs.List(context.TODO(), &epSlicesList, &rtclient.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list endpointslices: %w", err)
	}
	log.Debugf("amount of EndpointSlices in the cluster: %d", len(epSlicesList.Items))

	var servicesList corev1.ServiceList
	err = cs.List(context.TODO(), &servicesList, &rtclient.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	log.Debugf("amount of Services in the cluster: %d", len(servicesList.Items))

	var podsList corev1.PodList
	err = cs.List(context.TODO(), &podsList, &rtclient.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	log.Debug("amount of Pods in the cluster: ", len(podsList.Items))

	epsliceInfos, err := createEPSliceInfos(epSlicesList.Items, servicesList.Items, podsList.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to bundle resources: %w", err)
	}
	log.Debug("length of the created epsliceInfos slice: ", len(epsliceInfos))
	res := FilterForIngressTraffic(epsliceInfos)

	log.Debug("length of the slice after filter: ", len(res))
	return res, nil
}

func ToComDetails(cs *client.ClientSet, epSlicesInfo []EndpointSlicesInfo) ([]types.ComDetails, error) {
	comDetails := make([]types.ComDetails, 0)
	nodeList, err := cs.CoreV1Interface.Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, epSliceInfo := range epSlicesInfo {
		cds, err := epSliceInfo.toComDetails(nodeList.Items)
		if err != nil {
			return nil, err
		}

		comDetails = append(comDetails, cds...)
	}

	cleanedComDetails := removeDups(comDetails)
	return cleanedComDetails, nil
}

// createEPSliceInfos gets lists of EndpointSlices, Services, and Pods and generates
// a slice of EndpointSlicesInfo, each representing a distinct service.
func createEPSliceInfos(epSlices []discoveryv1.EndpointSlice, services []corev1.Service, pods []corev1.Pod) ([]EndpointSlicesInfo, error) {
	res := make([]EndpointSlicesInfo, 0)

	for _, epSlice := range epSlices {
		// Fetch info about the service behind the endpointslice.
		if len(epSlice.OwnerReferences) == 0 {
			log.Warnf("empty OwnerReferences in EndpointSlice %s/%s. skipping", epSlice.Namespace, epSlice.Name)
			continue
		}

		ownerRef := epSlice.OwnerReferences[0]
		name := ownerRef.Name
		namespace := epSlice.Namespace

		service := getService(name, namespace, services)
		if service == nil {
			return nil, fmt.Errorf("failed to get service for endpoint %s/%s", epSlice.Namespace, epSlice.Name)
		}

		// Fetch info about the pods behind the endpointslice.
		resPods := make([]corev1.Pod, 0)
		for _, endpoint := range epSlice.Endpoints {
			if endpoint.TargetRef == nil {
				log.Warnf("empty TargetRef for endpoint %s in EndpointSlice %s. skipping", *endpoint.NodeName, epSlice.Name)
				continue
			}
			name := endpoint.TargetRef.Name
			namespace := endpoint.TargetRef.Namespace

			pod := getPod(name, namespace, pods)
			if pod == nil {
				log.Warnf("failed to get pod %s/%s for endpoint in EndpointSlice %s. skipping", namespace, name, epSlice.Name)
				continue
			}

			resPods = append(resPods, *pod)
			log.Debugf("Added a new endpointSliceInfo with pods len: %d", len(pods))
			res = append(res, EndpointSlicesInfo{
				EndpointSlice: epSlice,
				Service:       *service,
				Pods:          resPods,
			})
		}
	}

	return res, nil
}

func getPod(name, namespace string, pods []corev1.Pod) *corev1.Pod {
	for _, pod := range pods {
		if pod.Name == name && pod.Namespace == namespace {
			return &pod
		}
	}
	return nil
}

func getService(name, namespace string, services []corev1.Service) *corev1.Service {
	for _, service := range services {
		if service.Name == name && service.Namespace == namespace {
			return &service
		}
	}

	return nil
}

// getEndpointSliceNodeRoles gets endpointslice Info struct and returns which node roles the services are on.
func getEndpointSliceNodeRoles(epSliceInfo *EndpointSlicesInfo, nodes []corev1.Node) []string {
	// map to prevent duplications
	rolesMap := make(map[string]bool)
	for _, endpoint := range epSliceInfo.EndpointSlice.Endpoints {
		nodeName := endpoint.NodeName
		for _, node := range nodes {
			if node.Name == *nodeName {
				role := nodesutil.GetRole(&node)
				rolesMap[role] = true
				log.Debug("found node, role is:", role)
			}
		}
	}

	roles := []string{}
	for k := range rolesMap {
		roles = append(roles, k)
	}

	return roles
}

func getContainerName(portNum int, pods []corev1.Pod) (string, error) {
	res := ""
	pod := pods[0]
	found := false

	if len(pods) == 0 {
		return "", fmt.Errorf("got empty pods slice")
	}

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

func extractPodName(pod *corev1.Pod) (string, error) {
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

func (epSliceinfo *EndpointSlicesInfo) toComDetails(nodes []corev1.Node) ([]types.ComDetails, error) {
	if len(epSliceinfo.EndpointSlice.OwnerReferences) == 0 {
		return nil, fmt.Errorf("empty OwnerReferences in EndpointSlice %s/%s. skipping", epSliceinfo.EndpointSlice.Namespace, epSliceinfo.EndpointSlice.Name)
	}

	res := make([]types.ComDetails, 0)

	// Get the Namespace and Pod's name from the service.
	namespace := epSliceinfo.Service.Namespace
	name, err := extractPodName(&epSliceinfo.Pods[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get pod name for endpointslice %s: %w", epSliceinfo.EndpointSlice.Name, err)
	}

	// Get the node roles of this endpointslice. (master or worker or both).
	roles := getEndpointSliceNodeRoles(epSliceinfo, nodes)

	epSlice := epSliceinfo.EndpointSlice

	optional := isOptional(epSlice)
	service := epSlice.Labels["kubernetes.io/service-name"]

	for _, role := range roles {
		for _, port := range epSlice.Ports {
			containerName, err := getContainerName(int(*port.Port), epSliceinfo.Pods)
			if err != nil {
				log.Warningf("failed to get container name for EndpointSlice %s/%s: %s", namespace, name, err)
				continue
			}

			res = append(res, types.ComDetails{
				Direction: consts.IngressLabel,
				Protocol:  string(*port.Protocol),
				Port:      int(*port.Port),
				Namespace: namespace,
				Pod:       name,
				Container: containerName,
				NodeRole:  role,
				Service:   service,
				Optional:  optional,
			})
		}
	}

	return res, nil
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
