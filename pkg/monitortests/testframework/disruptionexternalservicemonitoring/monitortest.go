package disruptionexternalservicemonitoring

import (
	"context"
	_ "embed"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/imageregistryutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	newConnectionTestName    = "[sig-trt] disruption/ci-cluster-network-liveness connection/new should be available throughout the test"
	reusedConnectionTestName = "[sig-trt] disruption/ci-cluster-network-liveness connection/reused should be available throughout the test"

	externalServiceURL = "http://static.redhat.com/test/rhel-networkmanager.txt"
)

type availability struct {
	kubeClient         kubernetes.Interface
	routeClient        routeclient.Interface
	imageRegistryRoute *routev1.Route

	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason string
	suppressJunit      bool
}

func NewAvailabilityInvariant() monitortestframework.MonitorTest {
	return &availability{}
}

func NewRecordAvailabilityOnly() monitortestframework.MonitorTest {
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

	_, err = w.kubeClient.CoreV1().Namespaces().Get(context.Background(), "openshift-image-registry", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		w.notSupportedReason = "namespace openshift-image-registry not present"
		return nil
	}
	if err != nil {
		return err
	}
	w.imageRegistryRoute, err = imageregistryutil.ExposeImageRegistryGenerateName(ctx, w.routeClient, "test-disruption-")
	if err != nil {
		return err
	}

	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"ci-cluster-network-liveness-new-connections",
		"",
		monitorapi.NewConnectionType)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"ci-cluster-network-liveness-reused-connections",
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
	if len(w.notSupportedReason) > 0 {
		return nil, nil, nil
	}
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
