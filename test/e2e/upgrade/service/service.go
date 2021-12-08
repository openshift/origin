package service

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/onsi/ginkgo"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	"k8s.io/kubernetes/test/e2e/framework/service"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// serviceLoadBalancerSetupTeardown tests that a service is available before, during, and
// after a cluster upgrade.
type serviceLoadBalancerSetupTeardown struct {
	// filled in by presetup
	jig                 *service.TestJig
	tcpService          *v1.Service
	unsupportedPlatform bool

	setup         sync.Once
	teardown      sync.Once
	testsInFlight sync.WaitGroup
}

// returns new and used disruption tests.  They have to share a single service and set of pods and teardown because it's
// doing something that is exposed on a node, so these are shared so that we can get this finished.
func NewServiceLoadBalancerDisruptionTests() (upgrades.Test, upgrades.Test) {
	serviceLBSetup := &serviceLoadBalancerSetupTeardown{
		setup:         sync.Once{},
		testsInFlight: sync.WaitGroup{},
	}

	newConnections := disruption.NewBackendDisruptionTest(
		"[sig-network-edge] Application behind service load balancer with PDB remains available using new connections",
		monitor.NewBackend(
			"service-loadbalancer-with-pdb",
			"/echo?msg=Hello",
			monitor.NewConnectionType).
			WithExpectedBody("Hello"),
	).WithAllowedDisruption(allowedServiceLBDisruption).
		WithPreSetup(serviceLBSetup.loadBalancerSetup).
		WithPostTeardown(serviceLBSetup.loadBalancerTeardown)

	usedConnections := disruption.NewBackendDisruptionTest(
		"[sig-network-edge] Application behind service load balancer with PDB remains available using reused connections",
		monitor.NewBackend(
			"service-loadbalancer-with-pdb",
			"/echo?msg=Hello",
			monitor.ReusedConnectionType).
			WithExpectedBody("Hello"),
	).WithAllowedDisruption(allowedServiceLBDisruption).
		WithPreSetup(serviceLBSetup.loadBalancerSetup).
		WithPostTeardown(serviceLBSetup.loadBalancerTeardown)

	return &serviceLoadBalancerUpdateTest{
			BackendDisruptionUpgradeTest: newConnections,
			setupTeardown:                serviceLBSetup,
		}, &serviceLoadBalancerUpdateTest{
			BackendDisruptionUpgradeTest: usedConnections,
			setupTeardown:                serviceLBSetup,
		}
}

func allowedServiceLBDisruption(f *framework.Framework, totalDuration time.Duration) (*time.Duration, error) {
	toleratedDisruption := 0.02
	allowedDisruptionNanoseconds := int64(float64(totalDuration.Nanoseconds()) * toleratedDisruption)
	allowedDisruption := time.Duration(allowedDisruptionNanoseconds)

	return &allowedDisruption, nil
}

func shouldTestPDBs() bool { return true }

func (t *serviceLoadBalancerSetupTeardown) loadBalancerSetup(f *framework.Framework, backendSampler disruption.BackendSampler) error {
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

	t.setup.Do(
		func() {
			serviceName := "service-test"
			jig := service.NewTestJig(f.ClientSet, f.Namespace.Name, serviceName)

			ns := f.Namespace
			cs := f.ClientSet

			ginkgo.By("creating a TCP service " + serviceName + " with type=LoadBalancer in namespace " + ns.Name)
			tcpService, err := jig.CreateTCPService(func(s *v1.Service) {
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
			tcpService, err = jig.WaitForLoadBalancer(service.GetServiceLoadBalancerCreationTimeout(cs))
			framework.ExpectNoError(err)

			ginkgo.By("creating RC to be part of service " + serviceName)
			rc, err := jig.Run(func(rc *v1.ReplicationController) {
				// ensure the pod waits long enough during update for the LB to see the newly ready pod, which
				// must be longer than the worst load balancer above (GCP at 32s)
				rc.Spec.MinReadySeconds = 33
				// ensure the pod waits long enough for most LBs to take it out of rotation, which has to be
				// longer than the LB failed health check duration + 1 cycle
				rc.Spec.Template.Spec.Containers[0].Lifecycle = &v1.Lifecycle{
					PreStop: &v1.Handler{
						Exec: &v1.ExecAction{Command: []string{"sleep", "45"}},
					},
				}
				// ensure the pod is not forcibly deleted at 30s, but waits longer than the graceful sleep
				minute := int64(60)
				rc.Spec.Template.Spec.TerminationGracePeriodSeconds = &minute

				jig.AddRCAntiAffinity(rc)
			})
			framework.ExpectNoError(err)

			if shouldTestPDBs() {
				ginkgo.By("creating a PodDisruptionBudget to cover the ReplicationController")
				_, err = jig.CreatePDB(rc)
				framework.ExpectNoError(err)
			}

			t.jig = jig
			t.tcpService = tcpService
		})

	tcpIngressIP := service.GetIngressPoint(&t.tcpService.Status.LoadBalancer.Ingress[0])
	svcPort := int(t.tcpService.Spec.Ports[0].Port)

	// Hit it once before considering ourselves ready
	ginkgo.By("hitting pods through the service's LoadBalancer")
	timeout := 10 * time.Minute
	// require thirty seconds of passing requests to continue (in case the SLB becomes available and then degrades)
	// TODO this seems weird to @deads2k, why is status not trustworthy
	TestReachableHTTPWithMinSuccessCount(tcpIngressIP, svcPort, 30, timeout)

	backendSampler.SetHost(fmt.Sprintf("http://%s", net.JoinHostPort(tcpIngressIP, strconv.Itoa(svcPort))))

	return nil
}

func (t *serviceLoadBalancerSetupTeardown) loadBalancerTeardown(f *framework.Framework) error {
	t.testsInFlight.Wait()

	// verify finalizer behavior
	// we can only do this once and only after all the tests are done
	t.teardown.Do(func() {
		defer func() {
			ginkgo.By("Check that service can be deleted with finalizer")
			service.WaitForServiceDeletedWithFinalizer(t.jig.Client, t.tcpService.Namespace, t.tcpService.Name)
		}()
		ginkgo.By("Check that finalizer is present on loadBalancer type service")
		service.WaitForServiceUpdatedWithFinalizer(t.jig.Client, t.tcpService.Namespace, t.tcpService.Name, true)
	})

	return nil
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

type serviceLoadBalancerUpdateTest struct {
	disruption.BackendDisruptionUpgradeTest

	setupTeardown *serviceLoadBalancerSetupTeardown
}

// Test runs a connectivity check to the service.
func (t *serviceLoadBalancerUpdateTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	if t.setupTeardown.unsupportedPlatform {
		return
	}
	t.setupTeardown.testsInFlight.Add(1)
	defer t.setupTeardown.testsInFlight.Done()

	t.BackendDisruptionUpgradeTest.Test(f, done, upgrade)
}
