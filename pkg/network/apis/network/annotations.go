package network

import (
	"fmt"
	"strings"
)

type PodNetworkAction string

const (

	// ChangePodNetworkAnnotation is an annotation on NetNamespace to request change of pod network
	ChangePodNetworkAnnotation string = "pod.network.openshift.io/multitenant.change-network"

	// Acceptable values for ChangePodNetworkAnnotation
	GlobalPodNetwork  PodNetworkAction = "global"
	JoinPodNetwork    PodNetworkAction = "join"
	IsolatePodNetwork PodNetworkAction = "isolate"
)

var (
	ErrorPodNetworkAnnotationNotFound = fmt.Errorf("ChangePodNetworkAnnotation not found")
)

// GetChangePodNetworkAnnotation fetches network change intent from NetNamespace
func GetChangePodNetworkAnnotation(netns *NetNamespace) (PodNetworkAction, string, error) {
	value, ok := netns.Annotations[ChangePodNetworkAnnotation]
	if !ok {
		return PodNetworkAction(""), "", ErrorPodNetworkAnnotationNotFound
	}

	args := strings.Split(value, ":")
	switch PodNetworkAction(args[0]) {
	case GlobalPodNetwork:
		return GlobalPodNetwork, "", nil
	case JoinPodNetwork:
		if len(args) != 2 {
			return PodNetworkAction(""), "", fmt.Errorf("invalid namespace for join pod network: %s", value)
		}
		namespace := args[1]
		return JoinPodNetwork, namespace, nil
	case IsolatePodNetwork:
		return IsolatePodNetwork, "", nil
	}

	return PodNetworkAction(""), "", fmt.Errorf("invalid ChangePodNetworkAnnotation: %s", value)
}

// SetChangePodNetworkAnnotation sets network change intent on NetNamespace
func SetChangePodNetworkAnnotation(netns *NetNamespace, action PodNetworkAction, params string) {
	if netns.Annotations == nil {
		netns.Annotations = make(map[string]string)
	}

	value := string(action)
	if len(params) != 0 {
		value = fmt.Sprintf("%s:%s", value, params)
	}
	netns.Annotations[ChangePodNetworkAnnotation] = value
}

// DeleteChangePodNetworkAnnotation removes network change intent from NetNamespace
func DeleteChangePodNetworkAnnotation(netns *NetNamespace) {
	delete(netns.Annotations, ChangePodNetworkAnnotation)
}
