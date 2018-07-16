package aggregated_logging

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

const (
	componentNameEs         = "es"
	componentNameEsOps      = "es-ops"
	componentNameKibana     = "kibana"
	componentNameKibanaOps  = "kibana-ops"
	componentNameCurator    = "curator"
	componentNameCuratorOps = "curator-ops"
	componentNameMux        = "mux"
)

// loggingComponents are those 'managed' by rep controllers (e.g. fluentd is deployed with a DaemonSet)
var expectedLoggingComponents = sets.NewString(componentNameEs, componentNameKibana, componentNameCurator)
var optionalLoggingComponents = sets.NewString(componentNameEsOps, componentNameKibanaOps, componentNameCuratorOps, componentNameMux)
var loggingComponents = expectedLoggingComponents.Union(optionalLoggingComponents)

const deploymentConfigWarnOptionalMissing = `
Did not find a DeploymentConfig to support optional component '%s'. If you require
this component, please re-install or update logging and specify the appropriate
variable to enable it.
`

const deploymentConfigZeroPodsFound = `
There were no Pods found that support logging.  Try running
the following commands for additional information:

  $ oc describe dc -n %[1]s
  $ oc get events -n %[1]s
`
const deploymentConfigNoPodsFound = `
There were no Pods found for DeploymentConfig '%[1]s'.  Try running
the following commands for additional information:

  $ oc describe dc %[1]s -n %[2]s
  $ oc get events -n %[2]s
`
const deploymentConfigPodsNotRunning = `
The Pod '%[1]s' matched by DeploymentConfig '%[2]s' is not in '%[3]s' status: %[4]s.

Depending upon the state, this could mean there is an error running the image
for one or more pod containers, the node could be pulling images, etc.  Try running
the following commands for additional information:

  $ oc describe pod %[1]s -n %[5]s
  $ oc logs %[1]s -n %[5]s
  $ oc get events -n %[5]s
`

func checkDeploymentConfigs(r diagnosticReporter, adapter deploymentConfigAdapter, project string) {
	compReq, _ := labels.NewRequirement(componentKey, selection.In, loggingComponents.List())
	provReq, _ := labels.NewRequirement(providerKey, selection.Equals, []string{openshiftValue})
	selector := labels.NewSelector().Add(*compReq, *provReq)
	r.Debug("AGL0040", fmt.Sprintf("Checking for DeploymentConfigs in project '%s' with selector '%s'", project, selector))
	dcList, err := adapter.deploymentconfigs(project, metav1.ListOptions{LabelSelector: selector.String()})
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
		if !found.Has(entry) {
			if optionalLoggingComponents.Has(entry) {
				r.Info("AGL0060", fmt.Sprintf(deploymentConfigWarnOptionalMissing, entry))
			} else {
				r.Error("AGL0065", nil, fmt.Sprintf("Did not find a DeploymentConfig to support component '%s'", entry))
			}
		}
	}
	checkDeploymentConfigPods(r, adapter, *dcList, project)
}

func checkDeploymentConfigPods(r diagnosticReporter, adapter deploymentConfigAdapter, dcs appsapi.DeploymentConfigList, project string) {
	compReq, _ := labels.NewRequirement(componentKey, selection.In, loggingComponents.List())
	provReq, _ := labels.NewRequirement(providerKey, selection.Equals, []string{openshiftValue})
	podSelector := labels.NewSelector().Add(*compReq, *provReq)
	r.Debug("AGL0070", fmt.Sprintf("Getting pods that match selector '%s'", podSelector))
	podList, err := adapter.pods(project, metav1.ListOptions{LabelSelector: podSelector.String()})
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
		dcName, hasDcName := pod.ObjectMeta.Annotations[appsapi.DeploymentConfigAnnotation]
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
