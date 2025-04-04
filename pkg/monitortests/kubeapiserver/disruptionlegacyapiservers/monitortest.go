package disruptionlegacyapiservers

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type availability struct {
	disruptionCheckers []*disruptionlibrary.Availability

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
func testNames(owner, disruptionBackendName string) (string, string) {
	return fmt.Sprintf("[%s] disruption/%s connection/new should be available throughout the test", owner, disruptionBackendName),
		fmt.Sprintf("[%s] disruption/%s connection/reused should be available throughout the test", owner, disruptionBackendName)
}

func newDisruptionCheckerForKubeAPI(adminRESTConfig *rest.Config) (*disruptionlibrary.Availability, error) {
	disruptionBackedName := "kube-api"
	newConnectionTestName, reusedConnectionTestName := testNames("sig-api-machinery", disruptionBackedName)
	newConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/api/v1/namespaces/default", monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/api/v1/namespaces/default", monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
}

func newDisruptionCheckerForKubeAPICached(adminRESTConfig *rest.Config) (*disruptionlibrary.Availability, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.

	disruptionBackedName := "cache-kube-api"
	newConnectionTestName, reusedConnectionTestName := testNames("sig-api-machinery", disruptionBackedName)
	newConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/api/v1/namespaces/default?resourceVersion=0", monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/api/v1/namespaces/default?resourceVersion=0", monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
}

func newDisruptionCheckerForOpenshiftAPI(adminRESTConfig *rest.Config) (*disruptionlibrary.Availability, error) {
	disruptionBackedName := "openshift-api"
	newConnectionTestName, reusedConnectionTestName := testNames("sig-api-machinery", disruptionBackedName)
	newConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/image.openshift.io/v1/namespaces/default/imagestreams", monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/image.openshift.io/v1/namespaces/default/imagestreams", monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
}

func newDisruptionCheckerForOpenshiftAPICached(adminRESTConfig *rest.Config) (*disruptionlibrary.Availability, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.

	disruptionBackedName := "cache-openshift-api"
	newConnectionTestName, reusedConnectionTestName := testNames("sig-api-machinery", disruptionBackedName)
	newConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/image.openshift.io/v1/namespaces/default/imagestreams?resourceVersion=0", monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/image.openshift.io/v1/namespaces/default/imagestreams?resourceVersion=0", monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
}

func newDisruptionCheckerForOAuthAPI(adminRESTConfig *rest.Config) (*disruptionlibrary.Availability, error) {
	disruptionBackedName := "oauth-api"
	newConnectionTestName, reusedConnectionTestName := testNames("sig-api-machinery", disruptionBackedName)
	newConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/oauth.openshift.io/v1/oauthclients", monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/oauth.openshift.io/v1/oauthclients", monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
}

func newDisruptionCheckerForOAuthCached(adminRESTConfig *rest.Config) (*disruptionlibrary.Availability, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.

	disruptionBackedName := "cache-oauth-api"
	newConnectionTestName, reusedConnectionTestName := testNames("sig-api-machinery", disruptionBackedName)
	newConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/oauth.openshift.io/v1/oauthclients?resourceVersion=0", monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createAPIServerBackendSampler(adminRESTConfig, disruptionBackedName, "/apis/oauth.openshift.io/v1/oauthclients?resourceVersion=0", monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
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

	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), "openshift-apiserver", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "namespace openshift-apiserver not present",
		}
		return w.notSupportedReason
	}
	if err != nil {
		return err
	}
	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), "openshift-oauth-apiserver", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "namespace openshift-oauth-apiserver not present",
		}
		return w.notSupportedReason
	}
	if err != nil {
		return err
	}

	var curr *disruptionlibrary.Availability

	curr, err = newDisruptionCheckerForKubeAPI(adminRESTConfig)
	if err != nil {
		return err
	}
	w.disruptionCheckers = append(w.disruptionCheckers, curr)
	curr, err = newDisruptionCheckerForKubeAPICached(adminRESTConfig)
	if err != nil {
		return err
	}
	w.disruptionCheckers = append(w.disruptionCheckers, curr)

	curr, err = newDisruptionCheckerForOpenshiftAPI(adminRESTConfig)
	if err != nil {
		return err
	}
	w.disruptionCheckers = append(w.disruptionCheckers, curr)
	curr, err = newDisruptionCheckerForOpenshiftAPICached(adminRESTConfig)
	if err != nil {
		return err
	}
	w.disruptionCheckers = append(w.disruptionCheckers, curr)

	curr, err = newDisruptionCheckerForOAuthAPI(adminRESTConfig)
	if err != nil {
		return err
	}
	w.disruptionCheckers = append(w.disruptionCheckers, curr)
	curr, err = newDisruptionCheckerForOAuthCached(adminRESTConfig)
	if err != nil {
		return err
	}
	w.disruptionCheckers = append(w.disruptionCheckers, curr)

	for i := range w.disruptionCheckers {
		if err := w.disruptionCheckers[i].StartCollection(ctx, adminRESTConfig, recorder); err != nil {
			return err
		}
	}

	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

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

func (w *availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}

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

func (w *availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return w.notSupportedReason
}

func (w *availability) Cleanup(ctx context.Context) error {
	return w.notSupportedReason
}
