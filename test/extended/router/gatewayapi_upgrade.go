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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayAPIUpgradeTest verifies that Gateway API resources work during upgrade,
// whether using OLM-based or CIO-based (NO-OLM) provisioning
type GatewayAPIUpgradeTest struct {
	oc               *exutil.CLI
	namespace        string
	gatewayName      string
	routeName        string
	hostname         string
	startedWithNoOLM bool // tracks if GatewayAPIWithoutOLM was enabled at start
}

func (t *GatewayAPIUpgradeTest) Name() string {
	return "gateway-api-upgrade"
}

func (t *GatewayAPIUpgradeTest) DisplayName() string {
	return "[sig-network-edge][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API functionality during upgrade"
}

// Setup creates Gateway and HTTPRoute resources and tests connectivity
func (t *GatewayAPIUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	g.By("Setting up Gateway API upgrade test")

	t.oc = exutil.NewCLIWithFramework(f)
	t.namespace = f.Namespace.Name
	t.gatewayName = "upgrade-test-gateway"
	t.routeName = "test-httproute"

	g.By("Checking if GatewayAPIWithoutOLM feature gate is enabled before upgrade")
	t.startedWithNoOLM = isNoOLMFeatureGateEnabled(t.oc)

	if t.startedWithNoOLM {
		framework.Logf("Starting with GatewayAPIWithoutOLM enabled (NO-OLM mode)")
	} else {
		framework.Logf("Starting with OLM-based Gateway API provisioning")
	}

	g.By("Creating default GatewayClass to trigger Gateway API installation")
	gatewayClassControllerName := "openshift.io/gateway-controller/v1"
	gatewayClass := buildGatewayClass(gatewayClassName, gatewayClassControllerName)
	_, err := t.oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(ctx, gatewayClass, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		framework.Failf("Failed to create GatewayClass %q: %v", gatewayClassName, err)
	}

	g.By("Waiting for GatewayClass to be accepted")
	err = checkGatewayClassCondition(t.oc, gatewayClassName, string(gatewayv1.GatewayClassConditionStatusAccepted), metav1.ConditionTrue)
	o.Expect(err).NotTo(o.HaveOccurred(), "GatewayClass %q was not accepted", gatewayClassName)

	g.By("Getting the default domain")
	defaultIngressDomain, err := getDefaultIngressClusterDomainName(t.oc, time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to find default domain name")
	customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-upgrade.", 1)
	t.hostname = "test-upgrade." + customDomain

	g.By("Creating Gateway")
	_, err = createAndCheckGateway(t.oc, t.gatewayName, gatewayClassName, customDomain)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create Gateway")

	g.By("Verify the gateway's LoadBalancer service and DNSRecords")
	assertGatewayLoadbalancerReady(t.oc, t.gatewayName, t.gatewayName+"-openshift-default")
	assertDNSRecordStatus(t.oc, t.gatewayName)

	if !t.startedWithNoOLM {
		g.By("Validating OLM-based provisioning before upgrade")
		validateOLMBasedOSSM(t.oc.AsAdmin(), 20*time.Minute)
		framework.Logf("GatewayAPI resources successfully created with OLM-based provisioning")
	} else {
		g.By("Validating CIO-based (NO-OLM) provisioning before upgrade")
		t.validateCIOProvisioning(ctx, false) // false = no migration occurred
		framework.Logf("GatewayAPI resources successfully created with CIO-based (NO-OLM) provisioning")
	}

	g.By("Creating HTTPRoute with backend")
	backendName := "echo-backend-" + t.gatewayName
	createHttpRoute(t.oc.AsAdmin(), t.gatewayName, t.routeName, t.hostname, backendName)

	g.By("Waiting for HTTPRoute to be accepted")
	_, err = assertHttpRouteSuccessful(t.oc.AsAdmin(), t.gatewayName, t.routeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying HTTP connectivity before upgrade")
	assertHttpRouteConnection(t.hostname)
	framework.Logf("HTTPRoute connectivity verified before upgrade")
}

// Test validates that resources continue working during upgrade and validates provisioning method
func (t *GatewayAPIUpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, _ upgrades.UpgradeType) {
	g.By("Validating Gateway API resources remain functional after upgrade")

	// Block until upgrade completes
	g.By("Waiting for upgrade to complete")
	<-done

	g.By("Verifying Gateway still exists and is programmed")
	_, err := checkGatewayStatus(t.oc, t.gatewayName, ingressNamespace)
	o.Expect(err).NotTo(o.HaveOccurred(), "Gateway should remain programmed")

	g.By("Checking if GatewayAPIWithoutOLM feature gate is enabled after upgrade")
	endsWithNoOLM := isNoOLMFeatureGateEnabled(t.oc)

	// Determine if migration happened: started with OLM, ended with NO-OLM
	migrationOccurred := !t.startedWithNoOLM && endsWithNoOLM
	if migrationOccurred {
		framework.Logf("Migration detected: started with OLM, ended with NO-OLM")
	} else {
		framework.Logf("No migration occurred (started with NO-OLM=%v, ended with NO-OLM=%v)", t.startedWithNoOLM, endsWithNoOLM)
	}

	if endsWithNoOLM {
		g.By("GatewayAPIWithoutOLM is enabled - validating CIO-based (NO-OLM) provisioning")
		t.validateCIOProvisioning(ctx, migrationOccurred)
	} else {
		g.By("GatewayAPIWithoutOLM is disabled - validating OLM-based provisioning")
		// A shorter timeout here is because the resources should already exist post-upgrade state.
		validateOLMBasedOSSM(t.oc.AsAdmin(), 2*time.Minute)
	}

	g.By("Verifying HTTPRoute still exists and is accepted after upgrade")
	_, err = assertHttpRouteSuccessful(t.oc.AsAdmin(), t.gatewayName, t.routeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying HTTP connectivity after upgrade")
	assertHttpRouteConnection(t.hostname)

	if migrationOccurred {
		framework.Logf("Gateway API successfully migrated from OLM to CIO (NO-OLM) during upgrade")
	} else if endsWithNoOLM {
		framework.Logf("Gateway API using CIO-based (NO-OLM) provisioning - no migration occurred")
	} else {
		framework.Logf("Gateway API remains on OLM-based provisioning after upgrade")
	}
}

// validateCIOProvisioning validates that Gateway API is using CIO-based (NO-OLM) provisioning
// If migrationOccurred is true, validates the migration from OLM to CIO Sail Library
func (t *GatewayAPIUpgradeTest) validateCIOProvisioning(ctx context.Context, migrationOccurred bool) {
	g.By("Verifying Istiod control plane is running")
	err := checkIstiodRunning(t.oc, 2*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying CIO has taken ownership via GatewayClass")
	// Check GatewayClass has CIO sail library finalizer
	err = checkGatewayClassFinalizer(t.oc, gatewayClassName, "openshift.io/ingress-operator-sail-finalizer")
	o.Expect(err).NotTo(o.HaveOccurred(), "GatewayClass should have CIO sail library finalizer")

	// Check GatewayClass has required CIO conditions
	err = checkGatewayClassCondition(t.oc, gatewayClassName, gatewayClassControllerInstalledConditionType, metav1.ConditionTrue)
	o.Expect(err).NotTo(o.HaveOccurred(), "GatewayClass should have ControllerInstalled condition")

	err = checkGatewayClassCondition(t.oc, gatewayClassName, gatewayClassCRDsReadyConditionType, metav1.ConditionTrue)
	o.Expect(err).NotTo(o.HaveOccurred(), "GatewayClass should have CRDsReady condition")

	g.By("Verifying istiod deployment is managed by sail library")
	err = checkIstiodManagedBySailLibrary(t.oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "Istiod should be managed by sail library")

	if migrationOccurred {
		g.By("Verifying Istio CRDs remain managed by OLM after migration")
		// When migrating from OLM, CRDs were installed by OLM and should remain OLM-managed
		err = assertIstioCRDsOwnedByOLM(t.oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Istio CRDs should remain OLM-managed after migration")

		g.By("Verifying OLM subscription still exists after migration")
		// The OLM Subscription for Sail Operator should still exist (it's not removed during migration)
		_, err = t.oc.AsAdmin().Run("get").Args("subscription", "-n", expectedSubscriptionNamespace, expectedSubscriptionName, "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Sail Operator subscription should still exist after migration")

		g.By("Verifying Istio CR was removed during migration")
		_, err = t.oc.AsAdmin().Run("get").Args("istio", istioName).Output()
		o.Expect(err).To(o.HaveOccurred(), "Istio CR %q should not exist", istioName)

		framework.Logf("Successfully validated OLM to NO-OLM migration")
	} else {
		g.By("Verifying Istio CRDs are managed by CIO")
		// When using CIO from the start, CRDs are CIO-managed
		err := assertIstioCRDsOwnedByCIO(t.oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Istio CRDs should be CIO-managed when using NO-OLM from the start")

		g.By("Verifying CIO-managed resources")
		// When using CIO from the start, Istio CR should not exist (CIO uses Sail Library directly)
		_, err = t.oc.AsAdmin().Run("get").Args("istio", istioName).Output()
		o.Expect(err).To(o.HaveOccurred(), "Istio CR should not exist when using CIO-based provisioning")
		framework.Logf("Successfully validated CIO-based (NO-OLM) provisioning")
	}
}

// Teardown cleans up Gateway API resources, Istio CR, and OSSM subscription
// This runs even if the test fails, ensuring complete cleanup
func (t *GatewayAPIUpgradeTest) Teardown(ctx context.Context, f *framework.Framework) {
	g.By("Deleting the GatewayClass")
	err := t.oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(ctx, gatewayClassName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Failed to delete GatewayClass %q: %v", gatewayClassName, err)
	}

	g.By("Deleting the Sail Operator subscription")
	// This doesn't get deleted by the CIO, so must manually clean up
	err = t.oc.AsAdmin().Run("delete").Args("--ignore-not-found=true", "subscription", "-n", expectedSubscriptionNamespace, expectedSubscriptionName).Execute()
	if err != nil {
		framework.Logf("Failed to delete Subscription %q: %v", expectedSubscriptionName, err)
	}

	g.By("Deleting Sail Operator CSV by label selector")
	// Delete CSV using label selector to handle any version (e.g., servicemeshoperator3.v3.2.0)
	labelSelector := fmt.Sprintf("operators.coreos.com/%s", serviceMeshOperatorName)
	err = t.oc.AsAdmin().Run("delete").Args("csv", "-n", expectedSubscriptionNamespace, "-l", labelSelector, "--ignore-not-found=true").Execute()
	if err != nil {
		framework.Logf("Failed to delete CSV with label %q: %v", labelSelector, err)
	}

	g.By("Deleting the Istio CR if it exists")
	// This should get cleaned up by the CIO, but this is here just in case of failure
	err = t.oc.AsAdmin().Run("delete").Args("--ignore-not-found=true", "istio", istioName).Execute()
	if err != nil {
		framework.Logf("Failed to delete Istio CR %q: %v", istioName, err)
	}

	g.By("Deleting the Gateway")
	err = t.oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Delete(ctx, t.gatewayName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Failed to delete Gateway %q: %v", t.gatewayName, err)
	}

	g.By("Waiting for istiod pods to be deleted")
	waitForIstiodPodDeletion(t.oc)

	g.By("Deleting Istio CRDs to clean up migration state")
	// Delete Istio CRDs so subsequent NO-OLM tests don't see OLM-managed CRDs
	crdList, err := t.oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		framework.Logf("Failed to list CRDs: %v", err)
	} else {
		for _, crd := range crdList.Items {
			if strings.HasSuffix(crd.Name, "istio.io") {
				err := t.oc.AsAdmin().Run("delete").Args("--ignore-not-found=true", "crd", crd.Name).Execute()
				if err != nil {
					framework.Logf("Failed to delete CRD %q: %v", crd.Name, err)
				}
			}
		}
	}

	framework.Logf("Gateway API resources, Istio CR, OSSM subscription, CSV, and Istio CRDs successfully cleaned up")
}
