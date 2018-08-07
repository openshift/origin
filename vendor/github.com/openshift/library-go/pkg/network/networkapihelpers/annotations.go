package networkapihelpers

import (
	"fmt"
	"strings"

	networkv1 "github.com/openshift/api/network/v1"
)

type PodNetworkAction string

const (
	// Acceptable values for ChangePodNetworkAnnotation
	GlobalPodNetwork  PodNetworkAction = "global"
	JoinPodNetwork    PodNetworkAction = "join"
	IsolatePodNetwork PodNetworkAction = "isolate"
)

var (
	ErrorPodNetworkAnnotationNotFound = fmt.Errorf("ChangePodNetworkAnnotation not found")
)

// GetChangePodNetworkAnnotation fetches network change intent from NetNamespace
func GetChangePodNetworkAnnotation(netns *networkv1.NetNamespace) (PodNetworkAction, string, error) {
	value, ok := netns.Annotations[networkv1.ChangePodNetworkAnnotation]
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
func SetChangePodNetworkAnnotation(netns *networkv1.NetNamespace, action PodNetworkAction, params string) {
	if netns.Annotations == nil {
		netns.Annotations = make(map[string]string)
	}

	value := string(action)
	if len(params) != 0 {
		value = fmt.Sprintf("%s:%s", value, params)
	}
	netns.Annotations[networkv1.ChangePodNetworkAnnotation] = value
}

// DeleteChangePodNetworkAnnotation removes network change intent from NetNamespace
func DeleteChangePodNetworkAnnotation(netns *networkv1.NetNamespace) {
	delete(netns.Annotations, networkv1.ChangePodNetworkAnnotation)
}
