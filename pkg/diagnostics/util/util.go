package util

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

var (
	AdminKubeConfigPaths = []string{
		"/etc/openshift/master/admin.kubeconfig",           // enterprise
		"/openshift.local.config/master/admin.kubeconfig",  // origin systemd
		"./openshift.local.config/master/admin.kubeconfig", // origin binary
	}
)

func GetNodes(kubeClient *kclient.Client) ([]kapi.Node, error) {
	nodeList, err := kubeClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Listing nodes in the cluster failed. Error: %s", err)
	}
	return nodeList.Items, nil
}

func GetSchedulableNodes(kubeClient *kclient.Client) ([]kapi.Node, error) {
	filteredNodes := []kapi.Node{}
	nodes, err := GetNodes(kubeClient)
	if err != nil {
		return filteredNodes, err
	}

	for _, node := range nodes {
		ready := kapi.ConditionUnknown
		// Get node ready status
		for _, condition := range node.Status.Conditions {
			if condition.Type == kapi.NodeReady {
				ready = condition.Status
				break
			}
		}

		// Skip if node is unschedulable or is not ready
		if node.Spec.Unschedulable || (ready != kapi.ConditionTrue) {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}
	return filteredNodes, nil
}
