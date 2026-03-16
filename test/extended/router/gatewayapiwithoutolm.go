package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
)

const (
	gatewayClassConditionIstioType = "ControllerInstalled"
	gatewayClassCRDType            = "CRDsReady"

	provisionOSSMFromCIO = "cio-provisions-ossm"
)

var _ = g.Describe("[sig-network-edge][OCPFeatureGate:GatewayAPIWithoutOLM][Feature:Router][apigroup:gateway.networking.k8s.io]", g.Ordered, g.Serial, func() {
	defer g.GinkgoRecover()
	var (
		oc         = exutil.NewCLIWithPodSecurityLevel("gatewayapi-withoutolm", admissionapi.LevelBaseline)
		err        error
		gateways   []string
		infPoolCRD = "https://raw.githubusercontent.com/kubernetes-sigs/gateway-api-inference-extension/main/config/crd/bases/inference.networking.k8s.io_inferencepools.yaml"
	)

	const (
		// The expected OSSM operator namespace.
		expectedSubscriptionNamespace = "openshift-operators"
		// gatewayClassControllerName is the name that must be used to create a supported gatewayClass.
		gatewayClassControllerName = "openshift.io/gateway-controller/v1"
		//OSSM Deployment Pod Name
		deploymentOSSMName          = "servicemesh-operator3"
		openshiftOperatorsNamespace = "openshift-operators"
	)
	g.BeforeEach(func() {
		isokd, err := isOKD(oc)
		if err != nil {
			e2e.Failf("Failed to get clusterversion to determine if release is OKD: %v", err)
		}
		if isokd {
			g.Skip("Skipping on OKD cluster as OSSM is not available as a community operator")
		}

		// skip non clould platforms since gateway needs LB service
		skipGatewayIfNonCloudPlatform(oc)

		// create the default gatewayClass
		gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			e2e.Failf("Failed to create GatewayClass %q: %v", gatewayClassName, err)
		}
	})

	g.AfterEach(func() {
		if !checkAllTestsDone(oc) {
			e2e.Logf("Skipping cleanup while not all GatewayAPIController tests are done")
		} else {
			g.By("Deleting the gateways")

			for _, name := range gateways {
				err = oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Delete(context.Background(), name, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "Gateway %s could not be deleted", name)
			}

			g.By("Deleting the GatewayClass")

			if err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(context.Background(), gatewayClassName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				e2e.Failf("Failed to delete GatewayClass %q", gatewayClassName)
			}

			g.By("Waiting for the istiod pod to be deleted")

			o.Eventually(func(g o.Gomega) {
				podsList, err := oc.AdminKubeClient().CoreV1().Pods(ingressNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app=istiod"})
				g.Expect(err).NotTo(o.HaveOccurred())
				g.Expect(podsList.Items).Should(o.BeEmpty())
			}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())

		}
	})

	g.It("Ensure OSSM is provisioned from CIO with no OLM resources with creation of GatewayClass", func() {
		defer markTestDone(oc, provisionOSSMFromCIO)

		g.By("Check if default GatewayClass is accepted")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

		g.By("Check the GatewayClass conditions to confirm OSSM is provisioned by CIO")
		errCheck = checkCIOConditions(oc, gatewayClassName, gatewayClassConditionIstioType)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q does not have the Istio condition", gatewayClassName)

		errCheck = checkCIOConditions(oc, gatewayClassName, gatewayClassCRDType)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q does not have the CRDType condition", gatewayClassName)

		g.By("Confirm that the GatewayClass has the correct finalizer")
		errCheck = checkGatewayClassFinalizer(oc, gatewayClassName, "openshift.io/ingress-operator-sail-finalizer")
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q does not have the finalizer", gatewayClassName)

		g.By("Confirm that the OLM Subscription, CSV and Pod do not exist")
		checkIfResourceExists(oc, expectedSubscriptionNamespace, "subscription")
		checkIfResourceExists(oc, expectedSubscriptionNamespace, "csv")
		checkIfResourceExists(oc, expectedSubscriptionNamespace, "pod")

		g.By("Confirm there is no Istio CR present")
		_, err = oc.AsAdmin().Run("get").Args("istio").Output()
		o.Expect(err).To(o.HaveOccurred(), "The Istio CR is installed")

		g.By("Ensure the istiod Deployment is present and managed by helm")
		errCheck = checkIstiodLabels(oc, ingressNamespace, istiodDeployment, "Helm")
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "istiod deployment %s does not have the desired label", istiodDeployment)

		g.By("Check the corresponding Istio CRDs are managed by CIO")
		err := istioManagedCRDs(oc)
		o.Expect(err).NotTo((o.HaveOccurred()))
	})

	g.It("Ensure default gatewayclass is accepted", func() {
		defer markTestDone(oc, defaultGatewayclassAccepted)

		g.By("Check if default GatewayClass is accepted after OLM resources are successful")
		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)
	})

	g.It("Ensure custom gatewayclass can be accepted and managed by CIO", func() {
		defer markTestDone(oc, customGatewayclassAccepted)

		customGatewayClassName := "custom-gatewayclass"

		g.By("Create Custom GatewayClass and check if CIO status exists")
		gatewayClass := buildGatewayClass(customGatewayClassName, gatewayClassControllerName)
		gwc, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("Failed to create GatewayClass %q: %v; checking its status...", customGatewayClassName, err)
		}
		errCheck := checkGatewayClass(oc, customGatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gwc.Name)

		errCheck = checkCIOConditions(oc, customGatewayClassName, gatewayClassConditionIstioType)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q does not have the Istio condition", customGatewayClassName)

		errCheck = checkCIOConditions(oc, customGatewayClassName, gatewayClassCRDType)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q does not have the CRDType condition", customGatewayClassName)

		errCheck = checkGatewayClassFinalizer(oc, customGatewayClassName, "openshift.io/ingress-operator-sail-finalizer")
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q does not have the finalizer", customGatewayClassName)

		g.By("Deleting Custom GatewayClass and confirming that it is no longer there")
		err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(context.Background(), customGatewayClassName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Get(context.Background(), customGatewayClassName, metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred(), "The custom gatewayClass \"custom-gatewayclass\" has been sucessfully deleted")

		g.By("check if default gatewayClass is accepted and istiod deployment still exits")
		defaultCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(defaultCheck).NotTo(o.HaveOccurred())
		errCheck = checkIstiodLabels(oc, ingressNamespace, istiodDeployment, "Helm")
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "istiod deployment %s does not have the desired label", istiodDeployment)

	})

	g.It("Ensure LB, service, and dnsRecord are created for a Gateway object", func() {
		defer markTestDone(oc, lbAndServiceAndDnsrecordAreCreated)

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
		defer markTestDone(oc, httprouteObjectCreated)

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

	g.It("Ensure GIE is enabled after creating an inferencePool CRD", func() {
		defer markTestDone(oc, gieEnabled)

		errCheck := checkGatewayClass(oc, gatewayClassName)
		o.Expect(errCheck).NotTo(o.HaveOccurred(), "GatewayClass %q was not installed and accepted", gatewayClassName)

		g.By("Install the GIE CRD")
		err := oc.AsAdmin().Run("create").Args("-f", infPoolCRD).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Confirm istio is healthy and contains the env variable")
		waitForIstioHealthy(oc)
		waitIstioErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 5*time.Minute, false, func(context context.Context) (bool, error) {
			istiod, err := oc.AdminKubeClient().AppsV1().Deployments(ingressNamespace).Get(context, istiodDeployment, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to get istiod deployment %q: %v; retrying...", istiodDeployment, err)
				return false, nil
			}
			envVar := istiod.Spec.Template.Spec.Containers[0].Env
			for _, env := range envVar {
				if env.Name == "ENABLE_GATEWAY_API_INFERENCE_EXTENSION" {
					if env.Value == "true" {
						e2e.Logf("GIE has been enabled, and the env variable is present in Istiod deployment resource")
						return true, nil
					}
				}
			}
			e2e.Logf("GIE env variable is not present, retrying...")
			return false, nil
		})
		o.Expect(waitIstioErr).NotTo(o.HaveOccurred(), "Timed out waiting for Istiod Deployment to have GIE env variable")

		g.By("Uninstall the GIE CRD and confirm the env variable is removed")
		err = oc.AsAdmin().Run("delete").Args("-f", infPoolCRD).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitIstioErr = wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 5*time.Minute, false, func(context context.Context) (bool, error) {
			istiod, err := oc.AdminKubeClient().AppsV1().Deployments(ingressNamespace).Get(context, istiodDeployment, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to get istiod deployment %q: %v; retrying...", istiodDeployment, err)
				return false, nil
			}
			envVar := istiod.Spec.Template.Spec.Containers[0].Env
			for _, env := range envVar {
				if env.Name == "ENABLE_GATEWAY_API_INFERENCE_EXTENSION" {
					e2e.Logf("GIE env variable is still present in Istiod deployment resource, retrying...")
					return false, nil
				}
			}
			e2e.Logf("GIE env variable has been removed from the Istio resource")
			return true, nil
		})
		o.Expect(waitIstioErr).NotTo(o.HaveOccurred(), "Timed out waiting for Istiod to remove GIE env variable")
	})

	g.It("Ensure istiod deployment and the istio could be deleted and then get recreated [Serial]", func() {
		// delete the istiod deployment and then checked if it is restored
		g.By(fmt.Sprintf("Try to delete the istiod deployment in %s namespace", ingressNamespace))
		pollWaitDeploymentReady(oc, ingressNamespace, istiodDeployment)
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(ingressNamespace).Get(context.Background(), istiodDeployment, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AdminKubeClient().AppsV1().Deployments(ingressNamespace).Delete(context.Background(), istiodDeployment, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Wait until the istiod deployment in %s namespace is automatically created successfully", ingressNamespace))
		pollWaitDeploymentCreated(oc, ingressNamespace, istiodDeployment, deployment.CreationTimestamp)
	})

	g.It("Ensure gateway loadbalancer service and dnsrecords could be deleted and then get recreated [Serial]", func() {
		g.By("Getting the default domain for creating a custom Gateway")
		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to find default domain name")
		customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-custom.", 1)

		g.By("Create a custom Gateway")
		gw := names.SimpleNameGenerator.GenerateName("gateway-")
		gateways = append(gateways, gw)
		_, gwerr := createAndCheckGateway(oc, gw, gatewayClassName, customDomain)
		o.Expect(gwerr).NotTo(o.HaveOccurred(), "Failed to create Gateway")

		// verify the gateway's LoadBalancer service
		assertGatewayLoadbalancerReady(oc, gw, gw+"-openshift-default")
		gatewayLbService := gw + "-openshift-default"

		// make sure the DNSRecord is ready to use.
		assertDNSRecordStatus(oc, gw)

		g.By(fmt.Sprintf("Try to delete the gateway lb service %s", gatewayLbService))
		lbService, err := oc.AdminKubeClient().CoreV1().Services(ingressNamespace).Get(context.Background(), gatewayLbService, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AdminKubeClient().CoreV1().Services(ingressNamespace).Delete(context.Background(), gatewayLbService, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Wait until the gateway lb service %s is automatically recreated successfully", gatewayLbService))
		pollWaitGWLBServiceRecreated(oc, ingressNamespace, gatewayLbService, lbService.ObjectMeta.CreationTimestamp)

		// make sure the DNSRecord is ready to use.
		assertDNSRecordStatus(oc, gw)

		// delete the gateway dnsrecords then checked if it is restored
		g.By(fmt.Sprintf("Get some info of the gateway dnsrecords in %s namespace, then try to delete it", ingressNamespace))
		dnsrecordList, err := oc.AdminIngressClient().IngressV1().DNSRecords(ingressNamespace).List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		dnsrecord, err := getGWDNSRecords(dnsrecordList, gw)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AdminIngressClient().IngressV1().DNSRecords(ingressNamespace).Delete(context.Background(), dnsrecord.ObjectMeta.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Wait unitl the gateway dnsrecords in %s namespace is automatically created successfully", ingressNamespace))
		pollWaitGWDNSRecordsRecreated(oc, gw, ingressNamespace, getSortedString(dnsrecord.Spec.Targets), dnsrecord.ObjectMeta.CreationTimestamp)
	})
})

func checkIstiodLabels(oc *exutil.CLI, namespace string, name string, label string) error {
	waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 5*time.Minute, false, func(context context.Context) (bool, error) {
		istiod, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context, name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get istiod deployment %q: %v; retrying...", name, err)
			return false, nil
		}
		for _, istioLabels := range istiod.Labels {
			if istioLabels == label {
				e2e.Logf("Successfully found the label, %s in istiod deployment", label)
				return true, nil
			}
		}
		e2e.Logf("istiod deployment %q does not have the desired label, retrying...", name)
		return false, nil
	})
	o.Expect(waitErr).NotTo(o.HaveOccurred(), "Timed out waiting for the label %s on the deployment %q", label, name)
	return nil
}

func checkIfResourceExists(oc *exutil.CLI, namespace string, name string) {
	_, err := oc.AsAdmin().Run("get").Args("-n", namespace, name).Output()
	o.Expect(err).Should(o.Or(o.Not(o.HaveOccurred()), o.MatchError(apierrors.IsNotFound, "NotFound")))
}

func checkCIOConditions(oc *exutil.CLI, name string, cond string) error {
	timeout := 20 * time.Minute
	waitErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, timeout, false, func(context context.Context) (bool, error) {
		gwc, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Get(context, name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get gatewayclass %s: %v; retrying...", name, err)
			return false, nil
		}
		for _, condition := range gwc.Status.Conditions {
			if condition.Type == cond {
				if condition.Status == metav1.ConditionTrue {
					e2e.Logf("The GatewayClass is managed by CIO with the condition type: %s", cond)
					return true, nil
				}
			}
		}
		e2e.Logf("Could not find the condition %s, retrying...", cond)
		return false, nil
	})

	o.Expect(waitErr).NotTo(o.HaveOccurred(), "GatewayClass %q did not have the condition %s", name, cond)
	return nil
}

func checkGatewayClassFinalizer(oc *exutil.CLI, name string, expectedFinalizer string) error {
	timeout := 5 * time.Minute
	waitErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, timeout, false, func(context context.Context) (bool, error) {
		gwc, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Get(context, name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get gatewayclass %s: %v; retrying...", name, err)
			return false, nil
		}
		for _, finalizer := range gwc.Finalizers {
			if finalizer == expectedFinalizer {
				e2e.Logf("The gatewayClass, %q has the expected finalizer %s", name, expectedFinalizer)
				return true, nil
			}
		}
		e2e.Logf("The gatewayclass %s, does not have the expected finalizer, retrying...", name)
		return false, nil
	})

	o.Expect(waitErr).NotTo(o.HaveOccurred(), "GatewayClass %q could not find the expected finalizer wihin %v", name, timeout)
	return nil
}

func istioManagedCRDs(oc *exutil.CLI) error {
	crdList, err := oc.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	for _, crd := range crdList.Items {
		if strings.Contains(crd.Name, "istio.io") {
			if value, err := crd.Labels["ingress.operator.openshift.io/owned"]; err && value == "true" {
				e2e.Logf("CRD %s has the specfic label value: %s", crd.Name, value)
			} else {
				e2e.Failf("CRD %s, is not managed by Istio!", crd.Name)
			}
		}
	}
	return nil
}
