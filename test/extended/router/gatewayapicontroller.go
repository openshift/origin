package router

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	sailv1 "github.com/istio-ecosystem/sail-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

var _ = g.Describe("[sig-network][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io]", func() {
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
	)
	g.BeforeEach(func() {
		// create the default gatewayClass and confirm that it is accepted
		gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())

		gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
		gwc, err := gwapiClient.GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("Gateway Class %s already exists, or has failed to be created, checking its status", gwc.Name)
		}

		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred())
		e2e.Logf("GatewayClass %s successfully installed and accepted!", gwc.Name)

	})

	g.Describe("Verify Gateway API controller resources are created", func() {
		g.It("and ensure OSSM related resources are created", func() {
			coreClient := clientset.NewForConfigOrDie(oc.AdminConfig())
			//check the subscription
			g.By("Check OSSM subscription and CSV")
			csvName, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "subscription", expectedSubscriptionName, "-o=jsonpath={.status.installedCSV}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The subscription is installed and the CSV is: %v", csvName)
			csvStatus, err := oc.AsAdmin().Run("get").Args("-n", expectedSubscriptionNamespace, "clusterserviceversion", csvName, "-o=jsonpath={.status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(csvStatus).To(o.Equal("Succeeded"))

			g.By("check OSSM Pod and istiod pod is present with OSSM Operator installation")
			podList, err := coreClient.CoreV1().Pods("openshift-operators").List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(podList).To(o.ContainSubstring(`servicemesh-operator3`))

			podList, err = coreClient.CoreV1().Pods("openshift-ingress").List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(podList).To(o.ContainSubstring(`istiod-openshift-gateway`))

			g.By("Confirm that ISTIO CR is created and in healthy state")
			resource := types.NamespacedName{Namespace: "openshift-ingress", Name: "openshift-gateway"}

			errIstio := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 30*time.Second, false, func(context context.Context) (bool, error) {
				istioStatus, errIstio := oc.AsAdmin().Run("get").Args("-n", resource.Namespace, "istio", resource.Name, "-o=jsonpath={.status.state}").Output()
				if errIstio != nil {
					e2e.Logf("failed to get Istio CR %s, retrying...", "openshift-gateway")
					return false, nil
				}
				if istioStatus == string(sailv1.IstioReasonHealthy) {
					e2e.Logf("Found Istio %s/%s, and it reports ready", resource.Namespace, resource.Name)
					return true, nil
				}
				e2e.Logf("Found Istio %s/%s, but it isn't ready.  Retrying...", resource.Namespace, resource.Name)
				return false, nil
			})
			if errIstio != nil {
				e2e.Failf("Expected ISTIO CR %s not found", "openshift-gateway")
			}

		})
		g.It("and ensure custom gatewayclass can be accepted", func() {
			gwapiClient := gatewayapiclientset.NewForConfigOrDie(oc.AdminConfig())
			coreClient := clientset.NewForConfigOrDie(oc.AdminConfig())

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

			podList, err := coreClient.CoreV1().Pods("openshift-ingress").List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(podList).To(o.ContainSubstring(`istiod-openshift-gateway`))

			istioCR, err := oc.AsAdmin().Run("get").Args("-n", "openshift-ingress", "istio", "openshift-gateway").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(istioCR).To(o.ContainSubstring(`openshift-gateway`))

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

// buildGatewayClass initializes the GatewayClass and returns its address.
func buildGatewayClass(name, controllerName string) *gatewayapiv1.GatewayClass {
	return &gatewayapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayapiv1.GatewayClassSpec{
			ControllerName: gatewayapiv1.GatewayController(controllerName),
		},
	}
}
