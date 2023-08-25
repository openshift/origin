package disruptionnewapiserver

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type newAPIServerDisruptionChecker struct {
	adminRESTConfig    *rest.Config
	notSupportedReason string
}

func NewDisruptionInvariant() monitortestframework.MonitorTest {
	return &newAPIServerDisruptionChecker{}
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
		w.notSupportedReason = "platform MicroShift not supported"
	}
	return nil
}

func (w *newAPIServerDisruptionChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if len(w.notSupportedReason) > 0 {
		return nil, nil, nil
	}
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}
	apiserverAvailabilityIntervals, err := apiserveravailability.APIServerAvailabilityIntervalsFromCluster(kubeClient, beginning, end)

	return apiserverAvailabilityIntervals, nil, err
}

func (*newAPIServerDisruptionChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*newAPIServerDisruptionChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *newAPIServerDisruptionChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *newAPIServerDisruptionChecker) Cleanup(ctx context.Context) error {
	if len(w.notSupportedReason) > 0 {
		return nil
	}

	if err := sampler.TearDownInClusterMonitors(w.adminRESTConfig); err != nil {
		return err
	}
	return nil
}
