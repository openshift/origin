package cloud_controller_manager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epodoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	azureInternalLoadBalancerAnnotation = "service.beta.kubernetes.io/azure-load-balancer-internal"
	azureLoadBalancerServicePort        = 18080
	azureLoadBalancerBackendPort        = 8080
)

var _ = g.Describe("[sig-cloud-provider][Feature:OpenShiftCloudControllerManager][Feature:IPv6DualStack][OCPFeatureGate:AzureDualStackInstall][Late] Azure dual-stack load balancers", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("ccm-azure-dualstack", admissionapi.LevelPrivileged)

	g.It("should expose public and internal services over IPv4 and IPv6 [Timeout:45m][apigroup:config.openshift.io]", func(ctx context.Context) {
		exutil.SkipIfNotPlatform(oc, configv1.AzurePlatformType)

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(infra.Status.PlatformStatus).NotTo(o.BeNil(), "infrastructure platformStatus is required")
		o.Expect(infra.Status.PlatformStatus.Azure).NotTo(o.BeNil(), "Azure platformStatus is required")
		if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("hosted control-plane clusters do not expose a guest control-plane node for safe load-balancer probes")
		}

		ipFamily := infra.Status.PlatformStatus.Azure.IPFamily
		if ipFamily != configv1.DualStackIPv4Primary && ipFamily != configv1.DualStackIPv6Primary {
			g.Skip(fmt.Sprintf("Azure cluster is not dual-stack: ipFamily=%q", ipFamily))
		}
		e2e.Logf("Testing Azure cluster with ipFamily=%s", ipFamily)

		client := oc.AdminKubeClient()
		namespace := oc.Namespace()

		g.By("creating a dual-stack service backend")
		backend := e2eservice.NewTestJig(client, namespace, "azure-dualstack-backend")
		_, err = backend.Run(ctx, func(deployment *appsv1.Deployment) {
			deployment.Spec.Template.Spec.SecurityContext = e2epod.GetRestrictedPodSecurityContext()
			for i := range deployment.Spec.Template.Spec.Containers {
				container := &deployment.Spec.Template.Spec.Containers[i]
				container.Args = []string{
					"netexec",
					fmt.Sprintf("--http-port=%d", azureLoadBalancerBackendPort),
					fmt.Sprintf("--udp-port=%d", azureLoadBalancerBackendPort),
				}
				container.SecurityContext = e2epod.GetRestrictedContainerSecurityContext()
				o.Expect(container.ReadinessProbe).NotTo(o.BeNil(), "jig deployment is expected to define a readiness probe")
				o.Expect(container.ReadinessProbe.HTTPGet).NotTo(o.BeNil(), "jig readiness probe is expected to use HTTP GET")
				container.ReadinessProbe.HTTPGet.Port = intstr.FromInt32(azureLoadBalancerBackendPort)
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create the load-balancer backend")
		g.DeferCleanup(func(ctx context.Context) {
			err := client.AppsV1().Deployments(namespace).Delete(ctx, backend.Name, metav1.DeleteOptions{})
			o.Expect(apierrors.IsNotFound(err) || err == nil).To(o.BeTrue(), "failed to delete backend deployment: %v", err)
		})

		testCases := []struct {
			name     string
			internal bool
		}{
			{name: "azure-dualstack-public"},
			{name: "azure-dualstack-internal", internal: true},
		}

		services := make([]*corev1.Service, 0, len(testCases))
		serviceNames := make([]string, 0, len(testCases))
		g.DeferCleanup(func(ctx context.Context) {
			err := deleteAzureLoadBalancerServicesAndWait(ctx, client, namespace, serviceNames...)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		for _, testCase := range testCases {
			g.By(fmt.Sprintf("creating the %s load-balancer service", testCase.name))
			service := newAzureDualStackLoadBalancerService(namespace, testCase.name, backend.Labels, testCase.internal)
			service, err = client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to create service %s", testCase.name)
			services = append(services, service)
			serviceNames = append(serviceNames, service.Name)
		}

		g.By("creating a host-networked client on a node excluded from service load balancers")
		clientNode, err := azureLoadBalancerClientNode(ctx, client)
		o.Expect(err).NotTo(o.HaveOccurred())

		probe := exutil.CreateExecPodOrFail(client, namespace, "azure-dualstack-probe", func(pod *corev1.Pod) {
			pod.Spec.NodeName = clientNode
			// Azure Load Balancer does not support hairpin traffic from a backend node.
			// Probe from the host network of an excluded control-plane node instead.
			pod.Spec.HostNetwork = true
			pod.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			pod.Spec.Tolerations = []corev1.Toleration{{Operator: corev1.TolerationOpExists}}
		})
		g.DeferCleanup(func(ctx context.Context) {
			err := client.CoreV1().Pods(namespace).Delete(ctx, probe.Name, metav1.DeleteOptions{})
			o.Expect(apierrors.IsNotFound(err) || err == nil).To(o.BeTrue(), "failed to delete probe pod: %v", err)
		})

		loadBalancerContext, cancelLoadBalancerWait := context.WithTimeout(ctx, e2eservice.GetServiceLoadBalancerCreationTimeout(ctx, client))
		defer cancelLoadBalancerWait()
		for i, service := range services {
			g.By(fmt.Sprintf("waiting for %s to publish IPv4 and IPv6 ingress addresses", service.Name))
			service, err = waitForAzureDualStackLoadBalancer(loadBalancerContext, client, namespace, service.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			services[i] = service
		}

		for _, service := range services {
			o.Expect(service.Spec.IPFamilyPolicy).NotTo(o.BeNil())
			o.Expect(*service.Spec.IPFamilyPolicy).To(o.Equal(corev1.IPFamilyPolicyRequireDualStack))
			o.Expect(service.Spec.IPFamilies).To(o.ConsistOf(corev1.IPv4Protocol, corev1.IPv6Protocol))

			ipv4, ipv6 := loadBalancerIngressIPs(service)
			o.Expect(ipv4).To(o.HaveLen(1), "service %s should have exactly one IPv4 ingress", service.Name)
			o.Expect(ipv6).To(o.HaveLen(1), "service %s should have exactly one IPv6 ingress", service.Name)
			o.Expect(service.Status.LoadBalancer.Ingress).To(o.HaveLen(2), "service %s should have exactly two ingress entries", service.Name)

			for _, addressFamily := range []struct {
				name      string
				addresses []string
			}{
				{name: "IPv4", addresses: ipv4},
				{name: "IPv6", addresses: ipv6},
			} {
				for _, ingressIP := range addressFamily.addresses {
					g.By(fmt.Sprintf("checking %s connectivity to %s", addressFamily.name, service.Name))
					err := probeAzureLoadBalancer(ctx, namespace, probe.Name, ingressIP, service.Spec.Ports[0].Port)
					o.Expect(err).NotTo(o.HaveOccurred(), "service %s is not reachable over %s", service.Name, addressFamily.name)
				}
			}
		}
	})
})

func newAzureDualStackLoadBalancerService(namespace, name string, selector map[string]string, internal bool) *corev1.Service {
	ipFamilyPolicy := corev1.IPFamilyPolicyRequireDualStack
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.ServiceSpec{
			Type:           corev1.ServiceTypeLoadBalancer,
			Selector:       selector,
			IPFamilyPolicy: &ipFamilyPolicy,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       azureLoadBalancerServicePort,
				TargetPort: intstr.FromInt32(azureLoadBalancerBackendPort),
			}},
		},
	}

	if internal {
		service.Annotations = map[string]string{azureInternalLoadBalancerAnnotation: "true"}
	}

	return service
}

func waitForAzureDualStackLoadBalancer(ctx context.Context, client kubernetes.Interface, namespace, name string) (*corev1.Service, error) {
	var service *corev1.Service
	timeout := e2eservice.GetServiceLoadBalancerCreationTimeout(ctx, client)

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		current, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if isRetryableKubernetesAPIError(err) {
				e2e.Logf("Retrying transient error while reading service %s/%s: %v", namespace, name, err)
				return false, nil
			}
			return false, err
		}

		service = current
		ipv4, ipv6 := loadBalancerIngressIPs(current)
		e2e.Logf("Service %s/%s ingress counts: IPv4=%d IPv6=%d", namespace, name, len(ipv4), len(ipv6))
		return len(ipv4) == 1 && len(ipv6) == 1 && len(current.Status.LoadBalancer.Ingress) == 2, nil
	})
	if err != nil {
		return nil, fmt.Errorf("timed out waiting for service %s/%s to publish one IPv4 and one IPv6 ingress: %w", namespace, name, err)
	}

	return service, nil
}

func loadBalancerIngressIPs(service *corev1.Service) (ipv4, ipv6 []string) {
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		ip := net.ParseIP(ingress.IP)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			ipv4 = append(ipv4, ingress.IP)
			continue
		}
		ipv6 = append(ipv6, ingress.IP)
	}
	return ipv4, ipv6
}

func azureLoadBalancerClientNode(ctx context.Context, client kubernetes.Interface) (string, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, node := range nodes.Items {
		_, isMaster := node.Labels["node-role.kubernetes.io/master"]
		_, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]
		_, excludedFromLoadBalancers := node.Labels[corev1.LabelNodeExcludeBalancers]
		if (isMaster || isControlPlane) && excludedFromLoadBalancers && nodeIsReady(&node) {
			return node.Name, nil
		}
	}

	return "", fmt.Errorf("no ready control-plane node excluded from external load balancers was found")
}

func nodeIsReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func probeAzureLoadBalancer(ctx context.Context, namespace, podName, ingressIP string, port int32) error {
	target := net.JoinHostPort(ingressIP, strconv.Itoa(int(port)))
	url := fmt.Sprintf("http://%s/hostname", target)
	command := fmt.Sprintf("curl -g --fail --silent --show-error --noproxy '*' --connect-timeout 5 --max-time 10 --output /dev/null %q", url)

	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		_, lastErr = e2epodoutput.RunHostCmd(namespace, podName, command)
		if lastErr != nil {
			e2e.Logf("Waiting to retry failed load-balancer probe")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("load-balancer probe did not succeed: %w", errors.Join(err, lastErr))
	}
	return nil
}

func deleteAzureLoadBalancerServicesAndWait(ctx context.Context, client kubernetes.Interface, namespace string, names ...string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Minute)
	defer cancel()

	pendingDeletion := make(map[string]struct{}, len(names))
	for _, name := range names {
		pendingDeletion[name] = struct{}{}
	}

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		for name := range pendingDeletion {
			err := client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
			if err == nil || apierrors.IsNotFound(err) {
				delete(pendingDeletion, name)
				continue
			}
			if isRetryableKubernetesAPIError(err) {
				e2e.Logf("Retrying transient error while deleting service %s/%s: %v", namespace, name, err)
				continue
			}
			return false, fmt.Errorf("failed to delete service %s/%s: %w", namespace, name, err)
		}
		return len(pendingDeletion) == 0, nil
	})
	if err != nil {
		return fmt.Errorf("failed to request deletion of Azure load-balancer services: %w", err)
	}

	pendingRemoval := make(map[string]struct{}, len(names))
	for _, name := range names {
		pendingRemoval[name] = struct{}{}
	}

	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		for name := range pendingRemoval {
			_, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				delete(pendingRemoval, name)
				continue
			}
			if err != nil {
				if isRetryableKubernetesAPIError(err) {
					e2e.Logf("Retrying transient error while waiting for service %s/%s deletion: %v", namespace, name, err)
					continue
				}
				return false, err
			}
		}
		return len(pendingRemoval) == 0, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for Azure load-balancer services to be deleted: %w", err)
	}

	return nil
}

func isRetryableKubernetesAPIError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) ||
		utilnet.IsConnectionRefused(err) ||
		utilnet.IsConnectionReset(err) ||
		utilnet.IsTimeout(err) ||
		utilnet.IsHTTP2ConnectionLost(err) ||
		utilnet.IsProbableEOF(err)
}
