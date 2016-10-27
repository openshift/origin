package aggregated_logging

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
)

const daemonSetNoLabeledNodes = `
There are no nodes that match the selector for DaemonSet '%[1]s'. This
means Fluentd is not running and is not gathering logs from any nodes.
An example of a command to target a specific node for this DaemonSet:

  oc label node/node1.example.com %[2]s

or to label them all:

  oc label node --all %[2]s
`

const daemonSetPartialNodesLabeled = `
There are some nodes that match the selector for DaemonSet '%s'.  
A list of matching nodes can be discovered by running:

  oc get nodes -l %s
`
const daemonSetNoPodsFound = `
There were no pods found that match DaemonSet '%s' with matchLabels '%s'
`
const daemonSetPodsNotRunning = `
The Pod '%[1]s' matched by DaemonSet '%[2]s' is not in '%[3]s' status: %[4]s. 

Depending upon the state, this could mean there is an error running the image 
for one or more pod containers, the node could be pulling images, etc.  Try running
the following commands to get additional information:

  oc describe pod %[1]s -n %[5]s
  oc logs %[1]s -n %[5]s
  oc get events -n %[5]s
`
const daemonSetNotFound = `
There were no DaemonSets in project '%s' that included label '%s'.  This implies
the Fluentd pods are not deployed or the logging stack needs to be upgraded.  Try
running the installer to upgrade the logging stack.
`

var loggingInfraFluentdSelector = labels.Set{loggingInfraKey: "fluentd"}

func checkDaemonSets(r diagnosticReporter, adapter daemonsetAdapter, project string) {
	r.Debug("AGL0400", fmt.Sprintf("Checking DaemonSets in project '%s'...", project))
	dsList, err := adapter.daemonsets(project, kapi.ListOptions{LabelSelector: loggingInfraFluentdSelector.AsSelector()})
	if err != nil {
		r.Error("AGL0405", err, fmt.Sprintf("There was an error while trying to retrieve the logging DaemonSets in project '%s' which is most likely transient: %s", project, err))
		return
	}
	if len(dsList.Items) == 0 {
		r.Error("AGL0407", err, fmt.Sprintf(daemonSetNotFound, project, loggingInfraFluentdSelector.AsSelector()))
		return
	}
	nodeList, err := adapter.nodes(kapi.ListOptions{})
	if err != nil {
		r.Error("AGL0410", err, fmt.Sprintf("There was an error while trying to retrieve the list of Nodes which is most likely transient: %s", err))
		return
	}
	for _, ds := range dsList.Items {
		labeled := 0
		nodeSelector := labels.Set(ds.Spec.Template.Spec.NodeSelector).AsSelector()
		r.Debug("AGL0415", fmt.Sprintf("Checking DaemonSet '%s' nodeSelector '%s'", ds.ObjectMeta.Name, nodeSelector))
		for _, node := range nodeList.Items {
			if nodeSelector.Matches(labels.Set(node.Labels)) {
				labeled = labeled + 1
			}
		}
		switch {
		case labeled == 0:
			r.Error("AGL0420", nil, fmt.Sprintf(daemonSetNoLabeledNodes, ds.ObjectMeta.Name, nodeSelector))
			break
		case labeled < len(nodeList.Items):
			r.Warn("AGL0425", nil, fmt.Sprintf(daemonSetPartialNodesLabeled, ds.ObjectMeta.Name, nodeSelector))
			break
		default:
			r.Debug("AGL0430", fmt.Sprintf("DaemonSet '%s' matches all nodes", ds.ObjectMeta.Name))
		}
		if labeled > 0 {
			checkDaemonSetPods(r, adapter, ds, project, labeled)
		}
	}
}

func checkDaemonSetPods(r diagnosticReporter, adapter daemonsetAdapter, ds kapisext.DaemonSet, project string, numLabeledNodes int) {
	if ds.Spec.Selector == nil {
		r.Debug("AGL0455", "DaemonSet selector is nil. Unable to verify a pod is running")
		return
	}
	podSelector := labels.Set(ds.Spec.Selector.MatchLabels).AsSelector()
	r.Debug("AGL0435", fmt.Sprintf("Checking for running pods for DaemonSet '%s' with matchLabels '%s'", ds.ObjectMeta.Name, podSelector))
	podList, err := adapter.pods(project, kapi.ListOptions{LabelSelector: podSelector})
	if err != nil {
		r.Error("AGL0438", err, fmt.Sprintf("There was an error retrieving pods matched to DaemonSet '%s' that is most likely transient: %s", ds.ObjectMeta.Name, err))
		return
	}
	if len(podList.Items) == 0 {
		r.Error("AGL0440", nil, fmt.Sprintf(daemonSetNoPodsFound, ds.ObjectMeta.Name, podSelector))
		return
	}
	if len(podList.Items) != numLabeledNodes {
		r.Error("AGL0443", nil, fmt.Sprintf("The number of deployed pods %s does not match the number of labeled nodes %d", len(podList.Items), numLabeledNodes))
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase != kapi.PodRunning {
			podName := pod.ObjectMeta.Name
			r.Error("AGL0445", nil, fmt.Sprintf(daemonSetPodsNotRunning, podName, ds.ObjectMeta.Name, kapi.PodRunning, pod.Status.Phase, project))
		}

	}
}
