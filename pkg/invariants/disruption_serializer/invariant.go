package disruption_serializer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type disruptionSummarySerializer struct {
}

func NewDisruptionSummarySerializer() invariants.InvariantTest {
	return &disruptionSummarySerializer{}
}

func (w *disruptionSummarySerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *disruptionSummarySerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*disruptionSummarySerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*disruptionSummarySerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*disruptionSummarySerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	backendDisruption := computeDisruptionData(finalIntervals)
	return writeDisruptionData(filepath.Join(storageDir, fmt.Sprintf("backend-disruption%s.json", timeSuffix)), backendDisruption)
}

func (*disruptionSummarySerializer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
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
