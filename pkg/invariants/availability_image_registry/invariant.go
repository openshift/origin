package availability_image_registry

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/openshift/origin/test/extended/util/imageregistryutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	newConnectionTestName    = "[sig-imageregistry] Image registry remains available using new connections"
	reusedConnectionTestName = "[sig-imageregistry] Image registry remains available using reused connections"
)

type availability struct {
	kubeClient         kubernetes.Interface
	routeClient        routeclient.Interface
	imageRegistryRoute *routev1.Route

	newConnectionDisruptionSampler    *backenddisruption.BackendSampler
	reusedConnectionDisruptionSampler *backenddisruption.BackendSampler

	// TODO stop storing this and use clients so we can unit test if we want
	adminRESTConfig *rest.Config
}

func NewAvailabilityInvariant() invariants.InvariantTest {
	return &availability{}
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error

	w.adminRESTConfig = adminRESTConfig
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	w.routeClient, err = routeclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	routeName := "test-disruption"
	w.imageRegistryRoute, err = imageregistryutil.ExposeImageRegistry(ctx, w.routeClient, routeName)
	if err != nil {
		return err
	}

	baseURL := fmt.Sprintf("https://%s", w.imageRegistryRoute.Status.Ingress[0].Host)
	namespace := "openshift-image-registry"
	disruptionBackendName := "image-registry"
	path := "/healthz"
	w.newConnectionDisruptionSampler =
		backenddisruption.NewSimpleBackendWithLocator(
			monitorapi.LocateRouteForDisruptionCheck(namespace, "test-disruption-new", disruptionBackendName, monitorapi.NewConnectionType),
			baseURL,
			disruptionBackendName,
			path,
			monitorapi.NewConnectionType)

	w.reusedConnectionDisruptionSampler =
		backenddisruption.NewSimpleBackendWithLocator(
			monitorapi.LocateRouteForDisruptionCheck(namespace, "test-disruption-reused", disruptionBackendName, monitorapi.ReusedConnectionType),
			baseURL,
			disruptionBackendName,
			path,
			monitorapi.ReusedConnectionType)
	if err := w.newConnectionDisruptionSampler.StartEndpointMonitoring(ctx, recorder, nil); err != nil {
		return err
	}
	if err := w.reusedConnectionDisruptionSampler.StartEndpointMonitoring(ctx, recorder, nil); err != nil {
		return err
	}

	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// when it is time to collect data, we need to stop the collectors.  they both  have to drain, so stop in parallel
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		w.newConnectionDisruptionSampler.Stop()
	}()
	go func() {
		defer wg.Done()
		w.reusedConnectionDisruptionSampler.Stop()
	}()
	wg.Wait()

	return nil, nil, nil
}

func (*availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func createDisruptionJunit(testName string, allowedDisruption *time.Duration, disruptionDetails, locator string, disruptedIntervals monitorapi.Intervals) *junitapi.JUnitTestCase {
	// Indicates there is no entry in the query_results.json data file, nor a valid fallback,
	// we do not wish to run the test. (this likely implies we do not have the required number of
	// runs in 3 weeks to do a reliable P99)
	if allowedDisruption == nil {
		return &junitapi.JUnitTestCase{
			Name: testName,
			SkipMessage: &junitapi.SkipMessage{
				Message: "No historical data to calculate allowedDisruption",
			},
		}
	}

	if *allowedDisruption < 1*time.Second {
		t := 1 * time.Second
		allowedDisruption = &t
		disruptionDetails = "always allow at least one second"
	}

	disruptionDuration := disruptedIntervals.Duration(1 * time.Second)
	roundedAllowedDisruption := allowedDisruption.Round(time.Second)
	roundedDisruptionDuration := disruptionDuration.Round(time.Second)

	if roundedDisruptionDuration <= roundedAllowedDisruption {
		return &junitapi.JUnitTestCase{
			Name: testName,
		}
	}

	reason := fmt.Sprintf("%s was unreachable during disruption: %v", locator, disruptionDetails)
	describe := disruptedIntervals.Strings()
	failureMessage := fmt.Sprintf("%s for at least %s (maxAllowed=%s):\n\n%s", reason, roundedDisruptionDuration, roundedAllowedDisruption, strings.Join(describe, "\n"))

	return &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: failureMessage,
		},
		SystemOut: failureMessage,
	}
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}

	// If someone feels motivated to DRY out further, find a way to abstract for ALL disruption.  I don't see it yet.
	// DO NOT MERGE something that just makes new and reused look the same.  That's a waste of time and willmake drift worse.
	{ // block to prevent cross-contamination.
		newConnectionAllowed, newConnectionDisruptionDetails, err := disruption.HistoricalAllowedDisruption(ctx, w.newConnectionDisruptionSampler, w.adminRESTConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to get new allowed disruption: %w", err)
		}
		junits = append(junits,
			createDisruptionJunit(
				newConnectionTestName, newConnectionAllowed, newConnectionDisruptionDetails, w.newConnectionDisruptionSampler.GetLocator(),
				finalIntervals.Filter(
					monitorapi.And(
						monitorapi.IsEventForLocator(w.newConnectionDisruptionSampler.GetLocator()),
						monitorapi.IsErrorEvent,
					),
				),
			),
		)
	}

	{ // block to prevent cross-contamination
		reusedConnectionAllowed, reusedConnectionDisruptionDetails, err := disruption.HistoricalAllowedDisruption(ctx, w.reusedConnectionDisruptionSampler, w.adminRESTConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to get reused allowed disruption: %w", err)
		}
		junits = append(junits,
			createDisruptionJunit(
				reusedConnectionTestName, reusedConnectionAllowed, reusedConnectionDisruptionDetails, w.reusedConnectionDisruptionSampler.GetLocator(),
				finalIntervals.Filter(
					monitorapi.And(
						monitorapi.IsEventForLocator(w.reusedConnectionDisruptionSampler.GetLocator()),
						monitorapi.IsErrorEvent,
					),
				),
			),
		)
	}

	return junits, nil
}

func (*availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *availability) Cleanup(ctx context.Context) error {
	if w.imageRegistryRoute != nil {
		err := w.routeClient.RouteV1().Routes("openshift-image-registry").Delete(ctx, w.imageRegistryRoute.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete route: %w", err)
		}
	}

	return nil
}
