package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	operatoringressv1 "github.com/openshift/api/operatoringress/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var (
	requiredCapabilities = []configv1.ClusterVersionCapability{
		configv1.ClusterVersionCapabilityMarketplace,
		configv1.ClusterVersionCapabilityOperatorLifecycleManager,
	}
)

const (
	// Max time duration for the DNS resolution
	dnsResolutionTimeout = 10 * time.Minute
	// Max time duration for the Load balancer address
	loadBalancerReadyTimeout = 10 * time.Minute
)

var _ = g.Describe("[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io]", g.Ordered, g.Serial, func() {
	defer g.GinkgoRecover()
	var (
		oc       = exutil.NewCLIWithPodSecurityLevel("gatewayapi-controller", admissionapi.LevelBaseline)
		csvName  string
		err      error
		gateways []string
	)

	const (
		// The expected OSSM subscription name.
		expectedSubscriptionName = "servicemeshoperator3"
		// Expected Subscription Source
		expectedSubscriptionSource = "redhat-operators"
		// The expected OSSM operator namespace.
		expectedSubscriptionNamespace = "openshift-operators"
		// The gatewayclass name used to create ossm and other gateway api resources.
		gatewayClassName = "openshift-default"
		// gatewayClassControllerName is the name that must be used to create a supported gatewayClass.
		gatewayClassControllerName = "openshift.io/gateway-controller/v1"
		//OSSM Deployment Pod Name
		deploymentOSSMName = "servicemesh-operator3"
	)
	g.BeforeAll(func() {
		isokd, err := isOKD(oc)
		if err != nil {
			e2e.Failf("Failed to get clusterversion to determine if release is OKD: %v", err)
		}
		if isokd {
			g.Skip("Skipping on OKD cluster as OSSM is not available as a community operator")
		}

		// skip non clould platforms since gateway needs LB service
		skipGatewayIfNonCloudPlatform(oc)

		// GatewayAPIController relies on OSSM OLM operator.
		// Skipping on clusters which don't have capabilities required
		// to install an OLM operator.
		exutil.SkipIfMissingCapabilities(oc, requiredCapabilities...)

		// create the default gatewayClass
		gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			e2e.Failf("Failed to create GatewayClass %q", gatewayClassName)
		}
	})

	g.AfterAll(func() {
		g.By("Cleaning up the GatewayAPI Objects")
		for _, name := range gateways {
			err = oc.AdminGatewayApiClient().GatewayV1().Gateways("openshift-ingress").Delete(context.Background(), name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "Gateway %s could not be deleted", name)
		}
	})

	g.It("Ensure OSSM and OLM related resources are created after creating GatewayClass", func() {
		//check the catalogSource
		g.By("Check OLM catalogSource, subscription, CSV and Pod")
		waitCatalogErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 20*time.Minute, false, func(context context.Context) (bool, error) {
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
		waitVersionErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 20*time.Minute, false, func(context context.Context) (bool, error) {
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

		waitCSVErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 20*time.Minute, false, func(context context.Context) (bool, error) {
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
		waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 20*time.Minute, false, func(context context.Context) (bool, error) {
			deployOSSM, err := oc.AdminKubeClient().AppsV1().Deployments(expectedSubscriptionNamespace).Get(context, "servicemesh-operator3", metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to get OSSM operator deployment %q, retrying...", deploymentOSSMName)
				return false, nil
			}
			if deployOSSM.Status.ReadyReplicas < 1 {
				e2e.Logf("OSSM operator deployment %q is not ready, retrying...", deploymentOSSMName)
				return false, nil
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
		customGatewayClassName := "custom-gatewayclass"

		g.By("Create Custom GatewayClass")
		gatewayClass := buildGatewayClass(customGatewayClassName, gatewayClassControllerName)
		gwc, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("Gateway Class \"custom-gatewayclass\" already exists, or has failed to be created, checking its status")
		}
		errCheck := checkGatewayClass(oc, customGatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gwc.Name)

		g.By("Deleting Custom GatewayClass and confirming that it is no longer there")
		err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(context.Background(), customGatewayClassName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Get(context.Background(), customGatewayClassName, metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred(), "The custom gatewayClass \"custom-gatewayclass\" has been sucessfully deleted")

		g.By("check if default gatewayClass is accepted and ISTIO CR and pod are still available")
		defaultCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(defaultCheck).NotTo(o.HaveOccurred())

		g.By("Confirm that ISTIO CR is created and in healthy state")
		waitForIstioHealthy(oc)
	})

	g.It("Ensure LB, service, and dnsRecord are created for a Gateway object", func() {
		g.By("Ensure default GatewayClass is accepted")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

		g.By("Getting the default domain")
		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")
		defaultDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-default.", 1)

		g.By("Create the default Gateway")
		gw := names.SimpleNameGenerator.GenerateName("gateway-")
		gateways = append(gateways, gw)
		_, gwerr := createAndCheckGateway(oc, gw, gatewayClassName, defaultDomain)
		o.Expect(gwerr).NotTo(o.HaveOccurred(), "failed to create Gateway")

		g.By("Verify the gateway's LoadBalancer service and DNSRecords")
		assertGatewayLoadbalancerReady(oc, gw, gw+"-openshift-default")

		// check the dns record is created and status of the published dnsrecord of all zones are True
		assertDNSRecordStatus(oc, gw)
	})

	g.It("Ensure HTTPRoute object is created", func() {
		g.By("Ensure default GatewayClass is accepted")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

		g.By("Getting the default domain")
		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to find default domain name")
		customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-custom.", 1)

		g.By("Create a custom Gateway for the HTTPRoute")
		gw := names.SimpleNameGenerator.GenerateName("gateway-")
		gateways = append(gateways, gw)
		_, gwerr := createAndCheckGateway(oc, gw, gatewayClassName, customDomain)
		o.Expect(gwerr).NotTo(o.HaveOccurred(), "Failed to create Gateway")

		// make sure the DNSRecord is ready to use.
		assertDNSRecordStatus(oc, gw)

		g.By("Create the http route using the custom gateway")
		defaultRoutename := "test-hostname." + customDomain
		createHttpRoute(oc, gw, "test-httproute", defaultRoutename, "echo-pod-"+gw)

		g.By("Checking the http route using the default gateway is accepted")
		assertHttpRouteSuccessful(oc, gw, "test-httproute")

		g.By("Validating the http connectivity to the backend application")
		assertHttpRouteConnection(defaultRoutename)
	})
})

func skipGatewayIfNonCloudPlatform(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(infra).NotTo(o.BeNil())

	o.Expect(infra.Status.PlatformStatus).NotTo(o.BeNil())
	platformType := infra.Status.PlatformStatus.Type
	o.Expect(platformType).NotTo(o.BeEmpty())
	switch platformType {
	case configv1.AWSPlatformType, configv1.AzurePlatformType, configv1.GCPPlatformType, configv1.IBMCloudPlatformType:
		// supported
	default:
		g.Skip(fmt.Sprintf("Skipping on non cloud platform type %q", platformType))
	}
}

func waitForIstioHealthy(oc *exutil.CLI) {
	resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}
	err := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 10*time.Minute, false, func(context context.Context) (bool, error) {
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
			e2e.Logf("Failed to get gatewayclass %s, retrying...", name)
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
	ingressNameSpace := "openshift-ingress"

	// Build the gateway object
	gatewaybuild := buildGateway(gwname, ingressNameSpace, gwclassname, "All", domain)

	// Create the gateway object
	_, errGwObj := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNameSpace).Create(context.TODO(), gatewaybuild, metav1.CreateOptions{})
	if errGwObj != nil {
		return nil, errGwObj
	}

	// Confirm the gateway is up and running
	return checkGatewayStatus(oc, gwname, ingressNameSpace)
}

func checkGatewayStatus(oc *exutil.CLI, gwname, ingressNameSpace string) (*gatewayapiv1.Gateway, error) {
	programmedGateway := &gatewayapiv1.Gateway{}
	if err := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 10*time.Minute, false, func(context context.Context) (bool, error) {
		gateway, err := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNameSpace).Get(context, gwname, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get gateway %q: %v, retrying...", gwname, err)
			return false, nil
		}
		// Checking the gateway controller status
		for _, condition := range gateway.Status.Conditions {
			if condition.Type == string(gatewayapiv1.GatewayConditionProgrammed) {
				if condition.Status == metav1.ConditionTrue {
					e2e.Logf("The gateway controller for gateway %q is programmed", gwname)
					programmedGateway = gateway
					return true, nil
				}
			}
		}
		e2e.Logf("Found gateway %q but the controller is still not programmed, retrying...", gwname)
		return false, nil
	}); err != nil {
		return nil, fmt.Errorf("timed out waiting for gateway %q to become programmed: %w", gwname, err)
	}
	e2e.Logf("Gateway %q successfully programmed!", gwname)
	return programmedGateway, nil
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

// assertGatewayLoadbalancerReady verifies that the given gateway has the service's load balancer address assigned.
func assertGatewayLoadbalancerReady(oc *exutil.CLI, gwName, gwServiceName string) {
	// check gateway LB service, note that External-IP might be hostname (AWS) or IP (Azure/GCP)
	var lbAddress string
	err := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, loadBalancerReadyTimeout, false, func(context context.Context) (bool, error) {
		lbService, err := oc.AdminKubeClient().CoreV1().Services("openshift-ingress").Get(context, gwServiceName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get service %q: %v, retrying...", gwServiceName, err)
			return false, nil
		}
		if lbService.Status.LoadBalancer.Ingress[0].Hostname != "" {
			lbAddress = lbService.Status.LoadBalancer.Ingress[0].Hostname
		} else {
			lbAddress = lbService.Status.LoadBalancer.Ingress[0].IP
		}
		if lbAddress == "" {
			e2e.Logf("No load balancer address for service %q, retrying", gwServiceName)
			return false, nil
		}
		e2e.Logf("Got load balancer address for service %q: %v", gwServiceName, lbAddress)

		gw, err := oc.AdminGatewayApiClient().GatewayV1().Gateways("openshift-ingress").Get(context, gwName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get gateway %q, retrying...", gwName)
			return false, nil
		}
		for _, gwAddr := range gw.Status.Addresses {
			if gwAddr.Value == lbAddress {
				return true, nil
			}
		}

		e2e.Logf("Gateway %q does not have service load balancer address, retrying...", gwName)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for gateway %q to get load balancer address of service %q", gwName, gwServiceName)
}

// assertDNSRecordStatus polls until the DNSRecord's status in the default operand namespace is True.
func assertDNSRecordStatus(oc *exutil.CLI, gatewayName string) {
	// find the DNS Record and confirm its zone status is True
	err := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 10*time.Minute, false, func(context context.Context) (bool, error) {
		gatewayDNSRecord := &operatoringressv1.DNSRecord{}
		gatewayDNSRecords, err := oc.AdminIngressClient().IngressV1().DNSRecords("openshift-ingress").List(context, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Failed to list DNS records for gateway %q: %v, retrying...", gatewayName, err)
			return false, nil
		}

		// get the desired DNS records of the given gateway
		for _, record := range gatewayDNSRecords.Items {
			if record.Labels["gateway.networking.k8s.io/gateway-name"] == gatewayName {
				gatewayDNSRecord = &record
				break
			}
		}

		// checking the gateway DNS record status
		for _, zone := range gatewayDNSRecord.Status.Zones {
			for _, condition := range zone.Conditions {
				if condition.Type == "Published" && condition.Status == "True" {
					return true, nil
				}
			}
		}
		e2e.Logf("DNS record %q is not ready, retrying...", gatewayDNSRecord.Name)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for gateway %q DNSRecord to become ready", gatewayName)
}

// createHttpRoute checks if the HTTPRoute can be created.
// If it can't an error is returned.
func createHttpRoute(oc *exutil.CLI, gwName, routeName, hostname, backendRefname string) (*gatewayapiv1.HTTPRoute, error) {
	namespace := oc.Namespace()
	ingressNameSpace := "openshift-ingress"
	gateway, errGwStatus := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNameSpace).Get(context.TODO(), gwName, metav1.GetOptions{})
	if errGwStatus != nil || gateway == nil {
		e2e.Failf("Unable to create httpRoute, no gateway available during route assertion %v", errGwStatus)
	}

	// Create the backend (service and pod) needed for the route to have resolvedRefs=true.
	// The httproute, service, and pod are cleaned up when the namespace is automatically deleted.
	// buildEchoPod builds a pod that listens on port 8080.
	// Use regular user to create pod, service and httproute.
	echoPod := buildEchoPod(backendRefname, namespace)
	_, echoPodErr := oc.KubeClient().CoreV1().Pods(namespace).Create(context.TODO(), echoPod, metav1.CreateOptions{})
	o.Expect(echoPodErr).NotTo(o.HaveOccurred())

	// buildEchoService builds a service that targets port 8080.
	echoService := buildEchoService(echoPod.Name, namespace, echoPod.ObjectMeta.Labels)
	_, echoServiceErr := oc.KubeClient().CoreV1().Services(namespace).Create(context.Background(), echoService, metav1.CreateOptions{})
	o.Expect(echoServiceErr).NotTo(o.HaveOccurred())

	// Create the HTTPRoute
	buildHTTPRoute := buildHTTPRoute(routeName, namespace, gateway.Name, ingressNameSpace, hostname, backendRefname)
	httpRoute, err := oc.GatewayApiClient().GatewayV1().HTTPRoutes(namespace).Create(context.Background(), buildHTTPRoute, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Confirm the HTTPRoute is up
	waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 4*time.Minute, false, func(context context.Context) (bool, error) {
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
func assertHttpRouteSuccessful(oc *exutil.CLI, gwName, name string) (*gatewayapiv1.HTTPRoute, error) {
	namespace := oc.Namespace()
	checkHttpRoute := &gatewayapiv1.HTTPRoute{}
	ingressNameSpace := "openshift-ingress"
	gateway, errGwStatus := oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNameSpace).Get(context.TODO(), gwName, metav1.GetOptions{})
	if errGwStatus != nil || gateway == nil {
		e2e.Failf("Unable to assert httproute, no gateway available, error %v", errGwStatus)
	}

	// Wait up to 4 minutes for parent(s) to update.
	err := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 4*time.Minute, false, func(context context.Context) (bool, error) {
		checkHttpRoute, err := oc.GatewayApiClient().GatewayV1().HTTPRoutes(namespace).Get(context, name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		numParents := len(checkHttpRoute.Status.Parents)
		if numParents == 0 {
			e2e.Logf("HTTPRoute %s/%s has no parent conditions, retrying...", namespace, name)
			return false, nil
		}
		e2e.Logf("Found httproute %s/%s with %d parent/s", namespace, name, numParents)
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	e2e.Logf("HTTPRoute %s/%s successful", namespace, name)
	return checkHttpRoute, nil
}

// assertHttpRouteConnection checks if the http route of the given name replies successfully,
// and returns an error if not
func assertHttpRouteConnection(hostname string) {
	// Create the http client to check the response status code.
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	err := wait.PollUntilContextTimeout(context.Background(), 20*time.Second, dnsResolutionTimeout, false, func(context context.Context) (bool, error) {
		_, err := net.LookupHost(hostname)
		if err != nil {
			e2e.Logf("[%v] Failed to resolve HTTP route's hostname %q: %v, retrying...", time.Now(), hostname, err)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for HTTP route's hostname %q to be resolved: %v", hostname, err)

	// Wait for http route to respond, and when it does, check for the status code.
	err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 5*time.Minute, false, func(context context.Context) (bool, error) {
		statusCode, err := getHttpResponse(client, hostname)
		if err != nil {
			e2e.Logf("HTTP GET request to %q failed: %v, retrying...", hostname, err)
			return false, nil
		}
		if statusCode != http.StatusOK {
			e2e.Logf("Unexpected status code for HTTP GET request to %q: %v, retrying...", hostname, statusCode)
			return false, nil // retry on 503 as pod/service may not be ready
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for successful HTTP GET response from %q: %v", hostname, err)
}

func getHttpResponse(client *http.Client, hostname string) (int, error) {
	// Send the HTTP request.
	response, err := client.Get("http://" + hostname)
	if err != nil {
		return 0, err
	}

	// Close response body.
	defer response.Body.Close()

	return response.StatusCode, nil
}

// Check for the existence of the okd-scos string in the version name to determine if it is OKD
func isOKD(oc *exutil.CLI) (bool, error) {
	current, err := exutil.GetCurrentVersion(context.TODO(), oc.AdminConfig())
	if err != nil {
		return false, err
	}
	if strings.Contains(current, "okd-scos") {
		return true, nil
	}
	return false, nil
}
