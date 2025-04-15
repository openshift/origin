package router

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	// clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

var _ = g.Describe("[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io]", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("gatewayapi-controller", admissionapi.LevelBaseline)
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
		// The gateway name used to create gateway resource.
		gatewayName = "standard-gateway"
		// The gateway name used to create custom gateway resource.
		customGatewayName = "custom-gateway"
	)
	g.BeforeEach(func() {
		gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
		// create the default gatewayClass
		gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
		gwc, err := gwapiClient.GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("Gateway Class %s already exists, or has failed to be created", gwc.Name)
		}
	})

	g.Describe("Verify Gateway API controller", func() {
		g.It("and ensure OSSM and OLM related resources are created after creating GatewayClass", func() {
			//check the catalogSource
			g.By("Check OLM catalogSource, subscription, CSV and Pod")
			waitCatalog := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				catalog, err := oc.AsAdmin().Run("get").Args("-n", "openshift-marketplace", "catalogsource", expectedSubscriptionSource, "-o=jsonpath={.status.connectionState.lastObservedState}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if catalog != "READY" {
					e2e.Logf("CatalogSource is not in ready state")
					return false, nil
				}
				e2e.Logf("catalogSource is ready!")
				return true, nil
			})
			if waitCatalog != nil {
				e2e.Failf("catalogSource never got ready")
			}

			// check Subscription
			waitVersion := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				csvName, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "subscription", expectedSubscriptionName, "-o=jsonpath={.status.installedCSV}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if csvName == "" {
					e2e.Logf("Subscription has not created the CSV, retrying")
					return false, nil
				}
				e2e.Logf("The subscription is installed and the CSV is: %v", csvName)
				return true, nil
			})
			if waitVersion != nil {
				e2e.Logf("Subscription does not have an installed CSV")
				err := oc.AsAdmin().Run("delete").Args("-n", expectedSubscriptionNamespace, "subscription", expectedSubscriptionName).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

			}

			// temp workaround to extract the csvName from startingCSV field
			csvName, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "subscription", expectedSubscriptionName, "-o=jsonpath={.status.currentCSV}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			waitCSV := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				csvStatus, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "clusterserviceversion", csvName, "-o=jsonpath={.status.phase}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if csvStatus != "Succeeded" {
					e2e.Logf("Cluster Service Version %s is not successful..., retrying", csvName)
					return false, nil
				}
				e2e.Logf("Cluster Service Version %s has succeeded!", csvName)
				return true, nil
			})
			if waitCSV != nil {
				e2e.Failf("Cluster Service Version never got ready")
			}

			// get OLM Deployment Pod
			waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				deployOSSM, err := oc.AsAdmin().CoreClient().AppsV1().Deployments(expectedSubscriptionNamespace).Get(context, "servicemesh-operator3", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				if deployOSSM.Status.ReadyReplicas != 1 {
					e2e.Logf("Deployment Pod %s is not ready, retrying...", "servicemesh-operator3")
					return false, nil
				}
				e2e.Logf("Deployment Pod %s ready", "servicemesh-operator3")
				return true, nil
			})
			if waitErr != nil {
				e2e.Failf("OSSM Deployment Pod never got ready")
			}

			g.By("Confirm that ISTIO CR is created and in healthy state")
			resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}

			waitCR := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				istioStatus, errIstio := oc.AsAdmin().Run("get").Args("-n", resource.Namespace, "istio", resource.Name, "-o=jsonpath={.status.state}").Output()
				o.Expect(errIstio).NotTo(o.HaveOccurred())
				if istioStatus != "Healthy" {
					e2e.Logf("Istio CR %s is not healthy, retrying...", resource.Name)
					return false, nil
				}
				e2e.Logf("Istio CR %s is in Healthy state!", resource.Name)
				return true, nil
			})
			if waitCR != nil {
				e2e.Failf("Istio CR never reached Healthy state")
			}

			// check OSSM deployment
			waitIstio := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				deployIstio, err := oc.AsAdmin().CoreClient().AppsV1().Deployments("openshift-ingress").Get(context, "istiod-openshift-gateway", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				if deployIstio.Status.ReadyReplicas != 1 {
					e2e.Logf("Deployment Pod %s is not ready, retrying...", "istiod-openshift-gateway")
					return false, nil
				}
				e2e.Logf("Deployment Pod %s ready", "istiod-openshift-gateway")
				return true, nil
			})
			if waitIstio != nil {
				e2e.Failf("Istio Deployment Pod never got ready")
			}

		})
		g.It("and ensure default gatewayclass is accepted", func() {

			g.By("Check if default GatewayClass is accepted after OLM resources are successful")
			errCheck := checkGatewayClass(oc, gatewayClassName)
			o.Expect(errCheck).NotTo(o.HaveOccurred())
			e2e.Logf("GatewayClass %s successfully installed and accepted!", gatewayClassName)

		})
		g.It("and ensure custom gatewayclass can be accepted", func() {
			gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())

			g.By("Create Custom GatewayClass")
			gatewayClass := buildGatewayClass("custom-gatewayclass", gatewayClassControllerName)
			gwc, err := gwapiClient.GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
			if err != nil {
				e2e.Logf("Gateway Class %s already exists, or has failed to be created, checking its status", "custom-gatewayclass")
			}
			errCheck := checkGatewayClass(oc, "custom-gatewayclass")
			o.Expect(errCheck).NotTo(o.HaveOccurred())
			e2e.Logf("GatewayClass %s successfully installed and accepted!", gwc.Name)

			g.By("Deleting Custom GatewayClass and confirming that it is no longer there")
			err = gwapiClient.GatewayV1().GatewayClasses().Delete(context.Background(), "custom-gatewayclass", metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = gwapiClient.GatewayV1().GatewayClasses().Get(context.Background(), "custom-gatewayclass", metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			e2e.Logf("The custom gatewayClass %s has been sucessfully deleted", "custom-gatewayclass")

			g.By("check if default gatewayClass is accepted and ISTIO CR and pod are still available")
			defaultCheck := checkGatewayClass(oc, gatewayClassName)
			o.Expect(defaultCheck).NotTo(o.HaveOccurred())

			waitIstio := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
				deployIstio, err := oc.AsAdmin().CoreClient().AppsV1().Deployments("openshift-ingress").Get(context, "istiod-openshift-gateway", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				if deployIstio.Status.ReadyReplicas != 1 {
					e2e.Logf("Deployment Pod %s is not ready, retrying...", "istiod-openshift-gateway")
					return false, nil
				}
				e2e.Logf("Deployment Pod %s ready", "istiod-openshift-gateway")
				return true, nil
			})
			if waitIstio != nil {
				e2e.Failf("Istio Deployment Pod never got ready")
			}

			g.By("Confirm that ISTIO CR is created and in healthy state")
			resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}

			istioStatus, errIstio := oc.AsAdmin().Run("get").Args("-n", resource.Namespace, "istio", resource.Name, "-o=jsonpath={.status.state}").Output()
			o.Expect(errIstio).NotTo(o.HaveOccurred())
			o.Expect(istioStatus).To(o.Equal(`Healthy`))
		})

		g.It("and ensure OSSM subscription and istio get recreated after deleting them", func() {
			const (
				operatorNamespace      = "openshift-operators"
				ingressNamespace       = "openshift-ingress"
				ossmSubscriptionName   = "servicemeshoperator3"
				csvName                = "servicemeshoperator3.v3.0.0"
				ossmOperatorDeployment = "servicemesh-operator3"
				istiodDeployment       = "istiod-openshift-gateway"
				istioName              = "openshift-gateway"
			)

			// deleted the OSSM subscription and then checked if it was restored
			g.By(fmt.Sprintf("Try to delete the subscription %s", ossmSubscriptionName))
			_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", operatorNamespace, "subscription/"+ossmSubscriptionName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Wait untill the the OSSM subscription %s is automatically created successfully", ossmSubscriptionName))
			var unhealthy string
			o.Eventually(func() string {
				var err error
				unhealthy, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", operatorNamespace, "subscription", ossmSubscriptionName, `-o=jsonpath={.status.conditions[?(@.type=="CatalogSourcesUnhealthy")].status}`).Output()
				if err != nil {
					e2e.Logf("Failed to check %s, error: %v, retrying...", ossmSubscriptionName, err)
				}
				e2e.Logf("Wait CatalogSourcesUnhealthy status to be False, and got %s", unhealthy)
				return unhealthy
			}, 5*time.Minute, time.Second).Should(o.Equal("False"))

			g.By(fmt.Sprintf("Wait untill the the OSSM csv %s is automatically created successfully", csvName))
			var phase string
			o.Eventually(func() string {
				var err error
				phase, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", operatorNamespace, "csv", csvName, `-o=jsonpath={.status.phase}`).Output()
				if err != nil {
					e2e.Logf("Failed to check %s, error: %v, retrying...", csvName, err)
				}
				e2e.Logf(fmt.Sprintf("Wait for phase to be Succeeded, and got %s", phase))
				return phase
			}, 5*time.Minute, time.Second).Should(o.Equal("Succeeded"))

			// deleted the istiod deployment and then checked if it was restored
			deleteDeploymentAndWaitAvailableAgain(oc, istiodDeployment, ingressNamespace)

			// deleted the istio and check if it was restored
			g.By(fmt.Sprintf("Try to delete the istio %s", istioName))
			output, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ingressNamespace, "istio/"+istioName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("deleted"))

			g.By(fmt.Sprintf("Wait untill the the istiod %s is automatically created successfully", istioName))
			o.Eventually(func() string {
				readyReplicas, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ingressNamespace, "istio/"+istioName, `-o=jsonpath={.status.revisions.ready}`).Output()
				if err != nil {
					e2e.Logf("Failed to check istio %s, error: %v, retrying...", istioName, err)
				}
				e2e.Logf("Wait for the ready replicas to be 1, and got %s", readyReplicas)
				return readyReplicas
			}, 5*time.Minute, time.Second).Should(o.Equal("1"))
		})

		g.It("and ensure gateway loadbalancer service and dnsrecords get recreated after deleting them", func() {
			const (
				operatorNamespace = "openshift-operators"
				ingressNamespace  = "openshift-ingress"
				gatewayDeployment = "gateway-openshift-default"
				gatewayLbService  = "gateway-openshift-default"
				gatewayName       = "gateway"
			)

			// ensure default gateway objects is created
			gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
			coreClient := clientset.NewForConfigOrDie(oc.AdminConfig())
			g.By("Getting the default domain")
			defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")
			defaultDomain := strings.Split(defaultIngressDomain, "apps.")[1]

			g.By("Create the default API Gateway if it is not created")
			_, errGwStatus := gwapiClient.GatewayV1().Gateways(ingressNamespace).Get(context.Background(), gatewayName, metav1.GetOptions{})
			if errGwStatus != nil {
				e2e.Logf("Failed to get gateway object, so create the gateway for the following test")
				_, err := createAndCheckGateway(oc, gatewayName, gatewayClassName, defaultDomain)
				o.Expect(err).NotTo(o.HaveOccurred(), "failed to create Gateway")
			}

			g.By("Ensure the gateway's LoadBalancer service and DNSRecords are available")
			gwlbAddress, err := ensureLbServiceRetrieveLbAddress(oc, ingressNamespace, gatewayLbService)
			o.Expect(err).NotTo(o.HaveOccurred())

			gatewayAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "gateway", gatewayName, "-o=jsonpath={.status.addresses[0].value}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The gateway address is: %v", gatewayAddress)
			o.Expect(gwlbAddress).To(o.Equal(gatewayAddress))

			// get the dnsrecord name
			dnsRecordName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "dnsrecord", "-l", "gateway.networking.k8s.io/gateway-name="+gatewayName, "-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The gateway API dnsrecord name is: %v", dnsRecordName)
			// check whether status of dns reccord is True
			dnsRecordstatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "dnsrecord", dnsRecordName, `-o=jsonpath={.status.zones[0].conditions[0].status}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dnsRecordstatus).To(o.Equal("True"))

			// deleted the gateway loadbalancer service and then checked if it was restored
			g.By(fmt.Sprintf("Try to delete the gateway lb service %s", gatewayLbService))
			lbService, err := coreClient.CoreV1().Services(ingressNamespace).Get(context.Background(), gatewayLbService, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			createdTime1 := lbService.ObjectMeta.CreationTimestamp
			err = oc.AdminKubeClient().CoreV1().Services(ingressNamespace).Delete(context.Background(), gatewayLbService, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Wait until the gateway lb service %s is automatically recreated successfully", gatewayLbService))
			o.Eventually(func() bool {
				lbService, err := coreClient.CoreV1().Services(ingressNamespace).Get(context.Background(), gatewayLbService, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return false
					}
					e2e.Logf("Error getting the gateway lb service %s: %v, retrying...", gatewayLbService, err)
					return false
				}

				createdTime2 := lbService.ObjectMeta.CreationTimestamp
				if createdTime2 == createdTime1 {
					return false
				}

				lb := lbService.Status.LoadBalancer
				searchInfo := regexp.MustCompile("(IP:([0-9\\.a-fA-F:]+))|(Hostname:([0-9\\.\\-a-zA-Z]+))").FindStringSubmatch(lb.String())
				if len(searchInfo) > 0 {
					if gwlb := searchInfo[2]; len(gwlb) > 0 {
						e2e.Logf("New load balancer ip %s is available", gwlb)
						return true
					}
					if gwlb := searchInfo[4]; len(gwlb) > 0 {
						e2e.Logf("New load balancer hostname %s is available", gwlb)
						return true
					}
				}
				e2e.Logf("Failed to get the new IP or hostname of the gateway lb service %s, retrying...", gatewayLbService)
				return false
			}, 5*time.Minute, 3*time.Second).Should(o.Equal(true))

			// deleted the gateway dnsrecords then checked if it was restored
			g.By(fmt.Sprintf("Get some info of the gateway dnsrecords in %s namespace, then try to delete it", ingressNamespace))
			dnsrecordName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ingressNamespace, "dnsrecords", "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			targetsIPList1, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ingressNamespace, "dnsrecords/"+dnsrecordName, "-o=jsonpath={.spec.targets[*]}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			targetsIPList1 = getSortedString(targetsIPList1)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-ingress", "dnsrecords/"+dnsrecordName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Wait unitl the gateway dnsrecords in %s namespace is automatically created successfully", ingressNamespace))
			o.Eventually(func() bool {
				targetsIPList2, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ingressNamespace, "dnsrecords/"+dnsrecordName, "-o=jsonpath={.spec.targets[*]}").Output()
				if err != nil {
					if errors.IsNotFound(err) {
						return false
					}
					e2e.Logf("Error getting the gateway dnsrecords: %v, retrying...", err)
					return false
				}

				if getSortedString(targetsIPList2) != targetsIPList1 {
					e2e.Logf("The gateway dnsrecords has not a targetsIP or a different one %s with %s, retrying...", getSortedString(targetsIPList2), targetsIPList1)
					return false
				}

				status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "dnsrecords/"+dnsrecordName, "-o=jsonpath={.status.zones[*].conditions[0].status}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				reason, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-ingress", "dnsrecords/"+dnsrecordName, "-o=jsonpath={.status.zones[*].conditions[0].reason}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Count(status, "True") == len(strings.Split(status, " ")) && strings.Count(reason, "ProviderSuccess") == len(strings.Split(reason, " ")) {
					return true
				}
				e2e.Logf("The status of the gateway dnsrecords does not become normal, retrying...")
				return false
			}, 3*time.Minute, 3*time.Second).Should(o.Equal(true))
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
	e2e.Logf("Gateway Class %s is created and accepted", name)
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
	gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
	ingressNameSpace := "openshift-ingress"

	// Build the gateway object
	gatewaybuild := buildGateway(gwname, ingressNameSpace, gwclassname, "All", domain)

	// Create the gateway object
	_, errGwObj := gwapiClient.GatewayV1().Gateways(ingressNameSpace).Create(context.TODO(), gatewaybuild, metav1.CreateOptions{})
	if errGwObj != nil {
		return nil, errGwObj
	}

	// Confirm the gateway is up and running
	return checkGatewayStatus(oc, gwname, ingressNameSpace)
}

func checkGatewayStatus(oc *exutil.CLI, gwname, ingressNameSpace string) (*gatewayapiv1.Gateway, error) {
	gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
	gateway := &gatewayapiv1.Gateway{}

	waitErr := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		gateway, errGwStatus := gwapiClient.GatewayV1().Gateways(ingressNameSpace).Get(context, gwname, metav1.GetOptions{})
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
		e2e.Failf("The gateway is still not up and running and here is error %v", waitErr)
		return gateway, waitErr
	}
	return gateway, waitErr
}

// buildGateway initializes the Gateway and returns its address.
func buildGateway(name, namespace, gcname, fromNs, domain string) *gatewayapiv1.Gateway {
	hostname := gatewayapiv1.Hostname("*." + "gwapi." + domain)
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
func createHttpRoute(oc *exutil.CLI, routeName, hostname, backendRefname string) (*gatewayapiv1.HTTPRoute, error) {
	gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())

	namespace := oc.Namespace()
	ingressNameSpace := "openshift-ingress"
	gateway, errGwStatus := gwapiClient.GatewayV1().Gateways(ingressNameSpace).Get(context.TODO(), "custom-gateway", metav1.GetOptions{})
	if errGwStatus != nil || gateway == nil {
		e2e.Failf("Unable to create httpRoute, no gateway available during route assertion %v", errGwStatus)
	}

	// Create the backend (service and pod) needed for the route to have resolvedRefs=true.
	// The http route, service, and pod are cleaned up when the namespace is automatically deleted.
	// buildEchoPod builds a pod that listens on port 8080.
	echoPod := buildEchoPod(backendRefname, namespace)
	_, echoPodErr := oc.AsAdmin().CoreClient().CoreV1().Pods(namespace).Create(context.TODO(), echoPod, metav1.CreateOptions{})
	o.Expect(echoPodErr).NotTo(o.HaveOccurred())

	// buildEchoService builds a service that targets port 8080.
	echoService := buildEchoService(echoPod.Name, namespace, echoPod.ObjectMeta.Labels)
	_, echoServiceErr := oc.AsAdmin().CoreClient().CoreV1().Services(namespace).Create(context.Background(), echoService, metav1.CreateOptions{})
	o.Expect(echoServiceErr).NotTo(o.HaveOccurred())

	// Create the HTTPRoute
	buildHTTPRoute := buildHTTPRoute(routeName, namespace, gateway.Name, ingressNameSpace, hostname, backendRefname)
	httpRoute, err := gwapiClient.GatewayV1().HTTPRoutes(namespace).Create(context.Background(), buildHTTPRoute, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Confirm the HTTPRoute is up
	waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		checkHttpRoute, err := gwapiClient.GatewayV1().HTTPRoutes(namespace).Get(context, httpRoute.Name, metav1.GetOptions{})
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
func assertHttpRouteSuccessful(oc *exutil.CLI, name string) (*gatewayapiv1.HTTPRoute, error) {
	namespace := oc.Namespace()
	checkHttpRoute := &gatewayapiv1.HTTPRoute{}
	ingressNameSpace := "openshift-ingress"
	gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
	gateway, errGwStatus := gwapiClient.GatewayV1().Gateways(ingressNameSpace).Get(context.TODO(), "custom-gateway", metav1.GetOptions{})
	if errGwStatus != nil || gateway == nil {
		e2e.Failf("Unable to assert httpRoute, no gateway available, error %v", errGwStatus)
	}

	// Wait 1 minute for parent/s to update
	err := wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 2*time.Minute, false, func(context context.Context) (bool, error) {
		checkHttpRoute, err := gwapiClient.GatewayV1().HTTPRoutes(namespace).Get(context, name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		numParents := len(checkHttpRoute.Status.Parents)
		if numParents == 0 {
			e2e.Logf("httpRoute %s/%s has no parent conditions, retrying...", namespace, name)
			return false, nil
		}
		e2e.Logf("found httproute %s/%s with %d parent/s", namespace, name, numParents)
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	e2e.Logf("httpRoute %s/%s successful", namespace, name)
	return checkHttpRoute, nil
}

// used to delete a deployment and wait for it is automatically recreated again
func deleteDeploymentAndWaitAvailableAgain(oc *exutil.CLI, deploymentName, ns string) {
	g.By(fmt.Sprintf("Try to delete the deployment %s in %s namespace", deploymentName, ns))
	client := clientset.NewForConfigOrDie(oc.AdminConfig())
	err := client.AppsV1().Deployments(ns).Delete(context.Background(), deploymentName, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("Wait until the deployment %s in %s namespace is recreated and returns back healthy", deploymentName, ns))
	err = wait.Poll(3*time.Second, 300*time.Second, func() (bool, error) {
		deployment, err := client.AppsV1().Deployments(ns).Get(context.Background(), deploymentName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			e2e.Logf("Error getting %s deployment: %v, retrying", deploymentName, err)
			return false, nil
		}

		readyReplicas := deployment.Status.ReadyReplicas
		e2e.Logf("The ready replicas is %v", readyReplicas)
		if readyReplicas != 1 {
			e2e.Logf(`The deployment %s in %s namespace is not ready(AvailableReplicas: %v), retrying`, deploymentName, ns, deployment.Status.AvailableReplicas)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func ensureLbServiceRetrieveLbAddress(oc *exutil.CLI, ingressNamespace, gatewayLbService string) (string, error) {
	var gwlb string
	coreClient := clientset.NewForConfigOrDie(oc.AdminConfig())
	logCount := 0
	err := wait.Poll(3*time.Second, 300*time.Second, func() (bool, error) {
		lbService, err := coreClient.CoreV1().Services(ingressNamespace).Get(context.Background(), gatewayLbService, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			e2e.Logf("Error getting the gateway lb service %s: %v, retrying...", gatewayLbService, err)
			return false, nil
		}

		lb := lbService.Status.LoadBalancer
		if logCount%10 == 0 {
			e2e.Logf("The lbService.Status.LoadBalancer is:\n%s", lb.String())
		}
		logCount++

		searchInfo := regexp.MustCompile("(IP:([0-9\\.a-fA-F:]+))|(Hostname:([0-9\\.\\-a-zA-Z]+))").FindStringSubmatch(lb.String())
		if len(searchInfo) > 0 {
			if gwlb = searchInfo[2]; len(gwlb) > 0 {
				e2e.Logf("New load balancer ip %s is available", gwlb)
				return true, nil
			}
			if gwlb = searchInfo[4]; len(gwlb) > 0 {
				e2e.Logf("New load balancer hostname %s is available", gwlb)
				return true, nil
			}
		} else {
			e2e.Logf("Failed to get a new load balancer ip or hostname, retrying")
		}
		return false, nil
	})
	return gwlb, err
}

// used to sort string type of slice or string which can be transformed to the slice
func getSortedString(obj interface{}) string {
	objList := []string{}
	str, ok := obj.(string)
	if ok {
		objList = strings.Split(str, " ")
	}
	strList, ok := obj.([]string)
	if ok {
		objList = strList
	}
	sort.Strings(objList)
	return strings.Join(objList, " ")
}
