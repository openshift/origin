package router

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	"k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-network-edge][Feature:Router][apigroup:route.openshift.io][OCPFeatureGate:IngressControllerDynamicConfigurationManager]", func() {
	defer g.GinkgoRecover()

	const dcmIngressTimeout = 2 * time.Minute
	const maxDynamicServers = 4

	ctx := context.Background()
	oc := exutil.NewCLIWithPodSecurityLevel("router-dcm-ingress", api.LevelPrivileged)

	// variables updated on every new test
	var (
		routerPod     *corev1.Pod
		controller    types.NamespacedName
		routeSelector labels.Selector
	)

	g.AfterEach(func() {
		if controller.Name != "" {
			err := oc.AsAdmin().AdminOperatorClient().OperatorV1().IngressControllers(controller.Namespace).Delete(ctx, controller.Name, *metav1.NewDeleteOptions(1))
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.BeforeEach(func() {
		// ingress controller need to be created in operator's namespace, ...
		nsOperator := "openshift-ingress-operator"
		controllerName := names.SimpleNameGenerator.GenerateName("e2e-dcm-")

		// ... and its router is created on router's namespace
		nsRouter := "openshift-ingress"
		svcName := "router-internal-" + controllerName

		routeSelectorSet := labels.Set{"select": names.SimpleNameGenerator.GenerateName("haproxy-cfgmgr-")}
		routeSelector = labels.SelectorFromSet(routeSelectorSet)

		ic := operatorv1.IngressController{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nsOperator,
				Name:      controllerName,
			},
			Spec: operatorv1.IngressControllerSpec{
				Replicas: ptr.To[int32](1),
				Domain:   controllerName + ".router.local",
				EndpointPublishingStrategy: &operatorv1.EndpointPublishingStrategy{
					Type:    operatorv1.PrivateStrategyType,
					Private: &operatorv1.PrivateStrategy{},
				},
				// TODO `NamespaceSelector` below makes our routes not to be found, need to debug; `RouteSelector` seems to be enough.
				// NamespaceSelector: v1.SetAsLabelSelector(labels.Set{"name": workingNamespace}),
				RouteSelector: metav1.SetAsLabelSelector(routeSelectorSet),
				UnsupportedConfigOverrides: runtime.RawExtension{
					// TODO move the `dynamicConfigManager` param to the ConfigurationManagement API field as soon as both PRs are merged:
					// * https://github.com/openshift/api/pull/2757
					// * https://github.com/openshift/cluster-ingress-operator/pull/1385
					Raw: fmt.Appendf(nil, `{"dynamicConfigManager":"true","maxDynamicServers":"%d"}`, maxDynamicServers),
				},
			},
		}
		_, err := oc.AsAdmin().AdminOperatorClient().OperatorV1().IngressControllers(nsOperator).Create(ctx, &ic, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		controller = types.NamespacedName{
			Namespace: nsOperator,
			Name:      controllerName,
		}

		// wait for the service to be available
		o.Eventually(func(g o.Gomega) {
			svc, err := oc.AdminKubeClient().CoreV1().Services(nsRouter).Get(ctx, svcName, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			listOpts := metav1.ListOptions{LabelSelector: labels.FormatLabels(svc.Spec.Selector)}
			pods, err := oc.AdminKubeClient().CoreV1().Pods(nsRouter).List(ctx, listOpts)
			g.Expect(err).NotTo(o.HaveOccurred())
			g.Expect(pods.Items).To(o.HaveLen(1))

			routerPod = &pods.Items[0]
		}).WithTimeout(dcmIngressTimeout).WithPolling(time.Second).Should(o.Succeed())

		// wait for router to respond requests
		_, err = waitLocalURL(ctx, routerPod, "localhost", "/", http.StatusServiceUnavailable, dcmIngressTimeout) // 503 expected when host/path does not have a route
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router with Dynamic Config Manager", func() {

		// Ensure that basic functionality works when `configurationManagement: Dynamic` is specified. For example, create
		// an application with 1 pod replica, create a route, and verify that you can connect to the route.
		g.It("should work on basic functionalities", func() {
			builder := newRouteStackBuilder(oc, "insecure-basic", routeSelector)
			hostname := "route-basic.local"

			g.By("creating deployment, service and route")

			// TODO image need to be fetched under a running test, because of that `imgAgnHost` is here.
			// init a struct instead, just like execPod?
			servers, err := builder.createRouteStack(ctx, routeTypeInsecure, hostname, 1, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(servers).To(o.HaveLen(1))

			g.By("waiting router to deploy the route")

			output, err := waitLocalURL(ctx, routerPod, hostname, "/", http.StatusOK, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.Equal(servers[0]))
		})

		// Ensure that DCM handles scale-out: Scale the application out to 2 pod replicas, and verify that the route now
		// has all 2 endpoints. Next, scale the application out to 2 pod replicas, and verify that the route now has all
		// 3 endponts. Currently 1 to 2 replicas causes a reload, but 1) the test does not know this; 2) dynamic update
		// should happen when moving to "add/del server" api.
		g.It("should handle scale-out operations", func() {
			builder := newRouteStackBuilder(oc, "insecure-scale-out", routeSelector)
			hostname := "route-scale-out.local"
			initReplicas := 1

			g.By("creating deployment, service and route")

			servers, err := builder.createRouteStack(ctx, routeTypeInsecure, hostname, initReplicas, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(servers).To(o.HaveLen(initReplicas))

			g.By("waiting router to deploy the route")

			output, err := waitLocalURL(ctx, routerPod, hostname, "/", http.StatusOK, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.Equal(servers[0]))

			// scaling-out to 4 replicas, one at a time
			for replicas := initReplicas + 1; replicas <= 4; replicas++ {

				g.By(fmt.Sprintf("scaling-out to %d replicas", replicas))

				currentServers, err := builder.scaleDeployment(ctx, replicas, dcmIngressTimeout)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting router to add all the backend servers to the load balance")

				// router should eventually reach all the known replicas
				eventuallyRouteAllServers(routerPod, hostname, currentServers, 0, dcmIngressTimeout)
			}
		})

		// Ensure that DCM handles scale-in. This should be made in a way that the endpoint remains available, so if DCM
		// did fail to update HAProxy, you would continue to see responses from it. This can be achieved e.g. using a
		// service without a selector, creating an endpointslice manually and removing manually the pods from this
		// endpointslice.
		g.It("should handle scale-in operations", func() {
			builder := newRouteStackBuilder(oc, "insecure-scale-in", routeSelector)
			hostname := "route-scale-in.local"
			initReplicas := 4

			g.By("creating deployment, service and route")

			// create the reference Service, attached to the deployment
			servers, err := builder.createDeploymentStack(ctx, routeTypeInsecure, initReplicas, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(servers).To(o.HaveLen(initReplicas))

			// create a detached Service that can be scaled-in without remove running pods
			serviceName, err := builder.createDetachedService(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())

			// route follows our detached service instead of the common one
			err = builder.createRoute(routeTypeInsecure, serviceName, hostname, "/")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting router to add all the backend servers to the load balance")

			eventuallyRouteAllServers(routerPod, hostname, servers, 0, dcmIngressTimeout)

			// scaling-in to 1 replica, one at a time.
			// using the detached service, so we scale the EndpointSlice instead of the deployment.
			// this way the target pod continues available, making us confident that the router removed the backend server from the pool,
			// instead of HAProxy removing it from the balance due to health-check starting to fail.
			for replicas := initReplicas - 1; replicas >= 1; replicas-- {

				g.By(fmt.Sprintf("scaling-in to %d replicas", replicas))

				currentServers, err := builder.scaleInEndpoints(ctx, serviceName, replicas)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("ensure that router removed the missing backend servers from the load balance")

				// router should not reach removed replicas from the EndpointSlice.
				// the test below runs another `5 * replicas` more times after succeeding
				// to ensure that only the expected backend servers are listed.
				eventuallyRouteAllServers(routerPod, hostname, currentServers, 5*replicas, dcmIngressTimeout)
			}
		})

		// Ensure that DCM handles various route updates, such as updating spec.path, spec.tls.termination, or annotations
		// like haproxy.router.openshift.io/rewrite-target. Right now, this is done by falling back to fork-and-reload,
		// but 1) the test doesn't know this, and 2) some changes should start to become dynamic in the future and should
		// behave in the same way from the user perspective.
		g.It("should handle various route updates", func() {
			g.Skip("not yet implemented")
		})

		// Ensure that the router maintains proper balancing for scale-out. This can be achieved by selecting a lb
		// algorithm having a predictable behavior, like RoundRobin. It should distribute requests as expected, despite
		// of scale-in/out operations happening at the same time. This is one of the issues mentioned in
		// https://github.com/openshift/enhancements/blob/master/enhancements/ingress/dynamic-config-manager.md#user-stories
		// that DCM should improve.
		g.It("should maintain proper balancing after scale-out and scale-in operations", func() {
			g.Skip("not yet implemented")
		})

		// Ensure that the router reports accurate metrics after a scale-in or scale-out event. This can use a long-lived
		// connection that is transferring data when the scale-in/out event happens and verify that data transferred after
		// the event continue to be reported in the bytes-in metric. This is described in more detail in the EP -
		// https://github.com/openshift/enhancements/blob/master/enhancements/ingress/dynamic-config-manager.md
		g.It("should report accurate metrics after scale-out and scale-in operations", func() {
			g.Skip("not yet implemented")
		})

		// Ensure that the router pod maintains ~steady memory usage and PID usage after scaling-out/in. The idea here is
		// that fork-and-reload would cause a significant memory and PID usage spike due to the forked process, while the
		// old ones continue running due to long lived connections. This can be done either 1) checking the consequence:
		// memory usage remains steady after scale-in/out operations, while maintaining persistent connections during one
		// scale operation and the next; or 2) checking the cause: HAProxy still reports the same PID after all the scale
		// operations.
		g.It("should maintain steady memory and PID usage after scale-out and scale-in operations", func() {
			builder := newRouteStackBuilder(oc, "insecure-steady-mem-pid", routeSelector)
			hostname := "route-steady-mem-pid.local"
			initReplicas := 3

			// Note: currently, scaling-in to less than `initReplicas` will leave some (maybe all) statically configured
			// servers in Maintenance state. After that, scaling-out to more than `maxDynamicServers` should lead to a
			// reload because router can only enable server slots from the `maxDynamicServers` pool.
			//
			// Related: https://redhat.atlassian.net/browse/OCPBUGS-80932
			//
			// TL;DR: once scaling-in to `maxDynamicServers` or less, a scale-out to more than
			// `maxDynamicServers` can lead to a reload.
			changingReplicas := []int{6, 5, 1, 2, 3, 4}
			maxReplicas := slices.Max(changingReplicas)
			o.Expect(maxReplicas).To(o.BeNumerically("<=", initReplicas+maxDynamicServers),
				"max of changingReplicas should not be more than %d (initReplicas) + %d (maxDynamicServers), but it is %d", initReplicas, maxDynamicServers, maxReplicas)

			g.By("creating deployment, service and route")

			// create the reference Service, attached to the deployment
			servers, err := builder.createRouteStack(ctx, routeTypeInsecure, hostname, initReplicas, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(servers).To(o.HaveLen(initReplicas))

			server, err := waitLocalURL(ctx, routerPod, hostname, "/", http.StatusOK, dcmIngressTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(server).To(o.BeElementOf(servers))

			g.By("waiting router to add all the backend servers to the load balance")

			eventuallyRouteAllServers(routerPod, hostname, servers, 0, dcmIngressTimeout)

			// checking HAProxy PID is a precise way to ensure the proxy wasn't reloaded, which is the
			// source of problems like PID and Memory exhaustion.
			checkPid := func() int {
				cmd := "echo show info | socat - /var/lib/haproxy/run/haproxy.sock | sed -n 's/Pid: //p'"
				pidStr, err := e2eoutput.RunHostCmd(routerPod.Namespace, routerPod.Name, cmd)
				o.Expect(err).NotTo(o.HaveOccurred())
				pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
				o.Expect(err).NotTo(o.HaveOccurred())
				return pid
			}

			pidBefore := checkPid()
			prevReplicas := initReplicas

			// Iterates over a number of scaling operations, always checking if the change is being applied.
			for i, replicas := range changingReplicas {

				g.By(fmt.Sprintf("iteration %d, scaling from %d to %d replicas", i+1, prevReplicas, replicas))

				currentServers, err := builder.scaleDeployment(ctx, replicas, dcmIngressTimeout)
				o.Expect(err).NotTo(o.HaveOccurred())
				eventuallyRouteAllServers(routerPod, hostname, currentServers, 5*replicas, dcmIngressTimeout)

				pidAfter := checkPid()
				o.Expect(pidBefore).To(o.Equal(pidAfter), "pid changed when scaling from %d to %d replicas", prevReplicas, replicas)

				prevReplicas = replicas
			}
		})
	})
})

// eventuallyRouteAllServers runs Eventually assertion against the provided hostname, and should find only
// the provided servers as backends. It expects some asynchronous scale-in and scale-out operations happening
// in parallel.
func eventuallyRouteAllServers(routerPod *corev1.Pod, hostname string, servers []string, repeat int, timeout time.Duration) {
	expectedServers := sets.NewString(servers...)
	actualServers := sets.NewString()
	assertion := o.Eventually(func(g o.Gomega) {
		code, output, err := readLocalURL(routerPod, hostname, "/")
		g.Expect(err).NotTo(o.HaveOccurred())
		g.Expect(code).To(o.Equal(http.StatusOK))
		g.Expect(output).To(o.BeElementOf(servers))
		actualServers.Insert(output)
		g.Expect(expectedServers.List()).To(o.Equal(actualServers.List()))
	}).WithTimeout(timeout).WithPolling(500 * time.Millisecond)
	if repeat > 0 {
		assertion.MustPassRepeatedly(repeat)
	}
	assertion.Should(o.Succeed())
}

// readLocalURL executes a `curl` in the router pod, retuning the response code and response content.
func readLocalURL(routerPod *corev1.Pod, host, abspath string) (code int, output string, err error) {
	host = exutil.IPUrl(host)
	proto := "http"
	port := 80
	uri := fmt.Sprintf("%s://%s:%d%s", proto, host, port, abspath)
	cmd := fmt.Sprintf("curl -ksSL -m 5 -w '\n%%{http_code}' --resolve %s:%d:%s %q", host, port, "127.0.0.1", uri)
	output, err = e2eoutput.RunHostCmd(routerPod.Namespace, routerPod.Name, cmd)
	if err != nil {
		return 0, "", err
	}
	output = strings.TrimSpace(output)

	// extract response code in the last line
	idx := strings.LastIndex(output, "\n")
	if idx < 0 {
		return 0, "", fmt.Errorf("output does not have a response code: %s", output)
	}
	codeStr := output[idx+1:]
	code, err = strconv.Atoi(codeStr)
	if err != nil {
		return 0, "", fmt.Errorf("failed parsing response code %q: %w", codeStr, err)
	}
	return code, output[:idx], nil
}

// waitLocalURL executes `curl` in the router pod every 2 seconds, until the expected response code is returned or the timeout expires.
func waitLocalURL(ctx context.Context, routerPod *corev1.Pod, host, abspath string, expectedCode int, timeout time.Duration) (output string, err error) {
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (done bool, err error) {
		code, out, err := readLocalURL(routerPod, host, abspath)
		if err != nil || code != expectedCode {
			framework.Logf("URL is not ready. Expected code: %d; Response code: %d, err: %v", expectedCode, code, err)
			return false, nil
		}
		output = out
		return true, nil
	})
	return output, err
}

// routeStackBuilder provides helper methods for common operations over the
// deployment + service + endpoint + route resources stack.
type routeStackBuilder struct {
	oc            *exutil.CLI
	namespace     string
	resourceName  string
	agnhostImage  string
	routeSelector labels.Selector
}

// newRouteStackBuilder creates a new routeStackBuilder.
// * oc: the OC client
// * resourceName: the common name to be used when creating or handling deployment, service and route resources.
func newRouteStackBuilder(oc *exutil.CLI, resourceName string, routeSelector labels.Selector) *routeStackBuilder {
	return &routeStackBuilder{
		oc:            oc,
		namespace:     oc.Namespace(),
		resourceName:  resourceName,
		agnhostImage:  image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
		routeSelector: routeSelector,
	}
}

// createRouteStack creates the deployment, service and route for the insecure route type.
func (r *routeStackBuilder) createRouteStack(ctx context.Context, routetype routeType, hostname string, replicas int, timeout time.Duration) (backendServers []string, err error) {
	backendServers, err = r.createDeploymentStack(ctx, routetype, replicas, timeout)
	if err = r.createRoute(routetype, r.resourceName, hostname, "/"); err != nil {
		return nil, err
	}
	return backendServers, nil
}

// createDeploymentStack creates the common deployment and service compatible with the provided routetype.
func (r *routeStackBuilder) createDeploymentStack(ctx context.Context, routetype routeType, replicas int, timeout time.Duration) (backendServers []string, err error) {
	switch routetype {
	case routeTypeInsecure:
		err = r.createServeHostnameDeployment(replicas)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported route type: %s", routetype)
	}
	if err = r.waitDeployment(replicas, timeout); err != nil {
		return nil, err
	}
	return r.exposeDeployment(ctx)
}

// scaleDeployment scales-in/out the common deployment to the specified replicas. It waits for all the pods to be created and returns their names.
func (r *routeStackBuilder) scaleDeployment(ctx context.Context, replicas int, timeout time.Duration) (backendServers []string, err error) {
	if err = r.oc.AsAdmin().Run("scale").Args("deploy", r.resourceName, "--replicas", strconv.Itoa(replicas)).Execute(); err != nil {
		return nil, err
	}

	// wait for the expected number of replicas and fetch their names
	err = wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (done bool, err error) {
		backendServers, err = r.fetchServiceReplicas(ctx)
		if err != nil {
			framework.Logf("error fetching service replicas: %s", err.Error())
			return false, nil
		}
		return len(backendServers) == replicas, nil
	})
	return backendServers, err
}

// createDetachedService creates a new service, endpoint and endpointSlice, detached from the common deployment and its pods by not having a selector.
// It is useful as a way to scale-in a service without removing the underlying pods the service references. See also `scaleDownEndpointSlice()`.
func (r *routeStackBuilder) createDetachedService(ctx context.Context) (serviceName string, err error) {
	svcCurrent, err := r.oc.AsAdmin().AdminKubeClient().CoreV1().Services(r.namespace).Get(ctx, r.resourceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// creating a new service without a pod selector
	serviceName = names.SimpleNameGenerator.GenerateName(r.resourceName + "-")
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svcCurrent.Namespace,
			Name:      serviceName,
		},
		Spec: corev1.ServiceSpec{
			Ports: svcCurrent.Spec.Ports,
			Type:  corev1.ServiceTypeClusterIP,
		},
	}
	if _, err = r.oc.AsAdmin().AdminKubeClient().CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		return "", err
	}

	// I'm observing that creating the Endpoints resource creates a shadow EndpointSlice,
	// flag this to false in case this changes.
	endpointsAPICreatesEndpointSlice := true
	if !endpointsAPICreatesEndpointSlice {
		// fetch the EndpointSlice from the provided service ...
		epsItem, err := r.fetchEndpointSlice(ctx, r.resourceName)
		if err != nil {
			return "", err
		}

		// ... and clone it, attaching to the selector-less service
		epsName := names.SimpleNameGenerator.GenerateName(serviceName + "-")
		eps := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: epsItem.Namespace,
				Name:      epsName,
				Labels:    map[string]string{"kubernetes.io/service-name": serviceName},
			},
			AddressType: epsItem.AddressType,
			Ports:       epsItem.Ports,
			Endpoints:   epsItem.Endpoints,
		}
		if _, err := r.oc.AsAdmin().AdminKubeClient().DiscoveryV1().EndpointSlices(eps.Namespace).Create(ctx, eps, metav1.CreateOptions{}); err != nil {
			return "", err
		}
	}

	// we also need the deprecated Endpoints API, since router still uses it depending on the ROUTER_WATCH_ENDPOINTS envvar
	epCurrent, err := r.oc.AsAdmin().AdminKubeClient().CoreV1().Endpoints(svcCurrent.Namespace).Get(ctx, svcCurrent.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svcCurrent.Namespace,
			Name:      serviceName,
		},
		Subsets: epCurrent.Subsets,
	}
	_, err = r.oc.AsAdmin().AdminKubeClient().CoreV1().Endpoints(ep.Namespace).Create(ctx, ep, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return serviceName, nil
}

// scaleInEndpoints changes the number of replicas of an endpoint and EndpointSlice. This only works on services
// without selector created via `createDetachedService()`. It is useful as a way to scale-in a service and route without
// removing the underlying pods of a deployment.
func (r *routeStackBuilder) scaleInEndpoints(ctx context.Context, detachedServiceName string, replicas int) (backendServers []string, err error) {
	var eps *discoveryv1.EndpointSlice
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		eps, err = r.fetchEndpointSlice(ctx, detachedServiceName)
		if err != nil {
			return err
		}
		if count := len(eps.Endpoints); count < replicas {
			return fmt.Errorf("endpoints can only be scaled-in. found %d replicas, want %d", count, replicas)
		}
		eps.Endpoints = eps.Endpoints[:replicas]
		_, err = r.oc.AsAdmin().AdminKubeClient().DiscoveryV1().EndpointSlices(eps.Namespace).Update(ctx, eps, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		backendServers = make([]string, len(eps.Endpoints))
		for i, ep := range eps.Endpoints {
			backendServers[i] = ep.TargetRef.Name
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ep, err := r.oc.AsAdmin().AdminKubeClient().CoreV1().Endpoints(r.namespace).Get(ctx, detachedServiceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// deleting addresses, from all subnets, whose IP address is not found in the patched `eps`
		for i := range ep.Subsets {
			ss := &ep.Subsets[i]
			ss.Addresses = slices.DeleteFunc(ss.Addresses, func(addr corev1.EndpointAddress) bool {
				return !slices.ContainsFunc(eps.Endpoints, func(e discoveryv1.Endpoint) bool {
					return addr.IP == e.Addresses[0]
				})
			})
		}
		_, err = r.oc.AsAdmin().AdminKubeClient().CoreV1().Endpoints(ep.Namespace).Update(ctx, ep, metav1.UpdateOptions{})
		return err

	})
	return backendServers, err
}

// waitDeployment waits the common deployment to report all its replicas as ready.
func (r *routeStackBuilder) waitDeployment(replicas int, timeout time.Duration) error {
	timeoutStr := fmt.Sprintf("%ds", timeout.Milliseconds()/1e3)
	return r.oc.AsAdmin().Run("wait").Args("--for", "jsonpath={.status.readyReplicas}="+strconv.Itoa(replicas), "--timeout", timeoutStr, "deployment/"+r.resourceName).Execute()
}

// createServeHostnameDeployment creates the common deployment as an insecure (http) backend that responds with its hostname / pod name.
func (r *routeStackBuilder) createServeHostnameDeployment(replicas int) error {
	return r.createDeployment(r.agnhostImage, replicas, 9376, "/agnhost", "serve-hostname")
}

// createDeployment creates the deployment resource. It should be called just once, since it uses the OC namespace and the common resource name.
func (r *routeStackBuilder) createDeployment(image string, replicas, port int, cmd ...string) error {
	runArgs := []string{"deployment", r.resourceName, "--image", image, "--replicas", strconv.Itoa(replicas), "--port", strconv.Itoa(port), "--"}
	runArgs = append(runArgs, cmd...)
	return r.oc.AsAdmin().Run("create").Args(runArgs...).Execute()
}

// exposeDeployment creates a service that exposes the common deployment. It returns all the pod names of the exposed deployment.
func (r *routeStackBuilder) exposeDeployment(ctx context.Context) (backendServers []string, err error) {
	err = r.oc.AsAdmin().Run("expose").Args("deployment", r.resourceName).Execute()
	if err != nil {
		return nil, err
	}
	return r.fetchServiceReplicas(ctx)
}

// fetchEndpointSlice fetches the EndpointSlice of the provided service name. It currently supports only one EndpointSlice instance for simplicity.
func (r *routeStackBuilder) fetchEndpointSlice(ctx context.Context, serviceName string) (*discoveryv1.EndpointSlice, error) {
	listOpts := metav1.ListOptions{LabelSelector: "kubernetes.io/service-name=" + serviceName}
	epsList, err := r.oc.AsAdmin().AdminKubeClient().DiscoveryV1().EndpointSlices(r.namespace).List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	if count := len(epsList.Items); count != 1 {
		// making it simple by returning just one epsName, instead of a list
		return nil, fmt.Errorf("currently only one EndpontSlice is supported, found %d", count)
	}
	return &epsList.Items[0], nil
}

// fetchServiceReplicas fetches the pod names from the exposed common deployment. It requires that `exposeDeployment()` was already called.
func (r *routeStackBuilder) fetchServiceReplicas(ctx context.Context) ([]string, error) {
	svc, err := r.oc.AsAdmin().AdminKubeClient().CoreV1().Services(r.namespace).Get(ctx, r.resourceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	listOpts := metav1.ListOptions{LabelSelector: labels.FormatLabels(svc.Spec.Selector)}
	pods, err := r.oc.AsAdmin().AdminKubeClient().CoreV1().Pods(r.namespace).List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	backendServers := make([]string, len(pods.Items))
	for i := range pods.Items {
		backendServers[i] = pods.Items[i].Name
	}
	return backendServers, nil
}

// createRoute creates a new route of the specified type, exposing the provided service. The service should be compatible with the routetype.
func (r *routeStackBuilder) createRoute(routetype routeType, serviceName, hostname, path string) error {
	// reusing for now
	if err := createRoute(r.oc, routetype, r.resourceName, serviceName, hostname, path); err != nil {
		return err
	}
	return r.oc.AsAdmin().Run("label").Args("route", "--overwrite", r.resourceName, r.routeSelector.String()).Execute()
}
