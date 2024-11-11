package disruptionexternalapiserver

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"

	imagev1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	oauthv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type InvariantExternalDisruption struct {
	notSupportedReason error
	disruptionCheckers []*disruptionlibrary.Availability

	adminRESTConfig *rest.Config
	kubeClient      kubernetes.Interface
}

func NewExternalDisruptionInvariant(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &InvariantExternalDisruption{}
}

func createBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType monitorapi.BackendConnectionType) (*backenddisruption.BackendSampler, error) {
	backendSampler, err := backenddisruption.NewAPIServerBackend(clusterConfig, disruptionBackendName, url, connectionType)
	if err != nil {
		return nil, err
	}
	backendSampler = backendSampler.WithUserAgent(fmt.Sprintf("openshift-external-backend-sampler-%s-%s", connectionType, disruptionBackendName))
	return backendSampler, nil
}

func testNames(disruptionBackendName, apiserver string) (string, string) {
	return fmt.Sprintf("[sig-api-machinery] disruption/%s apiserver/%s connection/%s should be available throughout the test", disruptionBackendName, apiserver, "new"),
		fmt.Sprintf("[sig-api-machinery] disruption/%s apiserver/%s connection/%s should be available throughout the test", disruptionBackendName, apiserver, "reused")
}

func createApiServerChecker(adminRESTConfig *rest.Config, disruptionBackendName, apiserver, url string) (*disruptionlibrary.Availability, error) {
	newConnectionTestName, reusedConnectionTestName := testNames(disruptionBackendName, apiserver)

	newConnections, err := createBackendSampler(adminRESTConfig, disruptionBackendName, url, monitorapi.NewConnectionType)
	if err != nil {
		return nil, err
	}
	reusedConnections, err := createBackendSampler(adminRESTConfig, disruptionBackendName, url, monitorapi.ReusedConnectionType)
	if err != nil {
		return nil, err
	}
	return disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnections, reusedConnections,
	), nil
}

func createKubeApiChecker(adminRESTConfig *rest.Config, url string, cache bool) (*disruptionlibrary.Availability, error) {
	disruptionBackendName := "kube-api"
	if cache {
		disruptionBackendName = fmt.Sprintf("cache-%s", disruptionBackendName)
	}
	return createApiServerChecker(adminRESTConfig, disruptionBackendName, "kube-apiserver", url)
}

func createOpenshiftApiChecker(adminRESTConfig *rest.Config, url string, cache bool) (*disruptionlibrary.Availability, error) {
	disruptionBackendName := "openshift-api"
	if cache {
		disruptionBackendName = fmt.Sprintf("cache-%s", disruptionBackendName)
	}
	return createApiServerChecker(adminRESTConfig, disruptionBackendName, "openshift-apiserver", url)
}

func createOauthApiChecker(adminRESTConfig *rest.Config, url string, cache bool) (*disruptionlibrary.Availability, error) {
	disruptionBackendName := "oauth-api"
	if cache {
		disruptionBackendName = fmt.Sprintf("cache-%s", disruptionBackendName)
	}
	return createApiServerChecker(adminRESTConfig, disruptionBackendName, "oauth-apiserver", url)
}

func (i *InvariantExternalDisruption) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error

	log := logrus.WithField("monitorTest", "apiserver-external-availability").WithField("func", "StartCollection")
	log.Infof("starting external API monitors")

	i.adminRESTConfig = adminRESTConfig

	i.kubeClient, err = kubernetes.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(i.kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		i.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "platform MicroShift not supported",
		}
	}
	if i.notSupportedReason != nil {
		return i.notSupportedReason
	}

	namespaces, err := i.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list namespaces for cached kube api checker: %v", err)
	}
	namespacesListRevision := namespaces.ResourceVersion

	imageClient, err := imagev1.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return fmt.Errorf("unable to create imagestream client for openshift-apiserver api checker: %v", err)
	}

	imageStreamNS := "openshift"
	imagestreams, err := imageClient.ImageStreams(imageStreamNS).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list imagestreams for openshift-apiserver api checker: %v", err)
	}
	if len(imagestreams.Items) == 0 {
		return fmt.Errorf("found no suitable imagestream for openshift-apiserver api checker: %v", imagestreams)
	}
	imageStreamName := imagestreams.Items[0].Name
	imageStreamRevision := imagestreams.Items[0].ResourceVersion

	oauthClient, err := oauthv1.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return fmt.Errorf("unable to create oauth client for oauth-apiserver api checker: %v", err)
	}
	oauthclients, err := oauthClient.OAuthClients().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list oauth for oauth-apiserver api checker: %v", err)
	}
	if len(oauthclients.Items) == 0 {
		return fmt.Errorf("found no suitable imagestream for oauth-apiserver api checker: %v", err)
	}
	oauthClientName := oauthclients.Items[0].Name
	oauthClientRevision := oauthclients.Items[0].ResourceVersion

	kubeApiChecker, err := createKubeApiChecker(i.adminRESTConfig, "/api/v1/namespaces/default", false)
	if err != nil {
		return fmt.Errorf("unable to create kube api checker: %v", err)
	}
	i.disruptionCheckers = append(i.disruptionCheckers, kubeApiChecker)

	kubeApiCachedChecker, err := createKubeApiChecker(i.adminRESTConfig, fmt.Sprintf("/api/v1/namespaces/default?resourceVersion=%s", namespacesListRevision), true)
	if err != nil {
		return fmt.Errorf("unable to create cached kube api checker: %v", err)
	}
	i.disruptionCheckers = append(i.disruptionCheckers, kubeApiCachedChecker)

	openshiftApiChecker, err := createOpenshiftApiChecker(i.adminRESTConfig, fmt.Sprintf("/apis/image.openshift.io/v1/namespaces/%s/imagestreams", imageStreamNS), false)
	if err != nil {
		return fmt.Errorf("unable to create openshift api checker: %v", err)
	}
	i.disruptionCheckers = append(i.disruptionCheckers, openshiftApiChecker)

	openshiftApiCachedChecker, err := createOpenshiftApiChecker(i.adminRESTConfig, fmt.Sprintf("/apis/image.openshift.io/v1/namespaces/%s/imagestreams/%s?resourceVersion=%s", imageStreamNS, imageStreamName, imageStreamRevision), true)
	if err != nil {
		return fmt.Errorf("unable to create cached openshift api checker: %v", err)
	}
	i.disruptionCheckers = append(i.disruptionCheckers, openshiftApiCachedChecker)

	oauthApiChecker, err := createOauthApiChecker(i.adminRESTConfig, "/apis/oauth.openshift.io/v1/oauthclients", false)
	if err != nil {
		return fmt.Errorf("unable to create oauth api checker: %v", err)
	}

	i.disruptionCheckers = append(i.disruptionCheckers, oauthApiChecker)

	oauthApiCachedChecker, err := createOauthApiChecker(i.adminRESTConfig, fmt.Sprintf("/apis/oauth.openshift.io/v1/oauthclients/%s?resourceVersion=%s", oauthClientName, oauthClientRevision), true)
	if err != nil {
		return fmt.Errorf("unable to create cached openshift api checker: %v", err)
	}
	i.disruptionCheckers = append(i.disruptionCheckers, oauthApiCachedChecker)

	for n := range i.disruptionCheckers {
		if err := i.disruptionCheckers[n].StartCollection(ctx, adminRESTConfig, recorder); err != nil {
			return err
		}
	}

	return nil
}

func (i *InvariantExternalDisruption) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	log := logrus.WithField("monitorTest", "apiserver-external-availability").WithField("func", "CollectData")
	log.Infof("collecting intervals")
	if i.notSupportedReason != nil {
		return nil, nil, i.notSupportedReason
	}
	// we failed and indicated it during setup.
	if i.disruptionCheckers == nil {
		return nil, nil, nil
	}

	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for n := range i.disruptionCheckers {
		// we failed and indicated it during setup.
		if i.disruptionCheckers[n] == nil {
			continue
		}

		localIntervals, localJunits, localErr := i.disruptionCheckers[n].CollectData(ctx)
		intervals = append(intervals, localIntervals...)
		junits = append(junits, localJunits...)
		if localErr != nil {
			errs = append(errs, localErr)
		}
	}

	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (i *InvariantExternalDisruption) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning time.Time, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

func (i *InvariantExternalDisruption) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {

	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for n := range i.disruptionCheckers {
		// we failed and indicated it during setup.
		if i.disruptionCheckers[n] == nil {
			continue
		}

		localJunits, localErr := i.disruptionCheckers[n].EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
		junits = append(junits, localJunits...)
		if localErr != nil {
			errs = append(errs, localErr)
		}
	}

	return junits, utilerrors.NewAggregate(errs)
}

func (i *InvariantExternalDisruption) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (i *InvariantExternalDisruption) Cleanup(ctx context.Context) error {
	return nil
}
