package disruptionexternalgcpcloudservicemonitoring

import (
	"context"
	_ "embed"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	newCloudConnectionTestName    = "[sig-trt] disruption/gcp-network-liveness connection/new should be available throughout the test"
	reusedCloudConnectionTestName = "[sig-trt] disruption/gcp-network-liveness connection/reused should be available throughout the test"

	// Cloud function URL
	//externalServiceURL = "https://us-east4-openshift-gce-devel.cloudfunctions.net/openshift-tests-endpoint"

	// Load balancer URL
	externalServiceURL = "http://34.150.152.77/health"
)

type cloudAvailability struct {
	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason error
	suppressJunit      bool
}

func NewCloudAvailabilityInvariant() monitortestframework.MonitorTest {
	return &cloudAvailability{}
}

func NewRecordCloudAvailabilityOnly() monitortestframework.MonitorTest {
	return &cloudAvailability{
		suppressJunit: true,
	}
}

func (w *cloudAvailability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"gcp-network-liveness-new-connections",
		"",
		monitorapi.NewConnectionType)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"gcp-network-liveness-reused-connections",
		"",
		monitorapi.ReusedConnectionType)

	w.disruptionChecker = disruptionlibrary.NewAvailabilityInvariant(
		newCloudConnectionTestName, reusedCloudConnectionTestName,
		newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
	)
	if err := w.disruptionChecker.StartCollection(ctx, adminRESTConfig, recorder); err != nil {
		return err
	}

	return nil
}

func (w *cloudAvailability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}
	return w.disruptionChecker.CollectData(ctx)
}

func (w *cloudAvailability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *cloudAvailability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.suppressJunit {
		return nil, nil
	}

	return nil, w.notSupportedReason
}

func (w *cloudAvailability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return w.notSupportedReason
}

func (w *cloudAvailability) Cleanup(ctx context.Context) error {
	return w.notSupportedReason
}
