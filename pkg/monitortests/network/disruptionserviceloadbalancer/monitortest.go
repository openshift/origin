package disruptionserviceloadbalancer

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework/service"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var (
	//go:embed namespace.yaml
	namespaceYaml []byte
)

var (
	namespace *corev1.Namespace
)

const (
	newConnectionTestName    = "[sig-network-edge] disruption/service-load-balancer-with-pdb connection/new should be available throughout the test"
	reusedConnectionTestName = "[sig-network-edge] disruption/service-load-balancer-with-pdb connection/reused should be available throughout the test"
)

func init() {
	namespace = resourceread.ReadNamespaceV1OrDie(namespaceYaml)
}

type availability struct {
	namespaceName      string
	notSupportedReason error
	kubeClient         kubernetes.Interface

	disruptionChecker *disruptionlibrary.Availability
	suppressJunit     bool
}

// isNotSupportedForPlatformExternal if platform type is external, this checks to see if the
// platform name is one that does not support a service load balancer.
func isNotSupportedForPlatformExternal(infra *configv1.Infrastructure) bool {
	notSupported := false
	if infra.Spec.PlatformSpec.External == nil {
		return notSupported
	}

	platformName := infra.Spec.PlatformSpec.External.PlatformName

	fmt.Fprintf(os.Stderr, "external platform name: %s\n", platformName)

	switch strings.ToLower(infra.Spec.PlatformSpec.External.PlatformName) {
	case "vsphere":
		notSupported = true
	}

	return notSupported
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
	var err error

	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(w.kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "platform MicroShift not supported"}
		return w.notSupportedReason
	}

	configClient, err := configclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	// ovirt does not support service type loadbalancer because it doesn't program a cloud.
	// none platform does not have CCM which exposes the service of type loadbalancer.
	if infra.Status.PlatformStatus.Type == configv1.OvirtPlatformType ||
		infra.Status.PlatformStatus.Type == configv1.KubevirtPlatformType ||
		infra.Status.PlatformStatus.Type == configv1.LibvirtPlatformType ||
		infra.Status.PlatformStatus.Type == configv1.NutanixPlatformType ||
		infra.Status.PlatformStatus.Type == configv1.VSpherePlatformType ||
		infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType ||
		infra.Status.PlatformStatus.Type == configv1.OpenStackPlatformType ||
		(infra.Status.PlatformStatus.Type == configv1.ExternalPlatformType && isNotSupportedForPlatformExternal(infra)) ||
		infra.Status.PlatformStatus.Type == configv1.NonePlatformType {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: fmt.Sprintf("platform %q is not supported", infra.Status.PlatformStatus.Type),
		}
	}
	// single node clusters are not supported because the replication controller has 2 replicas with anti-affinity for running on the same node.
	if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: fmt.Sprintf("topology %q is not supported", infra.Status.ControlPlaneTopology),
		}
	}
	nodeList, err := w.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(nodeList.Items) < 2 {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: fmt.Sprintf("insufficient nodes for service load balancers"),
		}
	}
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}

	actualNamespace, err := w.kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	w.namespaceName = actualNamespace.Name

	serviceName := "service-test"
	jig := service.NewTestJig(w.kubeClient, w.namespaceName, serviceName)

	fmt.Fprintf(os.Stderr, "creating a TCP service %v with type=LoadBalancer in namespace %v\n", serviceName, w.namespaceName)
	tcpService, err := jig.CreateTCPService(ctx, func(s *corev1.Service) {
		s.Spec.Type = corev1.ServiceTypeLoadBalancer
		// ServiceExternalTrafficPolicyTypeCluster performs during disruption, Local does not
		s.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		// We tune the LB checks to match the longest intervals available so that interactions between
		// upgrading components and the service are more obvious.
		// - AWS allows configuration, default is 70s (6 failed with 10s interval in 1.17) set to match GCP
		s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-interval"] = "8"
		s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-unhealthy-threshold"] = "3"
		s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-healthy-threshold"] = "2"
		// - Azure is hardcoded to 15s (2 failed with 5s interval in 1.17) and is sufficient
		// - GCP has a non-configurable interval of 32s (3 failed health checks with 8s interval in 1.17)
		//   - thus pods need to stay up for > 32s, so pod shutdown period will will be 45s
	})
	if err != nil {
		return fmt.Errorf("error creating tcp service: %w", err)
	}
	tcpService, err = jig.WaitForLoadBalancer(ctx, service.GetServiceLoadBalancerCreationTimeout(ctx, w.kubeClient))
	if err != nil {
		return fmt.Errorf("error waiting for load balancer: %w", err)
	}

	// Get info to hit it with
	tcpIngressIP := service.GetIngressPoint(&tcpService.Status.LoadBalancer.Ingress[0])
	svcPort := int(tcpService.Spec.Ports[0].Port)

	fmt.Fprintf(os.Stderr, "creating RC to be part of service %v\n", serviceName)
	rc, err := jig.Run(ctx, func(rc *corev1.ReplicationController) {
		// ensure the pod waits long enough during update for the LB to see the newly ready pod, which
		// must be longer than the worst load balancer above (GCP at 32s)
		rc.Spec.MinReadySeconds = 33

		// use a readiness endpoint that will go not ready before the pod terminates.
		// the probe will go false when the sig-term is sent.
		rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path = "/readyz"

		// delay shutdown long enough to go readyz=false before the process exits when the pod is deleted.
		// 80 second delay was found to not show disruption in testing
		rc.Spec.Template.Spec.Containers[0].Args = append(rc.Spec.Template.Spec.Containers[0].Args, "--delay-shutdown=80")

		// force the image to use the "normal" global mapping.
		originalAgnhost := k8simage.GetOriginalImageConfigs()[k8simage.Agnhost]
		rc.Spec.Template.Spec.Containers[0].Image = image.LocationFor(originalAgnhost.GetE2EImage())

		// ensure the pod is not forcibly deleted at 30s, but waits longer than the graceful sleep
		minuteAndAHalf := int64(90)
		rc.Spec.Template.Spec.TerminationGracePeriodSeconds = &minuteAndAHalf

		jig.AddRCAntiAffinity(rc)
	})
	if err != nil {
		return fmt.Errorf("error waiting for replicaset: %w", err)
	}

	fmt.Fprintf(os.Stderr, "creating a PodDisruptionBudget to cover the ReplicationController\n")
	_, err = jig.CreatePDB(ctx, rc)
	if err != nil {
		return fmt.Errorf("error creating PDB: %w", err)
	}

	// On certain platforms hitting the hostname before it is ready leads to a blackhole, this code checks
	// the host from the cluster's context
	if infra.Spec.PlatformSpec.Type == configv1.PowerVSPlatformType || infra.Spec.PlatformSpec.Type == configv1.IBMCloudPlatformType {
		nodeTgt := "node/" + nodeList.Items[0].ObjectMeta.Name
		if err := checkHostnameReady(ctx, tcpService, nodeTgt, w.namespaceName); err != nil {
			return err
		}
	}

	// Hit it once before considering ourselves ready
	fmt.Fprintf(os.Stderr, "hitting pods through the service's LoadBalancer\n")
	timeout := 10 * time.Minute
	// require thirty seconds of passing requests to continue (in case the SLB becomes available and then degrades)
	// TODO this seems weird to @deads2k, why is status not trustworthy
	baseURL := fmt.Sprintf("http://%s", net.JoinHostPort(tcpIngressIP, strconv.Itoa(svcPort)))
	path := "/echo?msg=hello"
	url := fmt.Sprintf("%s%s", baseURL, path)
	if err := testReachableHTTPWithMinSuccessCount(url, 30, timeout); err != nil {
		return fmt.Errorf("could not reach %v reliably: %w", url, err)
	}

	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		baseURL,
		"service-load-balancer-with-pdb-new-connections",
		path,
		monitorapi.NewConnectionType).
		WithExpectedBody("hello")
	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		baseURL,
		"service-load-balancer-with-pdb-reused-connections",
		path,
		monitorapi.ReusedConnectionType).
		WithExpectedBody("hello")

	w.disruptionChecker = disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
	)
	if err := w.disruptionChecker.StartCollection(ctx, adminRESTConfig, recorder); err != nil {
		return err
	}
	return nil
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
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

func (w *availability) namespaceDeleted(ctx context.Context) (bool, error) {
	var lastError error
	for retry := 0; retry < 6; retry++ {
		_, err := w.kubeClient.CoreV1().Namespaces().Get(ctx, w.namespaceName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		if err == nil {
			return false, nil
		}
		logrus.Errorf("Error checking for deleted namespace: %s, %s", w.namespaceName, err.Error())
		lastError = err
		time.Sleep(5 * time.Second)
	}
	return false, lastError
}

func (w *availability) Cleanup(ctx context.Context) error {
	if len(w.namespaceName) > 0 && w.kubeClient != nil {
		log := logrus.WithField("monitorTest", "service-type-load-balancer-availability").WithField("namespace", w.namespaceName)
		log.Info("deleting namespace")
		if err := w.kubeClient.CoreV1().Namespaces().Delete(ctx, w.namespaceName, metav1.DeleteOptions{}); err != nil {
			log.WithError(err).Error("error during namespace deletion")
			return err
		}

		startTime := time.Now()
		log.Info("waiting for namespace deletion to complete")
		err := wait.PollUntilContextTimeout(ctx, 30*time.Second, 20*time.Minute, true, w.namespaceDeleted)
		if err != nil {
			log.Errorf("Encountered error while waiting for deleted namespace: %s, %s", w.namespaceName, err)
			return err
		}
		log.Infof("namespace deleted in %.2f seconds", time.Now().Sub(startTime).Seconds())
	}

	return nil
}

// testReachableHTTPWithMinSuccessCount tests that the given host serves HTTP on the given port for a minimum of successCount number of
// counts at a given interval. If the service reachability fails, the counter gets reset
func testReachableHTTPWithMinSuccessCount(url string, successCount int, timeout time.Duration) error {
	consecutiveSuccessCnt := 0
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		resp, err := httpGetNoConnectionPoolTimeout(url, 10*time.Second)
		if err != nil {
			consecutiveSuccessCnt = 0
			return false, nil
		}
		resp.Body.Close()
		consecutiveSuccessCnt++
		return consecutiveSuccessCnt >= successCount, nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Does an HTTP GET, but does not reuse TCP connections
// This masks problems where the iptables rule has changed, but we don't see it
func httpGetNoConnectionPoolTimeout(url string, timeout time.Duration) (*http.Response, error) {
	tr := utilnet.SetTransportDefaults(&http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
	})
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	return client.Get(url)
}

// Uses the first node in the cluster to verify the LoadBalancer host is active before returning
func checkHostnameReady(ctx context.Context, tcpService *corev1.Service, nodeTgt string, namespace string) error {
	oc := exutil.NewCLIForMonitorTest(tcpService.GetObjectMeta().GetNamespace())

	var (
		stdOut string
		err    error
	)

	wait.PollUntilContextTimeout(ctx, 15*time.Second, 60*time.Minute, true, func(ctx context.Context) (bool, error) {
		logrus.Debug("Checking load balancer host is active \n")
		stdOut, _, err = oc.AsAdmin().WithoutNamespace().RunInMonitorTest("debug").Args(nodeTgt, "--to-namespace="+namespace, "--", "dig", "+short", "+notcp", tcpService.Status.LoadBalancer.Ingress[0].Hostname).Outputs()
		if err != nil {
			return false, nil
		}
		output := strings.TrimSpace(stdOut)
		if output == "" {
			logrus.Debug("Waiting for the load balancer to become active")
			return false, nil
		}
		logrus.Debug("Load balancer active")
		return true, nil
	})
	return err
}
