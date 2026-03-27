package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-network-edge][OCPFeatureGate:AWSDualStackInstall][Feature:Router][apigroup:route.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io]", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("router-dualstack", admissionapi.LevelBaseline)

	var baseDomain string

	g.BeforeEach(func() {
		requireAWSDualStack(context.Background(), oc)

		defaultDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")
		baseDomain = strings.TrimPrefix(defaultDomain, "apps.")
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWithInNamespace("router-"+oc.Namespace(), "openshift-ingress", oc.AsAdmin())
		}
	})

	g.It("should be reachable via IPv4 and IPv6 through a dual-stack ingress controller", func() {
		ctx := context.Background()

		ns := oc.KubeFramework().Namespace.Name
		shardFQDN := "nlb." + baseDomain

		// Deploy the shard first so DNS and LB can provision while we set up the backend.
		g.By("Deploying a new router shard with NLB")
		shardIngressCtrl, err := shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
			Domain: shardFQDN,
			Type:   oc.Namespace(),
			LoadBalancer: &operatorv1.LoadBalancerStrategy{
				Scope: operatorv1.ExternalLoadBalancer,
				ProviderParameters: &operatorv1.ProviderLoadBalancerParameters{
					Type: operatorv1.AWSLoadBalancerProvider,
					AWS: &operatorv1.AWSLoadBalancerParameters{
						Type: operatorv1.AWSNetworkLoadBalancer,
					},
				},
			},
		})
		defer func() {
			if shardIngressCtrl != nil {
				if err := oc.AdminOperatorClient().OperatorV1().IngressControllers(shardIngressCtrl.Namespace).Delete(ctx, shardIngressCtrl.Name, metav1.DeleteOptions{}); err != nil {
					e2e.Logf("deleting ingress controller failed: %v\n", err)
				}
			}
		}()
		o.Expect(err).NotTo(o.HaveOccurred(), "new router shard did not rollout")

		g.By("Disabling client IP preservation on the NLB target group to avoid hairpin issues (OCPBUGS-63219)")
		routerSvcName := "router-" + shardIngressCtrl.Name
		err = oc.AsAdmin().Run("annotate").Args("service", "-n", "openshift-ingress", routerSvcName,
			"service.beta.kubernetes.io/aws-load-balancer-target-group-attributes=preserve_client_ip.enabled=false").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Labelling the namespace for the shard")
		err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating backend service and pod")
		createBackendServiceAndPod(ctx, oc, ns, "dualstack-backend")

		g.By("Creating an edge-terminated route")
		routeHost := "dualstack-test." + shardFQDN
		createEdgeRoute(ctx, oc, ns, "dualstack-route", routeHost, "dualstack-backend")

		g.By("Waiting for the route to be admitted")
		waitForRouteAdmitted(ctx, oc, ns, "dualstack-route", routeHost, 5*time.Minute)

		g.By("Creating exec pod for curl tests")
		execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
		defer func() {
			oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, execPod.Name, *metav1.NewDeleteOptions(1))
		}()

		g.By("Waiting for DNS resolution of the route host")
		err = waitForDNSResolution(ctx, ns, execPod.Name, routeHost, 10*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "DNS resolution failed")

		g.By("Verifying route is reachable over IPv4")
		err = waitForRouteResponse(ctx, ns, execPod.Name, routeHost, "-4", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "route not reachable over IPv4")

		g.By("Verifying route is reachable over IPv6")
		err = waitForRouteResponse(ctx, ns, execPod.Name, routeHost, "-6", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "route not reachable over IPv6")
	})

	g.It("should be reachable via IPv4 through a Classic LB ingress controller on a dual-stack cluster", func() {
		ctx := context.Background()

		ns := oc.KubeFramework().Namespace.Name
		shardFQDN := "clb." + baseDomain

		// Deploy the shard first so DNS and LB can provision while we set up the backend.
		g.By("Deploying a new router shard with Classic LB")
		shardIngressCtrl, err := shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
			Domain: shardFQDN,
			Type:   oc.Namespace(),
			LoadBalancer: &operatorv1.LoadBalancerStrategy{
				Scope: operatorv1.ExternalLoadBalancer,
				ProviderParameters: &operatorv1.ProviderLoadBalancerParameters{
					Type: operatorv1.AWSLoadBalancerProvider,
					AWS: &operatorv1.AWSLoadBalancerParameters{
						Type: operatorv1.AWSClassicLoadBalancer,
					},
				},
			},
		})
		defer func() {
			if shardIngressCtrl != nil {
				if err := oc.AdminOperatorClient().OperatorV1().IngressControllers(shardIngressCtrl.Namespace).Delete(ctx, shardIngressCtrl.Name, metav1.DeleteOptions{}); err != nil {
					e2e.Logf("deleting ingress controller failed: %v\n", err)
				}
			}
		}()
		o.Expect(err).NotTo(o.HaveOccurred(), "new router shard did not rollout")

		g.By("Labelling the namespace for the shard")
		err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating backend service and pod")
		createBackendServiceAndPod(ctx, oc, ns, "classic-backend")

		g.By("Creating an edge-terminated route")
		routeHost := "classic-test." + shardFQDN
		createEdgeRoute(ctx, oc, ns, "classic-route", routeHost, "classic-backend")

		g.By("Waiting for the route to be admitted")
		waitForRouteAdmitted(ctx, oc, ns, "classic-route", routeHost, 5*time.Minute)

		g.By("Creating exec pod for curl tests")
		execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
		defer func() {
			oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, execPod.Name, *metav1.NewDeleteOptions(1))
		}()

		g.By("Waiting for DNS resolution of the route host")
		err = waitForDNSResolution(ctx, ns, execPod.Name, routeHost, 10*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "DNS resolution failed")

		g.By("Verifying route is reachable over IPv4")
		err = waitForRouteResponse(ctx, ns, execPod.Name, routeHost, "-4", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "route not reachable over IPv4")
	})
})

func requireAWSDualStack(ctx context.Context, oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get infrastructure CR")

	if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.Type != configv1.AWSPlatformType {
		g.Skip("Test requires AWS platform")
	}
	if infra.Status.PlatformStatus.AWS == nil {
		g.Skip("AWS platform status is not set")
	}
	ipFamily := infra.Status.PlatformStatus.AWS.IPFamily
	if ipFamily != configv1.DualStackIPv4Primary && ipFamily != configv1.DualStackIPv6Primary {
		g.Skip(fmt.Sprintf("Test requires DualStack IPFamily, got %q", ipFamily))
	}
}

func createBackendServiceAndPod(ctx context.Context, oc *exutil.CLI, ns, name string) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": name},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			IPFamilyPolicy: func() *corev1.IPFamilyPolicy {
				p := corev1.IPFamilyPolicyPreferDualStack
				return &p
			}(),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	_, err := oc.AdminKubeClient().CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": name},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: utilpointer.Int64(1),
			Containers: []corev1.Container{
				{
					Name:            "server",
					Image:           image.ShellImage(),
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{"/bin/bash", "-c", `while true; do
printf "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nContent-Type: text/plain\r\n\r\nOK" | ncat -l 8080 --send-only || true
done`},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8080,
							Name:          "http",
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}
	_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.ExpectNoError(e2epod.WaitForPodRunningInNamespaceSlow(ctx, oc.KubeClient(), name, ns), "backend pod not running")
}

func createEdgeRoute(ctx context.Context, oc *exutil.CLI, ns, name, host, serviceName string) {
	route := routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"type": oc.Namespace(),
			},
		},
		Spec: routev1.RouteSpec{
			Host: host,
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(8080),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: utilpointer.Int32(100),
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
	_, err := oc.RouteClient().RouteV1().Routes(ns).Create(ctx, &route, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForRouteAdmitted(ctx context.Context, oc *exutil.CLI, ns, name, host string, timeout time.Duration) {
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		r, err := oc.RouteClient().RouteV1().Routes(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("failed to get route: %v, retrying...", err)
			return false, nil
		}
		for _, ingress := range r.Status.Ingress {
			if ingress.Host == host {
				for _, condition := range ingress.Conditions {
					if condition.Type == routev1.RouteAdmitted && condition.Status == corev1.ConditionTrue {
						return true, nil
					}
				}
			}
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "route was not admitted")
}

func waitForDNSResolution(ctx context.Context, ns, execPodName, host string, timeout time.Duration) error {
	cmd := fmt.Sprintf("getent hosts %s", host)
	var lastOutput string
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
		lastOutput = output
		if err != nil {
			return false, nil
		}
		e2e.Logf("DNS resolution for %s:\n%s", host, strings.TrimSpace(output))
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("DNS resolution for %s timed out, last output: %s", host, lastOutput)
	}
	return nil
}

func waitForRouteResponse(ctx context.Context, ns, execPodName, host, ipFlag string, timeout time.Duration) error {
	curlCmd := fmt.Sprintf("curl %s -k -v -m 10 --connect-timeout 5 -o /dev/null https://%s 2>&1", ipFlag, host)
	var lastOutput string
	consecutiveSuccesses := 0
	requiredSuccesses := 3
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		output, err := e2eoutput.RunHostCmd(ns, execPodName, curlCmd)
		lastOutput = output
		if err != nil {
			consecutiveSuccesses = 0
			return false, nil
		}
		if strings.Contains(output, "< HTTP/1.1 200") || strings.Contains(output, "< HTTP/2 200") {
			consecutiveSuccesses++
			e2e.Logf("curl %s %s: success (%d/%d)", ipFlag, host, consecutiveSuccesses, requiredSuccesses)
			if consecutiveSuccesses >= requiredSuccesses {
				e2e.Logf("curl %s %s:\n%s", ipFlag, host, output)
				return true, nil
			}
			return false, nil
		}
		consecutiveSuccesses = 0
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("curl %s to %s timed out, last output:\n%s", ipFlag, host, lastOutput)
	}
	return nil
}
