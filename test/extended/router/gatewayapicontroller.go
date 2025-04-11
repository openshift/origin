package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	// clientset "k8s.io/client-go/kubernetes"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

var _ = g.Describe("[sig-network][OCPFeatureGate:GatewayAPIController][Feature:Router][Serial][apigroup:gateway.networking.k8s.io]", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("gatewayapi-controller", admissionapi.LevelBaseline)
	)
	const (
		// The expected OSSM subscription name.
		expectedSubscriptionName = "servicemeshoperator3"
		// The expected OSSM operator namespace.
		expectedSubscriptionNamespace = "openshift-operators"
		// The gatewayclass name used to create ossm and other gateway api resources.
		gatewayClassName = "openshift-default"
		// gatewayClassControllerName is the name that must be used to create a supported gatewayClass.
		gatewayClassControllerName = "openshift.io/gateway-controller/v1"
		// istiod deployment name
		deployName = "istiod-openshift-gateway"
	)

	g.Describe("Verify Gateway API controller", func() {
		g.It("and ensure OSSM related resources are created when default gatewayclass is accepted", func() {
			gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
			g.By("Create default gatewayclass")
			gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
			_, err := gwapiClient.GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			//check the subscription
			g.By("Check OSSM subscription and CSV")
			csvName, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "subscription", expectedSubscriptionName, "-o=jsonpath={.status.installedCSV}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The subscription is installed and the CSV is: %v", csvName)

			waitErr := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				csvStatus, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "clusterserviceversion", csvName, "-o=jsonpath={.status.phase}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if csvStatus != "Succeeded" {
					e2e.Logf("CSV status: %v, retrying....", csvStatus)
					return false, nil
				}
				return true, nil
			})
			o.Expect(waitErr).NotTo(o.HaveOccurred(), "CSV is not succeeded in allowed time")

			g.By("check OSSM Pod and istiod pod is present with OSSM Operator installation")
			podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-operators").List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(podList).To(o.ContainSubstring(`servicemesh-operator3`))

			deploy, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-ingress").Get(context.Background(), deployName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(deploy.Status.ReadyReplicas).To(o.Equal(int32(1)))

			g.By("Confirm that ISTIO CR is created and in healthy state")
			resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}

			istioStatus, err := oc.AsAdmin().Run("get").Args("-n", resource.Namespace, "istio", resource.Name, "-o=jsonpath={.status.state}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(istioStatus).To(o.Equal("Healthy"))
		})

		g.It("and ensure default gatewayclass is accepted", func() {
			g.By("Check default gatewayclass is accepted")
			errCheck := checkGatewayClass(oc, gatewayClassName)
			o.Expect(errCheck).NotTo(o.HaveOccurred())
		})

		g.It("and ensure custom gatewayclass can be accepted", func() {
			gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())

			g.By("Create Custom GatewayClass")
			gatewayClass := buildGatewayClass("custom-gateway", gatewayClassControllerName)
			gwc, err := gwapiClient.GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
			if err != nil {
				e2e.Logf("Gateway Class %s already exists, or has failed to be created, checking its status", "custom-gateway")
			}
			errCheck := checkGatewayClass(oc, "custom-gateway")
			o.Expect(errCheck).NotTo(o.HaveOccurred())
			e2e.Logf("GatewayClass %s successfully installed and accepted!", gwc.Name)

			g.By("Deleting Custom GatewayClass and confirming that it is no longer there")
			err = gwapiClient.GatewayV1().GatewayClasses().Delete(context.Background(), "custom-gateway", metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = gwapiClient.GatewayV1().GatewayClasses().Get(context.Background(), "custom-gateway", metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			e2e.Logf("The custom gatewayClass %s has been sucessfully deleted", "custom-gateway")

			g.By("check if default gatewayClass is accepted and ISTIO CR and pod are still available")
			defaultCheck := checkGatewayClass(oc, gatewayClassName)
			o.Expect(defaultCheck).NotTo(o.HaveOccurred())

			deploy, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-ingress").Get(context.Background(), deployName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(deploy.Status.ReadyReplicas).To(o.Equal(1))

			istioCR, err := oc.AsAdmin().Run("get").Args("-n", "openshift-ingress", "istio", "openshift-gateway").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(istioCR).To(o.ContainSubstring(`openshift-gateway`))
		})

		g.It("and HTTPRoute can be created and accepted", func() {
			gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
			gatewayclassName := "openshift-default"
			namespace := oc.Namespace()
			name := "gateway-same"

			g.By("Create dedicated Gateway for the same namespace")
			// in this test, we set gateway listener to "Same"
			sameNamespaces := "Same"
			defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			domain := strings.Replace(defaultIngressDomain, "apps.", "same.", 1)

			buildGateway := buildGateway(name, namespace, gatewayclassName, sameNamespaces, domain)
			gateway, err := gwapiClient.GatewayV1().Gateways(namespace).Create(context.Background(), buildGateway, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create a service for HTTPRoute")
			routeName := "httproute-test"
			hostname := routeName + "." + domain

			httpRouteService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "httproute-test",
					Labels: map[string]string{
						"app": "httproute-test",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "httproute-test",
					},
					Ports: []corev1.ServicePort{
						{
							Port:       8080,
							Name:       "8080-http",
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Services(namespace).Create(context.Background(), httpRouteService, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create HTTPRoute")
			backendRefname := "httproute-test"
			buildHTTPRoute := buildHTTPRoute(routeName, namespace, gateway.Name, namespace, hostname, backendRefname)
			httpRoute, err := gwapiClient.GatewayV1().HTTPRoutes(namespace).Create(context.Background(), buildHTTPRoute, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				checkHttpRoute, err := gwapiClient.GatewayV1().HTTPRoutes(namespace).Get(context, httpRoute.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, condition := range checkHttpRoute.Status.Parents[0].Conditions {
					if condition.Type == string(gatewayapiv1.RouteConditionAccepted) {
						if condition.Status == metav1.ConditionTrue {
							e2e.Logf("HTTPRoute condition: %v", condition)
							return true, nil
						}
					}
				}
				e2e.Logf("HTTPRoute %s is not accepted, retrying...", checkHttpRoute.Name)
				return false, nil
			})
			if waitErr != nil {
				e2e.Failf("HTTPRoute never got accepted")
			}
		})
	})
})

func checkGatewayClass(oc *exutil.CLI, name string) error {
	gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())

	waitErr := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		gwc, err := gwapiClient.GatewayV1().GatewayClasses().Get(context, name, metav1.GetOptions{})
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

	if waitErr != nil {
		return fmt.Errorf("Gatewayclass %s is not accepted", name)
	}
	e2e.Logf("Gateway Class %s is created and accpeted", name)
	return nil
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

// buildGatewayClass initializes the GatewayClass and returns its address.
func buildGatewayClass(name, controllerName string) *gatewayapiv1.GatewayClass {
	return &gatewayapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayapiv1.GatewayClassSpec{
			ControllerName: gatewayapiv1.GatewayController(controllerName),
		},
	}
}

// buildHTTPRoute initializes the HTTPRoute and returns its address.
func buildHTTPRoute(routeName, namespace, parentgateway, parentNamespace, hostname, backendRefname string) *gatewayapiv1.HTTPRoute {
	defaultPortNumber := 8080
	parentns := gatewayapiv1.Namespace(parentNamespace)
	parent := gatewayapiv1.ParentReference{Name: gatewayapiv1.ObjectName(parentgateway), Namespace: &parentns}
	port := gatewayapiv1.PortNumber(defaultPortNumber)
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
