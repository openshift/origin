package disruptionimageregistry

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/imageregistryutil"
)

const (
	newConnectionTestName    = "[sig-imageregistry] disruption/image-registry connection/new should be available throughout the test"
	reusedConnectionTestName = "[sig-imageregistry] disruption/image-registry connection/reused should be available throughout the test"
)

type availability struct {
	kubeClient         kubernetes.Interface
	routeClient        routeclient.Interface
	imageRegistryRoute *routev1.Route

	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason error
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

func (w *availability) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error

	namespace := "openshift-image-registry"
	imageRegistryDeploymentName := "image-registry"

	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	_, err = w.kubeClient.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "namespace openshift-image-registry not present"}
		return w.notSupportedReason
	}
	if err != nil {
		return err
	}

	deployment, err := w.kubeClient.AppsV1().Deployments(namespace).Get(ctx, imageRegistryDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 1 {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "image-registry only has a single replica"}
		return w.notSupportedReason
	}

	w.routeClient, err = routeclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	w.imageRegistryRoute, err = imageregistryutil.ExposeImageRegistryGenerateName(ctx, w.routeClient, "test-disruption-")
	if err != nil {
		return err
	}

	baseURL := fmt.Sprintf("https://%s", w.imageRegistryRoute.Status.Ingress[0].Host)
	historicalBackendDisruptionDataForNewConnectionsName := "image-registry-new-connections"
	historicalBackendDisruptionDataForReusedConnectionsName := "image-registry-reused-connections"
	path := "/healthz"
	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendWithLocator(
		monitorapi.NewLocator().LocateRouteForDisruptionCheck(historicalBackendDisruptionDataForNewConnectionsName, backenddisruption.OpenshiftTestsSource, namespace, "test-disruption-new", monitorapi.NewConnectionType),
		baseURL,
		path,
		monitorapi.NewConnectionType)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendWithLocator(
		monitorapi.NewLocator().LocateRouteForDisruptionCheck(historicalBackendDisruptionDataForReusedConnectionsName, backenddisruption.OpenshiftTestsSource, namespace, "test-disruption-reused", monitorapi.ReusedConnectionType),
		baseURL,
		path,
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
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}
	// we failed and indicated it during setup.
	if w.disruptionChecker == nil {
		return nil, nil, nil
	}

	return w.disruptionChecker.CollectData(ctx)
}

func (*availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	if w.suppressJunit {
		return nil, nil
	}
	// we failed and indicated it during setup.
	if w.disruptionChecker == nil {
		return nil, nil
	}

	return w.disruptionChecker.EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
}

func (w *availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return w.notSupportedReason
}

func (w *availability) routeDeleted(ctx context.Context) (bool, error) {
	_, err := w.routeClient.RouteV1().Routes("openshift-image-registry").Get(ctx, w.imageRegistryRoute.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		klog.Errorf("Error checking for deleted route: %s, %s", w.imageRegistryRoute.Name, err.Error())
		return false, err
	}

	return false, nil
}

func (w *availability) Cleanup(ctx context.Context) error {
	if w.imageRegistryRoute != nil {
		err := w.routeClient.RouteV1().Routes("openshift-image-registry").Delete(ctx, w.imageRegistryRoute.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete route: %w", err)
		}

		startTime := time.Now()
		err = wait.PollUntilContextTimeout(ctx, 15*time.Second, 20*time.Minute, true, w.routeDeleted)
		if err != nil {
			return err
		}

		klog.Infof("Deleting route: %s took %.2f seconds", w.imageRegistryRoute.Name, time.Now().Sub(startTime).Seconds())

	}

	return w.notSupportedReason
}
