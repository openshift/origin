package service

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/onsi/ginkgo/v2"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/test/extended/util/disruption"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	"k8s.io/kubernetes/test/e2e/framework/service"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// serviceLoadBalancerUpgradeTest tests that a service is available before, during, and
// after a cluster upgrade.
type serviceLoadBalancerUpgradeTest struct {
	// filled in by pre-setup
	jig                 *service.TestJig
	tcpService          *v1.Service
	unsupportedPlatform bool
	hostGetter          *backenddisruption.SimpleHostGetter

	backendDisruptionTest disruption.BackendDisruptionUpgradeTest
}

func NewServiceLoadBalancerWithNewConnectionsTest() upgrades.Test {
	serviceLBTest := &serviceLoadBalancerUpgradeTest{
		hostGetter: backenddisruption.NewSimpleHostGetter(""), // late binding host
	}
	serviceLBTest.backendDisruptionTest =
		disruption.NewBackendDisruptionTest(
			"[sig-network-edge] Application behind service load balancer with PDB remains available using new connections",
			backenddisruption.NewBackend(
				serviceLBTest.hostGetter,
				"service-load-balancer-with-pdb",
				"/echo?msg=Hello",
				monitorapi.NewConnectionType).
				WithExpectedBody("Hello"),
		).
			WithPreSetup(serviceLBTest.loadBalancerSetup)

	return serviceLBTest
}

func NewServiceLoadBalancerWithReusedConnectionsTest() upgrades.Test {
	serviceLBTest := &serviceLoadBalancerUpgradeTest{
		hostGetter: backenddisruption.NewSimpleHostGetter(""), // late binding host
	}
	serviceLBTest.backendDisruptionTest =
		disruption.NewBackendDisruptionTest(
			"[sig-network-edge] Application behind service load balancer with PDB remains available using reused connections",
			backenddisruption.NewBackend(
				serviceLBTest.hostGetter,
				"service-load-balancer-with-pdb",
				"/echo?msg=Hello",
				monitorapi.ReusedConnectionType).
				WithExpectedBody("Hello"),
		).
			WithPreSetup(serviceLBTest.loadBalancerSetup)

	return serviceLBTest
}

func (t *serviceLoadBalancerUpgradeTest) Name() string { return t.backendDisruptionTest.Name() }
func (t *serviceLoadBalancerUpgradeTest) DisplayName() string {
	return t.backendDisruptionTest.DisplayName()
}

// RequiresKubeNamespace indicates we get an e2e-k8s- namespace so we can bind low ports.
func (t *serviceLoadBalancerUpgradeTest) RequiresKubeNamespace() bool {
	return true
}

func shouldTestPDBs() bool { return true }

func (t *serviceLoadBalancerUpgradeTest) loadBalancerSetup(f *framework.Framework, backendSampler disruption.BackendSampler) error {
	// we must update our namespace to bypass SCC so that we can avoid default mutation of our pod and SCC evaluation.
	// technically we could also choose to bind an SCC, but I don't see a lot of value in doing that and we have to wait
	// for a secondary cache to fill to reflect that.  If we miss that cache filling, we'll get assigned a restricted on
	// and fail.
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ns, err := f.ClientSet.CoreV1().Namespaces().Get(context.Background(), f.Namespace.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels["security.openshift.io/disable-securitycontextconstraints"] = "true"
		ns, err = f.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	framework.ExpectNoError(err)

	configClient, err := configclient.NewForConfig(f.ClientConfig())
	framework.ExpectNoError(err)
	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)
	// ovirt does not support service type loadbalancer because it doesn't program a cloud.
	if infra.Status.PlatformStatus.Type == configv1.OvirtPlatformType || infra.Status.PlatformStatus.Type == configv1.KubevirtPlatformType || infra.Status.PlatformStatus.Type == configv1.LibvirtPlatformType || infra.Status.PlatformStatus.Type == configv1.VSpherePlatformType || infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType {
		t.unsupportedPlatform = true
	}
	// single node clusters are not supported because the replication controller has 2 replicas with anti-affinity for running on the same node.
	if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		t.unsupportedPlatform = true
	}
	if t.unsupportedPlatform {
		return nil
	}

	serviceName := "service-test"
	jig := service.NewTestJig(f.ClientSet, f.Namespace.Name, serviceName)

	ns := f.Namespace
	cs := f.ClientSet
	ctx := context.Background()

	ginkgo.By("creating a TCP service " + serviceName + " with type=LoadBalancer in namespace " + ns.Name)
	tcpService, err := jig.CreateTCPService(ctx, func(s *v1.Service) {
		s.Spec.Type = v1.ServiceTypeLoadBalancer
		// ServiceExternalTrafficPolicyTypeCluster performs during disruption, Local does not
		s.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster
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
	framework.ExpectNoError(err)
	tcpService, err = jig.WaitForLoadBalancer(ctx, service.GetServiceLoadBalancerCreationTimeout(ctx, cs))
	framework.ExpectNoError(err)

	// Get info to hit it with
	tcpIngressIP := service.GetIngressPoint(&tcpService.Status.LoadBalancer.Ingress[0])
	svcPort := int(tcpService.Spec.Ports[0].Port)

	ginkgo.By("creating RC to be part of service " + serviceName)
	rc, err := jig.Run(ctx, func(rc *v1.ReplicationController) {
		// ensure the pod waits long enough during update for the LB to see the newly ready pod, which
		// must be longer than the worst load balancer above (GCP at 32s)
		rc.Spec.MinReadySeconds = 33

		// use a readiness endpoint that will go not ready before the pod terminates.
		// the probe will go false when the sig-term is sent.
		rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path = "/readyz"

		// delay shutdown long enough to go readyz=false before the process exits when the pod is deleted.
		rc.Spec.Template.Spec.Containers[0].Args = append(rc.Spec.Template.Spec.Containers[0].Args, "--delay-shutdown=45")

		// ensure the pod is not forcibly deleted at 30s, but waits longer than the graceful sleep
		minute := int64(60)
		rc.Spec.Template.Spec.TerminationGracePeriodSeconds = &minute

		jig.AddRCAntiAffinity(rc)
	})
	framework.ExpectNoError(err)

	if shouldTestPDBs() {
		ginkgo.By("creating a PodDisruptionBudget to cover the ReplicationController")
		_, err = jig.CreatePDB(ctx, rc)
		framework.ExpectNoError(err)
	}

	// Hit it once before considering ourselves ready
	ginkgo.By("hitting pods through the service's LoadBalancer")
	timeout := 10 * time.Minute
	// require thirty seconds of passing requests to continue (in case the SLB becomes available and then degrades)
	// TODO this seems weird to @deads2k, why is status not trustworthy
	TestReachableHTTPWithMinSuccessCount(tcpIngressIP, svcPort, 30, timeout)

	t.hostGetter.SetHost(fmt.Sprintf("http://%s", net.JoinHostPort(tcpIngressIP, strconv.Itoa(svcPort))))

	t.jig = jig
	t.tcpService = tcpService
	return nil
}

// Test runs a connectivity check to the service.
func (t *serviceLoadBalancerUpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	if t.unsupportedPlatform {
		return
	}

	t.backendDisruptionTest.Test(ctx, f, done, upgrade)

	// verify finalizer behavior
	defer func() {
		ginkgo.By("Check that service can be deleted with finalizer")
		service.WaitForServiceDeletedWithFinalizer(ctx, t.jig.Client, t.tcpService.Namespace, t.tcpService.Name)
	}()
	ginkgo.By("Check that finalizer is present on loadBalancer type service")
	service.WaitForServiceUpdatedWithFinalizer(ctx, t.jig.Client, t.tcpService.Namespace, t.tcpService.Name, true)
}

func (t *serviceLoadBalancerUpgradeTest) Teardown(ctx context.Context, f *framework.Framework) {
	t.backendDisruptionTest.Teardown(ctx, f)
}

func (t *serviceLoadBalancerUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	t.backendDisruptionTest.Setup(ctx, f)
}

// TestReachableHTTPWithMinSuccessCount tests that the given host serves HTTP on the given port for a minimum of successCount number of
// counts at a given interval. If the service reachability fails, the counter gets reset
func TestReachableHTTPWithMinSuccessCount(host string, port int, successCount int, timeout time.Duration) {
	consecutiveSuccessCnt := 0
	err := wait.PollImmediate(framework.Poll, timeout, func() (bool, error) {
		result := e2enetwork.PokeHTTP(host, port, "/echo?msg=hello",
			&e2enetwork.HTTPPokeParams{
				BodyContains:   "hello",
				RetriableCodes: []int{},
			})
		if result.Status == e2enetwork.HTTPSuccess {
			consecutiveSuccessCnt++
			return consecutiveSuccessCnt >= successCount, nil
		}
		consecutiveSuccessCnt = 0
		return false, nil // caller can retry
	})
	framework.ExpectNoError(err)
}
