package endpointslices

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liornoy/node-comm-lib/pkg/client"
	"github.com/liornoy/node-comm-lib/pkg/consts"
	nodesutil "github.com/liornoy/node-comm-lib/pkg/nodes"
	"github.com/liornoy/node-comm-lib/pkg/types"
)

type EndpointSlicesInfo struct {
	EndpointSlice discoveryv1.EndpointSlice
	Serivce       corev1.Service
	Pods          []corev1.Pod
}

func GetIngressEndpointSlicesInfo(cs *client.ClientSet) ([]EndpointSlicesInfo, error) {
	var (
		epSlicesList discoveryv1.EndpointSliceList
		servicesList corev1.ServiceList
		podsList     corev1.PodList
	)

	err := cs.List(context.TODO(), &epSlicesList, &rtclient.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list endpointslices: %w", err)
	}
	log.Debugf("amount of EndpointSlices in the cluster: %d", len(epSlicesList.Items))

	err = cs.List(context.TODO(), &servicesList, &rtclient.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	log.Debugf("amount of Services in the cluster: %d", len(servicesList.Items))

	err = cs.List(context.TODO(), &podsList, &rtclient.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	log.Debug("amount of Pods in the cluster: ", len(podsList.Items))

	epsliceInfos, err := createEPSliceInfos(&epSlicesList, &servicesList, &podsList)
	if err != nil {
		return nil, fmt.Errorf("failed to bundle resources: %w", err)
	}
	log.Debug("length of the creaed epsliceInfos slice: ", len(epsliceInfos))
	res := FilterForIngressTraffic(epsliceInfos)

	log.Debug("length of the slice after filter: ", len(res))
	return res, nil
}

func ToComDetails(cs *client.ClientSet, epSlicesInfo []EndpointSlicesInfo) ([]types.ComDetails, error) {
	comDetails := make([]types.ComDetails, 0)
	nodeList, err := cs.Nodes().List(context.TODO(), metav1.ListOptions{})
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

// createEPSliceInfos retrieves lists of EndpointSlices, Services, and Pods from the cluster and generates
// a slice of EndpointSlicesInfo, each representing a distinct service.
func createEPSliceInfos(epSlicesList *discoveryv1.EndpointSliceList, servicesList *corev1.ServiceList, podsList *corev1.PodList) ([]EndpointSlicesInfo, error) {
	var service *corev1.Service
	var pod corev1.Pod
	found := false
	res := make([]EndpointSlicesInfo, 0)

	for _, epSlice := range epSlicesList.Items {
		// Fetch info about the service behind the endpointslice.
		if len(epSlice.OwnerReferences) == 0 {
			log.Warnf("warning: empty OwnerReferences in EndpointSlice %s/%s. skipping", epSlice.Namespace, epSlice.Name)
			continue
		}

		ownerRef := epSlice.OwnerReferences[0]
		name := ownerRef.Name
		namespace := epSlice.Namespace
		if service, found = getService(name, namespace, servicesList); !found {
			return nil, fmt.Errorf("failed to get service for endpoint %s/%s", epSlice.Namespace, epSlice.Name)
		}

		// Fetch info about the pods behind the endpointslice.
		pods := make([]corev1.Pod, 0)
		for _, endpoint := range epSlice.Endpoints {
			if endpoint.TargetRef == nil {
				log.Warnf("warning: empty TargetRef for endpoint %s in EndpointSlice %s. skipping", *endpoint.NodeName, epSlice.Name)
				continue
			}
			name := endpoint.TargetRef.Name
			namespace := endpoint.TargetRef.Namespace

			if pod, found = getPod(name, namespace, podsList); !found {
				log.Warnf("warning: failed to get pod %s/%s for endpoint in EndpointSlice %s. skipping", namespace, name, epSlice.Name)
				continue
			}

			pods = append(pods, pod)
			log.Debugf("Added a new endpointSliceInfo with pods len: %d", len(pods))
			res = append(res, EndpointSlicesInfo{
				EndpointSlice: epSlice,
				Serivce:       *service,
				Pods:          pods,
			})
		}
	}

	return res, nil
}

func getPod(name, namespace string, podsList *corev1.PodList) (corev1.Pod, bool) {
	for _, pod := range podsList.Items {
		if pod.Name == name && pod.Namespace == namespace {
			return pod, true
		}
	}
	return corev1.Pod{}, false
}

func getService(name, namespace string, serviceList *corev1.ServiceList) (*corev1.Service, bool) {
	for _, service := range serviceList.Items {
		if service.Name == name && service.Namespace == namespace {
			return &service, true
		}
	}

	return nil, false
}

// getEndpointSliceNodeRoles gets endpointslice Info struct and returns which node roles the services are on.
func getEndpointSliceNodeRoles(epSliceInfo *EndpointSlicesInfo, nodes []corev1.Node) []string {
	// map to prevent duplications
	rolesMap := make(map[string]bool)
	for _, endpoint := range epSliceInfo.EndpointSlice.Endpoints {
		nodeName := endpoint.NodeName
		for _, node := range nodes {
			if node.Name == *nodeName {
				role := nodesutil.GetRoles(&node)
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

func (epSliceinfo *EndpointSlicesInfo) toComDetails(nodes []corev1.Node) ([]types.ComDetails, error) {
	if len(epSliceinfo.EndpointSlice.OwnerReferences) == 0 {
		return nil, fmt.Errorf("empty OwnerReferences in EndpointSlice %s/%s. skipping", epSliceinfo.EndpointSlice.Namespace, epSliceinfo.EndpointSlice.Name)
	}

	res := make([]types.ComDetails, 0)

	// Get the Namespace and Pod's name from the service.
	namespace := epSliceinfo.Serivce.Namespace
	name := epSliceinfo.EndpointSlice.OwnerReferences[0].Name

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
				Port:      fmt.Sprint(int(*port.Port)),
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
