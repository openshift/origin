package disruptionexternalawscloudservicemonitoring

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/kubernetes"
)

const (
	newCloudConnectionTestName    = "[sig-trt] disruption/aws-network-liveness connection/new should be available throughout the test"
	reusedCloudConnectionTestName = "[sig-trt] disruption/aws-network-liveness connection/reused should be available throughout the test"

	// Load balancer URL
	externalServiceURL = "http://trt-openshift-tests-endpoint-lb-1161093811.us-east-1.elb.amazonaws.com/health"
)

type cloudAvailability struct {
	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason error
	suppressJunit      bool
}

func NewCloudAvailabilityInvariant() monitortestframework.MonitorTest {
	var notSupportedReason error

	return &cloudAvailability{
		suppressJunit:      true,
		notSupportedReason: notSupportedReason,
	}
}

func (w *cloudAvailability) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *cloudAvailability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	{
		kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
		if err != nil {
			return err
		}
		isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
		if err != nil {
			return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
		}
		if isMicroShift {
			w.notSupportedReason = &monitortestframework.NotSupportedError{
				Reason: "platform MicroShift not supported",
			}
			return w.notSupportedReason
		}
	}
	// Proxy jobs may require a whitelist we don't want to deal with:
	clusterState, err := clusterdiscovery.DiscoverClusterState(adminRESTConfig)
	if err != nil {
		logrus.WithError(err).Error("error loading cluster state")
		return err
	}
	clusterConfig, err := clusterdiscovery.LoadConfig(clusterState)
	if err != nil {
		logrus.WithError(err).Error("error loading cluster config")
		return err
	}
	if clusterConfig.IsProxied {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "aws-network-liveness disruption monitor is disabled when HTTP_PROXY is in use"}
		return w.notSupportedReason
	}

	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"aws-network-liveness-new-connections",
		"",
		monitorapi.NewConnectionType)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"aws-network-liveness-reused-connections",
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
