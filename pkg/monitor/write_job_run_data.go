package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/kustomize/kyaml/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

func WriteEventsForJobRun(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	return monitorserialization.EventsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), events)
}

func WriteTrackedResourcesForJobRun(artifactDir string, recordedResources monitorapi.ResourcesMap, _ monitorapi.Intervals, timeSuffix string) error {
	errors := []error{}

	// write out the current state of resources that we explicitly tracked.
	for resourceType, instanceMap := range recordedResources {
		targetFile := fmt.Sprintf("resource-%s%s.zip", resourceType, timeSuffix)
		if err := monitorserialization.InstanceMapToFile(filepath.Join(artifactDir, targetFile), resourceType, instanceMap); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func WriteBackendDisruptionForJobRun(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	backendDisruption := computeDisruptionData(events)
	return writeDisruptionData(filepath.Join(artifactDir, fmt.Sprintf("backend-disruption%s.json", timeSuffix)), backendDisruption)
}

type BackendDisruptionList struct {
	// BackendDisruptions is keyed by name to make the consumption easier
	BackendDisruptions map[string]*BackendDisruption
}

type BackendDisruption struct {
	// Name ensure self-identification, it includes the connection type
	Name string
	// BackendName is the name of backend.  It is the same across all connection types.
	BackendName string
	// ConnectionType is New or Reused
	ConnectionType string

	DisruptedDuration  metav1.Duration
	DisruptionMessages []string

	// New disruption test framework is introducing these fields, for
	// previous version of the test, these fields will default:
	//   LoadBalancerType will default to "external-lb"
	//   Protocol will default to http1
	//   TargetAPI will default to an empty string
	LoadBalancerType string
	Protocol         string
	TargetAPI        string
}

func writeDisruptionData(filename string, disruption *BackendDisruptionList) error {
	jsonContent, err := json.MarshalIndent(disruption, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, jsonContent, 0644)
}

func computeDisruptionData(eventIntervals monitorapi.Intervals) *BackendDisruptionList {
	ret := &BackendDisruptionList{
		BackendDisruptions: map[string]*BackendDisruption{},
	}

	allBackendLocators := sets.String{}
	allDisruptionEventsIntervals := eventIntervals.Filter(
		monitorapi.And(
			monitorapi.IsDisruptionEvent,
			monitorapi.Or(
				monitorapi.IsErrorEvent, // ignore Warning events, we use these for disruption we don't actually think was from the cluster under test (i.e. DNS)
				monitorapi.IsInfoEvent,  // Must keep including info disruption events as 0s disruptions don't get recorded otherwise
			),
		),
	)
	for _, eventInterval := range allDisruptionEventsIntervals {
		allBackendLocators.Insert(eventInterval.Locator)
	}

	for _, locator := range allBackendLocators.List() {
		locatorParts := monitorapi.LocatorParts(locator)
		disruptionBackend := monitorapi.DisruptionFrom(locatorParts)

		connectionType := monitorapi.DisruptionConnectionTypeFrom(locatorParts)
		aggregatedDisruptionName := strings.ToLower(fmt.Sprintf("%s-%s-connections", disruptionBackend, connectionType))

		// load-balancer has been introduced in the new disruption test framework
		loadBalancerType := monitorapi.DisruptionLoadBalancerTypeFrom(locatorParts)
		if len(loadBalancerType) > 0 {
			// the name is unique and has all the descriptors including connection type
			aggregatedDisruptionName = disruptionBackend
		}

		disruptionDuration, disruptionMessages, connectionType :=
			monitorapi.BackendDisruptionSeconds(locator, allDisruptionEventsIntervals)
		bs := &BackendDisruption{
			Name:               aggregatedDisruptionName,
			BackendName:        disruptionBackend,
			ConnectionType:     strings.Title(connectionType),
			DisruptedDuration:  metav1.Duration{Duration: disruptionDuration},
			DisruptionMessages: disruptionMessages,
			LoadBalancerType:   "external-lb",
			Protocol:           "http1",
			// for existing disruption test, the 'disruption' locator
			// part closely resembles the api being tested.
			TargetAPI: disruptionBackend,
		}
		ret.BackendDisruptions[aggregatedDisruptionName] = bs

		if len(loadBalancerType) > 0 {
			bs.LoadBalancerType = loadBalancerType
			bs.Protocol = monitorapi.DisruptionProtocolFrom(locatorParts)
			bs.TargetAPI = monitorapi.DisruptionTargetAPIFrom(locatorParts)
			bs.BackendName = fmt.Sprintf("%s-%s-%s", bs.TargetAPI, bs.Protocol, bs.LoadBalancerType)
		}
	}

	return ret
}

func WasMasterNodeUpdated(events monitorapi.Intervals) string {
	nodeUpdates := events.Filter(monitorapi.NodeUpdate)

	for _, i := range nodeUpdates {
		// "locator": "node/ip-10-0-240-32.us-west-1.compute.internal",
		//            "message": "reason/NodeUpdate phase/Update config/rendered-master-757d729d8565a6f9f4e59913d4731db1 roles/control-plane,master reached desired config roles/control-plane,master",
		// vs
		// "locator": "node/ip-10-0-228-209.us-west-1.compute.internal",
		//            "message": "reason/NodeUpdate phase/Update config/rendered-worker-722803a00bad408ee94572ab244ad3bc roles/worker reached desired config roles/worker",
		if strings.Contains(i.Message, "master") {
			return "Y"
		}
	}

	return "N"
}

func WriteClusterData(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	return writeClusterData(filepath.Join(artifactDir, fmt.Sprintf("cluster-data%s.json", timeSuffix)), CollectClusterData(WasMasterNodeUpdated(events)))
}

func writeClusterData(filename string, clusterData platformidentification.ClusterData) error {
	jsonContent, err := json.MarshalIndent(clusterData, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, jsonContent, 0644)
}

func CollectClusterData(masterNodeUpdated string) platformidentification.ClusterData {
	clusterData := platformidentification.ClusterData{}
	var errs *[]error
	restConfig, err := GetMonitorRESTConfig()
	if err != nil {
		return clusterData
	}

	clusterData, errs = platformidentification.BuildClusterData(context.TODO(), restConfig)

	if errs != nil {
		for _, err := range *errs {
			e2e.Logf("Error building cluster data: %s", err.Error())
		}
		e2e.Logf("Ignoring cluster data due to previous errors: %v", clusterData)
		return platformidentification.ClusterData{}
	}

	clusterData.MasterNodesUpdated = masterNodeUpdated
	return clusterData
}
