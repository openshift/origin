package disruptioningress

import (
	"context"
	_ "embed"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	configclient "github.com/openshift/client-go/config/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type availability struct {
	disruptionCheckers []*disruptionlibrary.Availability
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

	// Ingress monitoring checks for oauth and console routes to monitor healthz endpoints. Check availability
	// before setting up any monitors.
	routeClient, err := routeclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	_, err = routeClient.RouteV1().Routes(oauthRouteNamespace).Get(ctx, oauthRouteName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
	case err != nil:
		return err
	default:
		newConnectionTestName := "[sig-network-edge] ns/openshift-authentication route/oauth-openshift disruption/ingress-to-oauth-server connection/new should be available throughout the test"
		reusedConnectionTestName := "[sig-network-edge] ns/openshift-authentication route/oauth-openshift disruption/ingress-to-oauth-server connection/reused should be available throughout the test"
		newConnectionDisruptionSampler := createOAuthRouteAvailableWithNewConnections(adminRESTConfig)
		reusedConnectionDisruptionSampler := createOAuthRouteAvailableWithConnectionReuse(adminRESTConfig)

		disruptionChecker := disruptionlibrary.NewAvailabilityInvariant(
			newConnectionTestName, reusedConnectionTestName,
			newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
		)
		w.disruptionCheckers = append(w.disruptionCheckers, disruptionChecker)
	}

	configAvailable, err := exutil.DoesApiResourceExist(adminRESTConfig, "clusterversions", "config.openshift.io")
	switch {
	case err != nil:
		return err
	case configAvailable:
		// Some jobs explicitly disable the console and other features. Check if it's disabled and if so,
		// do not run a disruption monitoring backend for it.
		configClient, err := configclient.NewForConfig(adminRESTConfig)
		if err != nil {
			return err
		}
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Failed to get cluster version: %v", err)
		}
		// If the cluster does not know about the Console capability, it likely predates 4.12 and we can assume
		// it has it by default. This is to catch possible future scenarios where we upgrade 4.11 no cap to 4.12 no cap.
		if hasCapability(clusterVersion, "Console") {
			newConnectionTestName := "[sig-network-edge] ns/openshift-console route/console disruption/ingress-to-console connection/new should be available throughout the test"
			reusedConnectionTestName := "[sig-network-edge] ns/openshift-console route/console disruption/ingress-to-console connection/reused should be available throughout the test"
			newConnectionDisruptionSampler := CreateConsoleRouteAvailableWithNewConnections(adminRESTConfig)
			reusedConnectionDisruptionSampler := createConsoleRouteAvailableWithConnectionReuse(adminRESTConfig)

			disruptionChecker := disruptionlibrary.NewAvailabilityInvariant(
				newConnectionTestName, reusedConnectionTestName,
				newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
			)
			w.disruptionCheckers = append(w.disruptionCheckers, disruptionChecker)

		}
	}

	for i := range w.disruptionCheckers {
		if err := w.disruptionCheckers[i].StartCollection(ctx, adminRESTConfig, recorder); err != nil {
			return err
		}
	}

	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for i := range w.disruptionCheckers {
		// we failed and indicated it during setup.
		if w.disruptionCheckers[i] == nil {
			continue
		}

		localIntervals, localJunits, localErr := w.disruptionCheckers[i].CollectData(ctx)
		intervals = append(intervals, localIntervals...)
		junits = append(junits, localJunits...)
		if localErr != nil {
			errs = append(errs, localErr)
		}
	}

	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (*availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.suppressJunit {
		return nil, nil
	}

	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for i := range w.disruptionCheckers {
		// we failed and indicated it during setup.
		if w.disruptionCheckers[i] == nil {
			continue
		}

		localJunits, localErr := w.disruptionCheckers[i].EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
		junits = append(junits, localJunits...)
		if localErr != nil {
			errs = append(errs, localErr)
		}
	}

	return junits, utilerrors.NewAggregate(errs)
}

func (*availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *availability) Cleanup(ctx context.Context) error {
	return nil
}

const (
	oauthRouteNamespace = "openshift-authentication"
	oauthRouteName      = "oauth-openshift"
)

func hasCapability(clusterVersion *configv1.ClusterVersion, desiredCapability string) bool {
	for _, ec := range clusterVersion.Status.Capabilities.EnabledCapabilities {
		if string(ec) == desiredCapability {
			return true
		}
	}
	return false
}

func createOAuthRouteAvailableWithNewConnections(adminRESTConfig *rest.Config) *backenddisruption.BackendSampler {
	return backenddisruption.NewRouteBackend(
		adminRESTConfig,
		oauthRouteNamespace,
		oauthRouteName,
		"ingress-to-oauth-server",
		"/healthz",
		monitorapi.NewConnectionType).
		WithExpectedBody("ok")
}

func createOAuthRouteAvailableWithConnectionReuse(adminRESTConfig *rest.Config) *backenddisruption.BackendSampler {
	return backenddisruption.NewRouteBackend(
		adminRESTConfig,
		oauthRouteNamespace,
		oauthRouteName,
		"ingress-to-oauth-server",
		"/healthz",
		monitorapi.ReusedConnectionType).
		WithExpectedBody("ok")
}

func CreateConsoleRouteAvailableWithNewConnections(adminRESTConfig *rest.Config) *backenddisruption.BackendSampler {
	return backenddisruption.NewRouteBackend(
		adminRESTConfig,
		"openshift-console",
		"console",
		"ingress-to-console",
		"/healthz",
		monitorapi.NewConnectionType).
		WithExpectedBodyRegex(`(Red Hat OpenShift|OKD)`)
}

func createConsoleRouteAvailableWithConnectionReuse(adminRESTConfig *rest.Config) *backenddisruption.BackendSampler {
	return backenddisruption.NewRouteBackend(
		adminRESTConfig,
		"openshift-console",
		"console",
		"ingress-to-console",
		"/healthz",
		monitorapi.ReusedConnectionType).
		WithExpectedBodyRegex(`(Red Hat OpenShift|OKD)`)
}
