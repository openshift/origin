package disruptionmetricsapi

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	monitoringNamespace         = "openshift-monitoring"
	metricsServerDeploymentName = "metrics-server"
)

type availability struct {
	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason error
}

func NewAvailabilityInvariant() monitortestframework.MonitorTest {
	return &availability{}
}

func createBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType monitorapi.BackendConnectionType) (*backenddisruption.BackendSampler, error) {
	backendSampler, err := backenddisruption.NewAPIServerBackend(clusterConfig, disruptionBackendName, url, connectionType)
	if err != nil {
		return nil, err
	}
	backendSampler = backendSampler.WithUserAgent(fmt.Sprintf("openshift-external-backend-sampler-%s-%s", connectionType, disruptionBackendName))
	return backendSampler, nil
}

func (w *availability) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error

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

	deploymentScale, err := kubeClient.AppsV1().Deployments(monitoringNamespace).GetScale(ctx, metricsServerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Skip for single replica Deployments.
	if deploymentScale.Spec.Replicas == 1 {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf("%s only has a single replica", deploymentScale.Name)}
		return w.notSupportedReason
	}

	disruptionBackedName := "metrics-api"
	newConnectionTestName := fmt.Sprintf("[sig-instrumentation] disruption/%s connection/new should be available throughout the test", disruptionBackedName)
	reusedConnectionTestName := fmt.Sprintf("[sig-instrumentation] disruption/%s connection/reused should be available throughout the test", disruptionBackedName)
	url := "/apis/metrics.k8s.io/v1beta1"

	newConnections, err := createBackendSampler(adminRESTConfig, disruptionBackedName, url, monitorapi.NewConnectionType)
	if err != nil {
		return err
	}
	reusedConnections, err := createBackendSampler(adminRESTConfig, disruptionBackedName, url, monitorapi.ReusedConnectionType)
	if err != nil {
		return err
	}

	w.disruptionChecker = disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	)

	if err := w.disruptionChecker.StartCollection(ctx, adminRESTConfig, recorder); err != nil {
		return err
	}

	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}
	// we failed and indicated it during setup.
	if w.disruptionChecker == nil {
		return nil, nil, nil
	}

	return w.disruptionChecker.CollectData(ctx)
}

func (w *availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	// we failed and indicated it during setup.
	if w.disruptionChecker == nil {
		return nil, nil
	}

	return w.disruptionChecker.EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
}

func (w *availability) WriteContentToStorage(ctx context.Context, storageDir string, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return w.notSupportedReason
}

func (w *availability) Cleanup(ctx context.Context) error {
	return w.notSupportedReason
}
