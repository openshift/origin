package disruptionnewapiserver

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

type newAPIServerDisruptionChecker struct {
	adminRESTConfig    *rest.Config
	notSupportedReason error
}

func NewDisruptionInvariant() monitortestframework.MonitorTest {
	return &newAPIServerDisruptionChecker{}
}

func (w *newAPIServerDisruptionChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *newAPIServerDisruptionChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig

	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
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
	}
	return w.notSupportedReason
}

func (w *newAPIServerDisruptionChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}
	apiserverAvailabilityIntervals, err := apiserveravailability.APIServerAvailabilityIntervalsFromCluster(kubeClient, beginning, end)

	return apiserverAvailabilityIntervals, nil, err
}

func (w *newAPIServerDisruptionChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *newAPIServerDisruptionChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, w.notSupportedReason
}

func (w *newAPIServerDisruptionChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return w.notSupportedReason
}

func (w *newAPIServerDisruptionChecker) Cleanup(ctx context.Context) error {
	return nil
}
