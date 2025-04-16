package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var _ = g.Describe("[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc      = exutil.NewCLIWithPodSecurityLevel("gatewayapi-controller", admissionapi.LevelBaseline)
		csvName string
		err     error
	)
	const (
		// The expected OSSM subscription name.
		expectedSubscriptionName = "servicemeshoperator3"
		// Expected Subscription Source
		expectedSubscriptionSource = "redhat-operators"
		// The expected OSSM operator namespace.
		expectedSubscriptionNamespace = "openshift-operators"
		// The default gatewayclass name used to create ossm and other gateway api resources.
		gatewayClassName = "openshift-default"
		// gatewayClassControllerName is the name that must be used to create a supported gatewayClass.
		gatewayClassControllerName = "openshift.io/gateway-controller/v1"
		//OSSM Deployment Pod Name
		deploymentOSSMName = "servicemesh-operator3"
		// The default gateway name used to create gateway resource.
		gatewayName = "gateway-default"
	)
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(infra).NotTo(o.BeNil())

		platformType := infra.Status.Platform
		if infra.Status.PlatformStatus != nil {
			platformType = infra.Status.PlatformStatus.Type
		}
		switch platformType {
		case configv1.AWSPlatformType, configv1.AzurePlatformType, configv1.GCPPlatformType, configv1.IBMCloudPlatformType:
			// supported
		default:
			g.Skip(fmt.Sprintf("Skipping on non cloud platform type %q", platformType))
		}

		// create the default gatewayClass
		gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				e2e.Logf("The default gatewayclass already exists, continue...")
			} else {
				e2e.Failf("Failed to create GatewayClass %q", gatewayClassName)
			}
		}
	})

	g.It("Ensure OSSM and OLM related resources are created after creating GatewayClass", func() {
		//check the catalogSource
		g.By("Check OLM catalogSource, subscription, CSV and Pod")
		waitCatalogErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
			catalog, err := oc.AsAdmin().Run("get").Args("-n", "openshift-marketplace", "catalogsource", expectedSubscriptionSource, "-o=jsonpath={.status.connectionState.lastObservedState}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if catalog != "READY" {
				e2e.Logf("CatalogSource %q is not in ready state, retrying...", expectedSubscriptionSource)
				return false, nil
			}
			e2e.Logf("CatalogSource %q is ready!", expectedSubscriptionSource)
			return true, nil
		})
		o.Expect(waitCatalogErr).NotTo(o.HaveOccurred(), "Timed out waiting for CatalogSource %q to become ready", expectedSubscriptionSource)

		// check Subscription
		waitVersionErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
			csvName, err = oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "subscription", expectedSubscriptionName, "-o=jsonpath={.status.installedCSV}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if csvName == "" {
				e2e.Logf("Subscription %q doesn't have installed CSV, retrying...", expectedSubscriptionName)
				return false, nil
			}
			e2e.Logf("Subscription %q has installed CSV: %s", expectedSubscriptionName, csvName)
			return true, nil
		})
		o.Expect(waitVersionErr).NotTo(o.HaveOccurred(), "Timed out waiting for the ClusterServiceVersion to install")

		waitCSVErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
			csvStatus, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "clusterserviceversion", csvName, "-o=jsonpath={.status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if csvStatus != "Succeeded" {
				e2e.Logf("Cluster Service Version %q is not successful, retrying...", csvName)
				return false, nil
			}
			e2e.Logf("Cluster Service Version %q has succeeded!", csvName)
			return true, nil
		})
		o.Expect(waitCSVErr).NotTo(o.HaveOccurred(), "Cluster Service Version %s never reached succeeded status", csvName)

		// get OSSM Operator deployment
		waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
			deployOSSM, err := oc.AdminKubeClient().AppsV1().Deployments(expectedSubscriptionNamespace).Get(context, "servicemesh-operator3", metav1.GetOptions{})
			if err != nil {
				if deployOSSM.Status.ReadyReplicas < 1 {
					e2e.Logf("OSSM operator deployment %q is not ready, retrying...", deploymentOSSMName)
					return false, nil
				}
			}
			e2e.Logf("OSSM operator deployment %q is ready", deploymentOSSMName)
			return true, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "OSSM Operator deployment %q did not successfully deploy its pod", deploymentOSSMName)

		g.By("Confirm that Istio CR is created and in healthy state")
		waitForIstioHealthy(oc)
	})

	g.It("Ensure default gatewayclass is accepted", func() {

		g.By("Check if default GatewayClass is accepted after OLM resources are successful")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

	})

	g.It("Ensure custom gatewayclass can be accepted", func() {
		customGatewayClassName := "gatewayclass-custom"

		g.By("Ensure default GatewayClass is accepted and Istio is healthy")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)
		waitForIstioHealthy(oc)

		g.By("Create Custom GatewayClass")
		gatewayClass := buildGatewayClass(customGatewayClassName, gatewayClassControllerName)
		gwc, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("Gateway Class \"custom-gatewayclass\" already exists, or has failed to be created, checking its status")
		}
		errCheck = checkGatewayClass(oc, customGatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gwc.Name)

		g.By("Deleting Custom GatewayClass and confirming that it is no longer there")
		err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(context.Background(), customGatewayClassName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check if default gatewayClass is accepted and ISTIO CR and pod are still available")
		defaultCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(defaultCheck).NotTo(o.HaveOccurred())

		g.By("Confirm that ISTIO CR is created and in healthy state")
		resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}

		istioStatus, errIstio := oc.AsAdmin().Run("get").Args("-n", resource.Namespace, "istio", resource.Name, "-o=jsonpath={.status.state}").Output()
		o.Expect(errIstio).NotTo(o.HaveOccurred())
		o.Expect(istioStatus).To(o.Equal(`Healthy`))
	})

	g.It("Ensure loadbalancer service and dnsRecord are created for a Gateway object", func() {
		var lbAddress string
		g.By("Ensure default GatewayClass is accepted")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

		g.By("Getting the default domain")
		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")
		defaultDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-default.", 1)

		g.By("Create the default gateway")
		_, gwerr := createAndCheckGateway(oc, gatewayName, gatewayClassName, defaultDomain)
		o.Expect(gwerr).NotTo(o.HaveOccurred(), "failed to create Gateway")

		g.By("Verify the gateway's LoadBalancer service and DNSRecords")
		// check gateway LB service, note that External-IP might be hostname (AWS) or IP (Azure/GCP)
		lbService, err := oc.AdminKubeClient().CoreV1().Services("openshift-ingress").Get(context.Background(), gatewayName+"-openshift-default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if lbService.Status.LoadBalancer.Ingress[0].Hostname != "" {
			lbAddress = lbService.Status.LoadBalancer.Ingress[0].Hostname
		} else {
			lbAddress = lbService.Status.LoadBalancer.Ingress[0].IP
		}
		e2e.Logf("The load balancer External-IP is: %v", lbAddress)

		gwlist, haerr := oc.AdminGatewayApiClient().GatewayV1().Gateways("openshift-ingress").Get(context.Background(), gatewayName, metav1.GetOptions{})
		e2e.Logf("The gateway hostname address is %v ", gwlist.Status.Addresses[0].Value)
		o.Expect(haerr).NotTo(o.HaveOccurred())
		o.Expect(lbAddress).To(o.Equal(gwlist.Status.Addresses[0].Value))

		// get the dnsrecord name
		dnsRecordName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "dnsrecord", "-l", "gateway.networking.k8s.io/gateway-name="+gatewayName, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The gateway API dnsrecord name is: %v", dnsRecordName)
		// check status of published dnsrecord of the gateway, all zones should be True (not contains False)
		dnsRecordStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "dnsrecord", dnsRecordName, `-o=jsonpath={.status.zones[*].conditions[0].status}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The dnsrecords status of all zones: %v", dnsRecordStatus)
		o.Expect(dnsRecordStatus).NotTo(o.ContainSubstring("False"))
	})

	g.It("Ensure HTTPRoute object is created", func() {
		customGatewayName := "gateway-custom"
		ingressNamespace := "openshift-ingress"

		g.By("Ensure default GatewayClass is accepted")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

		g.By("Getting the default domain for creating custom gateway")
		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")
		customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-custom.", 1)

		g.By("Create the custom gateway")
		defer func() {
			_ = oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Delete(context.Background(), customGatewayName, metav1.DeleteOptions{})
		}()
		_, gwerr := createAndCheckGateway(oc, customGatewayName, gatewayClassName, customDomain)
		o.Expect(gwerr).NotTo(o.HaveOccurred(), "failed to create custom Gateway")

		g.By("Confirm the custom gateway is up")
		_, err = checkGatewayStatus(oc, customGatewayName, ingressNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the http route using the default gateway")
		defaultRoutename := "test-hostname." + customDomain
		createHttpRoute(oc, "test-httproute", defaultRoutename, customGatewayName, "echo-pod-"+gatewayName)

		g.By("Checking the http route using the default gateway is accepted")
		assertHttpRouteSuccessful(oc, "test-httproute", customGatewayName)
	})
})

func waitForIstioHealthy(oc *exutil.CLI) {
	resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}
	err := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		istioStatus, errIstio := oc.AsAdmin().Run("get").Args("-n", resource.Namespace, "istio", resource.Name, "-o=jsonpath={.status.state}").Output()
		o.Expect(errIstio).NotTo(o.HaveOccurred())
		if istioStatus != "Healthy" {
			e2e.Logf("Istio CR %q is not healthy, retrying...", resource.Name)
			return false, nil
		}
		e2e.Logf("Istio CR %q is healthy", resource.Name)
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Istio CR %q did not reach healthy state in time", resource.Name)
}

func checkGatewayClass(oc *exutil.CLI, name string) error {

	waitErr := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 10*time.Minute, false, func(context context.Context) (bool, error) {
		gwc, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Get(context, name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("failed to get gatewayclass %s, retrying...", name)
			return false, nil
		}
		for _, condition := range gwc.Status.Conditions {
			if condition.Type == string(gatewayapiv1.GatewayClassConditionStatusAccepted) {
				if condition.Status == metav1.ConditionTrue {
					return true, nil
				}
			}
		}
		e2e.Logf("Found gatewayclass %s but it is not accepted, retrying...", name)
		return false, nil
	})

	o.Expect(waitErr).NotTo(o.HaveOccurred(), "Gatewayclass %s is not accepted", name)
	return nil
}

// buildGatewayClass initializes the GatewayClass and returns its address.
func buildGatewayClass(name, controllerName string) *gatewayapiv1.GatewayClass {
	return &gatewayapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayapiv1.GatewayClassSpec{
			ControllerName: gatewayapiv1.GatewayController(controllerName),
		},
	}
}

// createAndCheckGateway build and creates the Gateway.
func createAndCheckGateway(oc *exutil.CLI, gwname, gwclassname, domain string) (*gatewayapiv1.Gateway, error) {
	ingressNamespace := "openshift-ingress"

	// Build the gateway object
	gatewaybuild := buildGateway(gwname, ingressNamespace, gwclassname, "All", domain)

	// Create the gateway object
	_, errGwObj := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Create(context.TODO(), gatewaybuild, metav1.CreateOptions{})
	if errGwObj != nil {
		return nil, errGwObj
	}

	// Confirm the gateway is up and running
	return checkGatewayStatus(oc, gwname, ingressNamespace)
}

func checkGatewayStatus(oc *exutil.CLI, gatewayName, ingressNamespace string) (*gatewayapiv1.Gateway, error) {
	gateway := &gatewayapiv1.Gateway{}

	waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		gateway, errGwStatus := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Get(context, gatewayName, metav1.GetOptions{})
		if errGwStatus != nil {
			e2e.Logf("Failed to get gateway object, retrying...")
			return false, nil
		}
		// Checking the gateway controller status
		for _, condition := range gateway.Status.Conditions {
			if condition.Type == string(gatewayapiv1.GatewayConditionProgrammed) {
				if condition.Status == metav1.ConditionTrue {
					e2e.Logf("The gateway controller is up and running")
					return true, nil
				}
			}
		}
		e2e.Logf("Found gateway but the controller is still not programmed, retrying...")
		return false, nil
	})

	if waitErr != nil {
		return nil, fmt.Errorf("timed out waiting for gateway %q to become programmed: %w", gatewayName, waitErr)
	}
	e2e.Logf("Gateway %q successfully programmed!", gatewayName)
	return gateway, nil
}

// buildGateway initializes the Gateway and returns its address.
func buildGateway(name, namespace, gcname, fromNs, domain string) *gatewayapiv1.Gateway {
	hostname := gatewayapiv1.Hostname("*." + domain)
	fromNamespace := gatewayapiv1.FromNamespaces(fromNs)
	// Tell the gateway listener to allow routes from the namespace/s in the fromNamespaces variable, which could be "All".
	allowedRoutes := gatewayapiv1.AllowedRoutes{Namespaces: &gatewayapiv1.RouteNamespaces{From: &fromNamespace}}
	listener1 := gatewayapiv1.Listener{Name: "http", Hostname: &hostname, Port: 80, Protocol: "HTTP", AllowedRoutes: &allowedRoutes}

	return &gatewayapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: gatewayapiv1.GatewaySpec{
			GatewayClassName: gatewayapiv1.ObjectName(gcname),
			Listeners:        []gatewayapiv1.Listener{listener1},
		},
	}
}

// createHttpRoute checks if the HTTPRoute can be created.
// If it can't an error is returned.
func createHttpRoute(oc *exutil.CLI, routeName, hostname, gatewayName, backendRefname string) (*gatewayapiv1.HTTPRoute, error) {
	namespace := oc.Namespace()
	ingressNamespace := "openshift-ingress"
	gateway, err := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Get(context.TODO(), gatewayName, metav1.GetOptions{})
	if err != nil || gateway == nil {
		e2e.Failf("Unable to create httpRoute, no gateway available during route assertion %v", err)
	}

	// Create the backend (service and pod) needed for the route to have resolvedRefs=true.
	// The http route, service, and pod are cleaned up when the namespace is automatically deleted.
	// buildEchoPod builds a pod that listens on port 8080.
	echoPod := buildEchoPod(backendRefname, namespace)
	_, echoPodErr := oc.AdminKubeClient().CoreV1().Pods(namespace).Create(context.TODO(), echoPod, metav1.CreateOptions{})
	o.Expect(echoPodErr).NotTo(o.HaveOccurred())

	// buildEchoService builds a service that targets port 8080.
	echoService := buildEchoService(echoPod.Name, namespace, echoPod.ObjectMeta.Labels)
	_, echoServiceErr := oc.AdminKubeClient().CoreV1().Services(namespace).Create(context.Background(), echoService, metav1.CreateOptions{})
	o.Expect(echoServiceErr).NotTo(o.HaveOccurred())

	// Create the HTTPRoute
	buildHTTPRoute := buildHTTPRoute(routeName, namespace, gatewayName, ingressNamespace, hostname, backendRefname)
	httpRoute, err := oc.GatewayApiClient().GatewayV1().HTTPRoutes(namespace).Create(context.Background(), buildHTTPRoute, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Confirm the HTTPRoute is up
	waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		checkHttpRoute, err := oc.GatewayApiClient().GatewayV1().HTTPRoutes(namespace).Get(context, httpRoute.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, condition := range checkHttpRoute.Status.Parents[0].Conditions {
			if condition.Type == string(gatewayapiv1.RouteConditionAccepted) {
				if condition.Status == metav1.ConditionTrue {
					return true, nil
				}
			}
		}
		e2e.Logf("HTTPRoute %s is not ready, retrying...", checkHttpRoute.Name)
		return false, nil
	})
	if waitErr != nil {
		e2e.Failf("HTTPRoute never ready")
	}
	return httpRoute, nil
}

// buildEchoPod returns a pod definition for an socat-based echo server.
func buildEchoPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": name,
			},
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					// Note that HTTP/1.0 will strip the HSTS response header
					Args: []string{
						"TCP4-LISTEN:8080,reuseaddr,fork",
						`EXEC:'/bin/bash -c \"printf \\\"HTTP/1.0 200 OK\r\n\r\n\\\"; sed -e \\\"/^\r/q\\\"\"'`,
					},
					Command: []string{"/bin/socat"},
					Image:   "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest",
					Name:    "echo",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: int32(8080),
							Protocol:      corev1.ProtocolTCP,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: pointer.Bool(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						RunAsNonRoot: pointer.Bool(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}
}

// buildEchoService returns a service definition for an HTTP service.
func buildEchoService(name, namespace string, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       int32(80),
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: labels,
		},
	}
}

// buildHTTPRoute initializes the HTTPRoute and returns its address.
func buildHTTPRoute(routeName, namespace, parentgateway, parentNamespace, hostname, backendRefname string) *gatewayapiv1.HTTPRoute {
	parentns := gatewayapiv1.Namespace(parentNamespace)
	parent := gatewayapiv1.ParentReference{Name: gatewayapiv1.ObjectName(parentgateway), Namespace: &parentns}
	port := gatewayapiv1.PortNumber(int32(80))
	rule := gatewayapiv1.HTTPRouteRule{
		BackendRefs: []gatewayapiv1.HTTPBackendRef{{
			BackendRef: gatewayapiv1.BackendRef{
				BackendObjectReference: gatewayapiv1.BackendObjectReference{
					Name: gatewayapiv1.ObjectName(backendRefname),
					Port: &port,
				},
			},
		}},
	}

	return &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: routeName, Namespace: namespace},
		Spec: gatewayapiv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{ParentRefs: []gatewayapiv1.ParentReference{parent}},
			Hostnames:       []gatewayapiv1.Hostname{gatewayapiv1.Hostname(hostname)},
			Rules:           []gatewayapiv1.HTTPRouteRule{rule},
		},
	}
}

// assertHttpRouteSuccessful checks if the http route was created and has parent conditions that indicate
// it was accepted successfully.  A parent is usually a gateway.  Returns an error not accepted and/or not resolved.
func assertHttpRouteSuccessful(oc *exutil.CLI, httpRouteName, gatewayName string) (*gatewayapiv1.HTTPRoute, error) {
	namespace := oc.Namespace()
	checkHttpRoute := &gatewayapiv1.HTTPRoute{}
	ingressNamespace := "openshift-ingress"
	gateway, err := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Get(context.TODO(), gatewayName, metav1.GetOptions{})
	if err != nil || gateway == nil {
		e2e.Failf("Unable to assert httpRoute, no gateway available, error %v", err)
	}

	// Wait 1 minute for parent/s to update
	err = wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		checkHttpRoute, err := oc.GatewayApiClient().GatewayV1().HTTPRoutes(namespace).Get(context, httpRouteName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		numParents := len(checkHttpRoute.Status.Parents)
		if numParents == 0 {
			e2e.Logf("httpRoute %s/%s has no parent conditions, retrying...", namespace, httpRouteName)
			return false, nil
		}
		e2e.Logf("found httproute %s/%s with %d parent/s", namespace, httpRouteName, numParents)
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	e2e.Logf("httpRoute %s/%s successful", namespace, httpRouteName)
	return checkHttpRoute, nil
}
