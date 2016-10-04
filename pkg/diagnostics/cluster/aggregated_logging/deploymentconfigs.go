package aggregated_logging

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/selection"
	"k8s.io/kubernetes/pkg/util/sets"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

const (
	componentNameEs        = "es"
	componentNameEsOps     = "es-ops"
	componentNameKibana    = "kibana"
	componentNameKibanaOps = "kibana-ops"
	componentNameCurator   = "curator"
)

// loggingComponents are those 'managed' by rep controllers (e.g. fluentd is deployed with a DaemonSet)
var loggingComponents = sets.NewString(componentNameEs, componentNameEsOps, componentNameKibana, componentNameKibanaOps, componentNameCurator)

const deploymentConfigWarnMissingForOps = `
Did not find a DeploymentConfig to support component '%s'.  If you require
a separate ElasticSearch cluster to aggregate operations logs, please re-install
or update logging and specify the appropriate switch to enable the ops cluster.
`

const deploymentConfigZeroPodsFound = `
There were no Pods found that support logging.  Try running
the following commands for additional information:

  oc describe dc -n %[1]s
  oc get events -n %[1]s
`
const deploymentConfigNoPodsFound = `
There were no Pods found for DeploymentConfig '%[1]s'.  Try running
the following commands for additional information:

  oc describe dc %[1]s -n %[2]s
  oc get events -n %[2]s
`
const deploymentConfigPodsNotRunning = `
The Pod '%[1]s' matched by DeploymentConfig '%[2]s' is not in '%[3]s' status: %[4]s. 

Depending upon the state, this could mean there is an error running the image 
for one or more pod containers, the node could be pulling images, etc.  Try running
the following commands for additional information:

  oc describe pod %[1]s -n %[5]s
  oc logs %[1]s -n %[5]s
  oc get events -n %[5]s
`

func checkDeploymentConfigs(r diagnosticReporter, adapter deploymentConfigAdapter, project string) {
	req, _ := labels.NewRequirement(loggingInfraKey, selection.Exists, nil)
	selector := labels.NewSelector().Add(*req)
	r.Debug("AGL0040", fmt.Sprintf("Checking for DeploymentConfigs in project '%s' with selector '%s'", project, selector))
	dcList, err := adapter.deploymentconfigs(project, kapi.ListOptions{LabelSelector: selector})
	if err != nil {
		r.Error("AGL0045", err, fmt.Sprintf("There was an error while trying to retrieve the DeploymentConfigs in project '%s': %s", project, err))
		return
	}
	if len(dcList.Items) == 0 {
		r.Error("AGL0047", nil, fmt.Sprintf("Did not find any matching DeploymentConfigs in project '%s' which means no logging components were deployed.  Try running the installer.", project))
		return
	}
	found := sets.NewString()
	for _, entry := range dcList.Items {
		comp := labels.Set(entry.ObjectMeta.Labels).Get(componentKey)
		found.Insert(comp)
		r.Debug("AGL0050", fmt.Sprintf("Found DeploymentConfig '%s' for component '%s'", entry.ObjectMeta.Name, comp))
	}
	for _, entry := range loggingComponents.List() {
		exists := found.Has(entry)
		if !exists {
			if strings.HasSuffix(entry, "-ops") {
				r.Info("AGL0060", fmt.Sprintf(deploymentConfigWarnMissingForOps, entry))
			} else {
				r.Error("AGL0065", nil, fmt.Sprintf("Did not find a DeploymentConfig to support component '%s'", entry))
			}
		}
	}
	checkDeploymentConfigPods(r, adapter, *dcList, project)
}

func checkDeploymentConfigPods(r diagnosticReporter, adapter deploymentConfigAdapter, dcs deployapi.DeploymentConfigList, project string) {
	compReq, _ := labels.NewRequirement(componentKey, selection.In, loggingComponents)
	provReq, _ := labels.NewRequirement(providerKey, selection.Equals, sets.NewString(openshiftValue))
	podSelector := labels.NewSelector().Add(*compReq, *provReq)
	r.Debug("AGL0070", fmt.Sprintf("Getting pods that match selector '%s'", podSelector))
	podList, err := adapter.pods(project, kapi.ListOptions{LabelSelector: podSelector})
	if err != nil {
		r.Error("AGL0075", err, fmt.Sprintf("There was an error while trying to retrieve the pods for the AggregatedLogging stack: %s", err))
		return
	}
	if len(podList.Items) == 0 {
		r.Error("AGL0080", nil, fmt.Sprintf(deploymentConfigZeroPodsFound, project))
		return
	}
	dcPodCount := make(map[string]int, len(dcs.Items))
	for _, dc := range dcs.Items {
		dcPodCount[dc.ObjectMeta.Name] = 0
	}

	for _, pod := range podList.Items {
		r.Debug("AGL0082", fmt.Sprintf("Checking status of Pod '%s'...", pod.ObjectMeta.Name))
		dcName, hasDcName := pod.ObjectMeta.Annotations[deployapi.DeploymentConfigAnnotation]
		if !hasDcName {
			r.Warn("AGL0085", nil, fmt.Sprintf("Found Pod '%s' that that does not reference a logging deployment config which may be acceptable. Skipping check to see if its running.", pod.ObjectMeta.Name))
			continue
		}
		if pod.Status.Phase != kapi.PodRunning {
			podName := pod.ObjectMeta.Name
			r.Error("AGL0090", nil, fmt.Sprintf(deploymentConfigPodsNotRunning, podName, dcName, kapi.PodRunning, pod.Status.Phase, project))
		}
		count, _ := dcPodCount[dcName]
		dcPodCount[dcName] = count + 1
	}
	for name, count := range dcPodCount {
		if count == 0 {
			r.Error("AGL0095", nil, fmt.Sprintf(deploymentConfigNoPodsFound, name, project))
		}
	}
}
