package disruptionexternalservicemonitoring

import (
	"context"
	_ "embed"
	"time"

	"github.com/openshift/origin/pkg/invariantlibrary/disruptionlibrary"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/imageregistryutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	newConnectionTestName    = "[sig-trt] disruption/ci-cluster-network-liveness connection/new should be available throughout the test"
	reusedConnectionTestName = "[sig-trt] disruption/ci-cluster-network-liveness connection/reused should be available throughout the test"

	LivenessProbeBackend = "ci-cluster-network-liveness"
	externalServiceURL   = "http://static.redhat.com/test/rhel-networkmanager.txt"
)

type availability struct {
	kubeClient         kubernetes.Interface
	routeClient        routeclient.Interface
	imageRegistryRoute *routev1.Route

	disruptionChecker *disruptionlibrary.Availability
	suppressJunit     bool
}

func NewAvailabilityInvariant() invariants.InvariantTest {
	return &availability{}
}

func NewRecordAvailabilityOnly() invariants.InvariantTest {
	return &availability{
		suppressJunit: true,
	}
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error

	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	w.routeClient, err = routeclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	w.imageRegistryRoute, err = imageregistryutil.ExposeImageRegistryGenerateName(ctx, w.routeClient, "test-disruption-")
	if err != nil {
		return err
	}

	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackend(
		externalServiceURL,
		LivenessProbeBackend,
		"",
		monitorapi.NewConnectionType)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackend(
		externalServiceURL,
		LivenessProbeBackend,
		"",
		monitorapi.ReusedConnectionType)

	w.disruptionChecker = disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
	)
	if err := w.disruptionChecker.StartCollection(ctx, adminRESTConfig, recorder); err != nil {
		return err
	}

	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return w.disruptionChecker.CollectData(ctx)
}

func (*availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.suppressJunit {
		return nil, nil
	}

	return nil, nil
}

func (*availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *availability) Cleanup(ctx context.Context) error {
	return nil
}
