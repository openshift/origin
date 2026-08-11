package router

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"

	exutil "github.com/openshift/origin/test/extended/util"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayAPIManagementModeUpgradeTest verifies that Gateway API management mode
// transitions work correctly during upgrades and resources remain functional
type GatewayAPIManagementModeUpgradeTest struct {
	oc                    *exutil.CLI
	namespace             string
	gatewayClassName      string
	gatewayName           string
	routeName             string
	hostname              string
	lbAddress             string
	startMode             operatorv1alpha1.GatewayAPIManagementMode
	loadBalancerSupported bool
	managedDNS            bool
}

func (t *GatewayAPIManagementModeUpgradeTest) Name() string {
	return "gateway-api-management-mode-upgrade"
}

func (t *GatewayAPIManagementModeUpgradeTest) DisplayName() string {
	return "[sig-network-edge][OCPFeatureGate:GatewayAPIManagementMode][Feature:Router][apigroup:operator.openshift.io] Verify Gateway API management mode transitions during upgrade"
}

// Skip checks if this upgrade test should be skipped
func (t *GatewayAPIManagementModeUpgradeTest) Skip(_ upgrades.UpgradeContext) bool {
	oc := exutil.NewCLIForMonitorTest("gateway-api-mgmt-mode-upgrade-skip").AsAdmin()

	// Check if feature gate is enabled
	if !exutil.IsTechPreviewNoUpgrade(context.Background(), oc.AdminConfigClient()) {
		e2e.Logf("Skipping: GatewayAPIManagementMode feature is not in TechPreviewNoUpgrade")
		return true
	}

	skip, reason, err := shouldSkipGatewayAPITests(oc, true) // NoOLM is default/GA
	if err != nil {
		e2e.Logf("Failed to check Gateway API skip conditions: %v", err)
		return true
	}
	if skip {
		e2e.Logf("Skipping test: %s", reason)
		return true
	}

	return false
}

// Setup creates Gateway resources and records initial management mode
func (t *GatewayAPIManagementModeUpgradeTest) Setup(ctx context.Context, f *e2e.Framework) {
	g.By("Setting up Gateway API management mode upgrade test")

	t.oc = exutil.NewCLIWithFramework(f).AsAdmin()
	t.namespace = f.Namespace.Name

	// Get platform capabilities
	t.loadBalancerSupported, t.managedDNS = getPlatformCapabilities(t.oc)

	g.By("Recording initial management mode before upgrade")
	ingress, err := getIngressCR(ctx, t.oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	t.startMode = ingress.Spec.GatewayAPI.ManagementMode
	if t.startMode == "" {
		t.startMode = operatorv1alpha1.GatewayAPIManagementModeManaged
	}
	e2e.Logf("Starting with management mode: %s", t.startMode)

	// Ensure we're in Managed mode for test setup
	if t.startMode != operatorv1alpha1.GatewayAPIManagementModeManaged {
		g.By("Transitioning to Managed mode for setup")
		err = setManagementMode(ctx, t.oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, t.oc, operatorv1alpha1.GatewayAPIManagementModeManaged, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.By("Creating GatewayClass")
	t.gatewayClassName = "upgrade-test-mgmt-mode"
	gatewayClass := buildGatewayClass(t.gatewayClassName, "openshift.io/gateway-controller/v1")
	_, err = t.oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(ctx, gatewayClass, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		e2e.Failf("Failed to create GatewayClass: %v", err)
	}

	err = checkGatewayClassCondition(t.oc, t.gatewayClassName, string(gatewayv1.GatewayClassConditionStatusAccepted), metav1.ConditionTrue)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Creating Gateway")
	defaultIngressDomain, err := getDefaultIngressClusterDomainName(t.oc, 1*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-upgrade-mgmt.", 1)

	t.gatewayName = "upgrade-mgmt-mode-gateway"
	// Randomize the hostname so a retry against the same cluster never
	// reuses a public DNS name that a prior attempt's create/delete cycle
	// may have left negatively cached by an intermediate resolver.
	t.hostname = fmt.Sprintf("test-upgrade-mgmt-%d.%s", rand.Intn(10000), customDomain)

	_, err = createAndCheckGateway(t.oc, t.gatewayName, t.gatewayClassName, customDomain, t.loadBalancerSupported)
	o.Expect(err).NotTo(o.HaveOccurred())

	if t.loadBalancerSupported {
		g.By("Verifying LoadBalancer service is ready")
		t.lbAddress = assertGatewayLoadbalancerReady(t.oc, t.gatewayName, t.gatewayName+"-"+t.gatewayClassName)
	}

	if t.managedDNS {
		g.By("Verifying DNS controller creates DNSRecord")
		assertDNSRecordStatus(t.oc, t.gatewayName)
	}

	g.By("Creating HTTPRoute with backend")
	t.routeName = "test-upgrade-mgmt-route"
	backendName := "echo-backend-" + t.gatewayName
	createHttpRoute(t.oc, t.gatewayName, t.routeName, t.hostname, backendName)

	g.By("Waiting for HTTPRoute to be accepted")
	_, err = assertHttpRouteSuccessful(t.oc, t.gatewayName, t.routeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	if t.loadBalancerSupported && t.managedDNS {
		g.By("Verifying HTTP connectivity before upgrade")
		assertHttpRouteConnectionViaAddress(t.lbAddress, t.hostname)
		e2e.Logf("HTTPRoute connectivity verified before upgrade")
	}

	e2e.Logf("Setup complete: Gateway and HTTPRoute created in %s mode", t.startMode)
}

// Test validates resources after upgrade and tests mode transitions
func (t *GatewayAPIManagementModeUpgradeTest) Test(ctx context.Context, f *e2e.Framework, done <-chan struct{}, _ upgrades.UpgradeType) {
	g.By("Validating Gateway API management mode functionality after upgrade")

	// Block until upgrade completes
	g.By("Waiting for upgrade to complete")
	<-done

	g.By("Verifying Gateway still exists and is programmed")
	_, err := checkGatewayStatus(t.oc, t.gatewayName, ingressNamespace, t.loadBalancerSupported)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying HTTPRoute still exists and is accepted")
	_, err = assertHttpRouteSuccessful(t.oc, t.gatewayName, t.routeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	if t.loadBalancerSupported && t.managedDNS {
		g.By("Verifying HTTP connectivity after upgrade")
		assertHttpRouteConnectionViaAddress(t.lbAddress, t.hostname)
	}

	g.By("Checking current management mode after upgrade")
	ingress, err := getIngressCR(ctx, t.oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	currentMode := ingress.Spec.GatewayAPI.ManagementMode
	if currentMode == "" {
		currentMode = operatorv1alpha1.GatewayAPIManagementModeManaged
	}
	e2e.Logf("Current management mode after upgrade: %s", currentMode)

	// Test mode transitions in both directions
	g.By("Testing mode transitions after upgrade")

	// Transition 1: Current mode → Opposite mode
	var targetMode1 operatorv1alpha1.GatewayAPIManagementMode
	if currentMode == operatorv1alpha1.GatewayAPIManagementModeManaged {
		targetMode1 = operatorv1alpha1.GatewayAPIManagementModeUnmanaged
	} else {
		targetMode1 = operatorv1alpha1.GatewayAPIManagementModeManaged
	}

	g.By(fmt.Sprintf("Transitioning from %s to %s", currentMode, targetMode1))
	err = setManagementMode(ctx, t.oc, targetMode1)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = waitForManagementModeTransition(ctx, t.oc, targetMode1, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	t.validateModeState(ctx, targetMode1)

	g.By("Verifying Gateway and HTTPRoute remain functional after first transition")
	_, err = checkGatewayStatus(t.oc, t.gatewayName, ingressNamespace, t.loadBalancerSupported)
	o.Expect(err).NotTo(o.HaveOccurred())

	_, err = assertHttpRouteSuccessful(t.oc, t.gatewayName, t.routeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Transition 2: Opposite mode → Original mode
	targetMode2 := currentMode

	g.By(fmt.Sprintf("Transitioning from %s back to %s", targetMode1, targetMode2))
	err = setManagementMode(ctx, t.oc, targetMode2)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = waitForManagementModeTransition(ctx, t.oc, targetMode2, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	t.validateModeState(ctx, targetMode2)

	g.By("Verifying Gateway and HTTPRoute remain functional after second transition")
	_, err = checkGatewayStatus(t.oc, t.gatewayName, ingressNamespace, t.loadBalancerSupported)
	o.Expect(err).NotTo(o.HaveOccurred())

	_, err = assertHttpRouteSuccessful(t.oc, t.gatewayName, t.routeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	if t.loadBalancerSupported && t.managedDNS {
		g.By("Verifying HTTP connectivity still works after both transitions")
		assertHttpRouteConnectionViaAddress(t.lbAddress, t.hostname)
	}

	// Verify DNS and controller reconciliation if in Managed mode
	if targetMode2 == operatorv1alpha1.GatewayAPIManagementModeManaged {
		g.By("Verifying controllers are actively reconciling in Managed mode")

		if t.managedDNS {
			g.By("Verifying DNS controller is reconciling DNSRecords")
			assertDNSRecordStatus(t.oc, t.gatewayName)
		}

		g.By("Verifying gateway-status controller is updating Gateway status")
		_, err = checkGatewayStatus(t.oc, t.gatewayName, ingressNamespace, t.loadBalancerSupported)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	e2e.Logf("Successfully tested management mode transitions after upgrade: %s → %s → %s",
		currentMode, targetMode1, targetMode2)
}

// validateModeState verifies the cluster state matches the expected management mode
func (t *GatewayAPIManagementModeUpgradeTest) validateModeState(ctx context.Context, expectedMode operatorv1alpha1.GatewayAPIManagementMode) {
	if expectedMode == operatorv1alpha1.GatewayAPIManagementModeManaged {
		g.By("Validating Managed mode state")

		err := checkIngressCondition(ctx, t.oc, "GatewayAPICRDsManaged", metav1.ConditionTrue, "")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = assertVAPExists(ctx, t.oc, gwapiCRDVAPName)
		o.Expect(err).NotTo(o.HaveOccurred(), "VAP should exist in Managed mode")

		err = checkIstiodRunning(t.oc, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Istiod should be running in Managed mode")

		e2e.Logf("Validated Managed mode state: VAP and Istiod are present")
	} else {
		g.By("Validating Unmanaged mode state")

		err := checkIngressCondition(ctx, t.oc, "GatewayAPICRDsManaged", metav1.ConditionFalse, "Unmanaged")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = assertVAPDeleted(ctx, t.oc, gwapiCRDVAPName)
		o.Expect(err).NotTo(o.HaveOccurred(), "VAP should be deleted in Unmanaged mode")

		waitForIstiodPodDeletion(t.oc)

		e2e.Logf("Validated Unmanaged mode state: VAP and Istiod are removed")
	}

	// CRDs should always be present regardless of mode
	g.By("Verifying Gateway API CRDs are still present")
	assertGatewayAPICRDsInstalled(ctx, t.oc)
}

// Teardown cleans up Gateway API resources
func (t *GatewayAPIManagementModeUpgradeTest) Teardown(ctx context.Context, f *e2e.Framework) {
	if t.oc == nil || t.gatewayName == "" {
		e2e.Logf("Skipping cleanup because setup did not initialize resources")
		return
	}

	g.By("Ensuring Managed mode for cleanup")
	err := setManagementMode(ctx, t.oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
	if err != nil {
		e2e.Logf("Failed to set Managed mode during cleanup: %v", err)
	} else {
		_ = waitForManagementModeTransition(ctx, t.oc, operatorv1alpha1.GatewayAPIManagementModeManaged, 5*time.Minute)
	}

	g.By("Deleting HTTPRoute")
	err = t.oc.AdminGatewayApiClient().GatewayV1().HTTPRoutes(t.namespace).Delete(ctx, t.routeName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("Failed to delete HTTPRoute: %v", err)
	}

	g.By("Deleting Gateway")
	err = t.oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Delete(ctx, t.gatewayName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("Failed to delete Gateway: %v", err)
	}

	g.By("Waiting for Gateway deployment to be deleted")
	if err := waitForGatewayDeploymentDeletion(t.oc, t.gatewayName); err != nil {
		e2e.Logf("Gateway deployment was not cleaned up: %v", err)
	}

	g.By("Deleting GatewayClass")
	err = t.oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(ctx, t.gatewayClassName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("Failed to delete GatewayClass: %v", err)
	}

	e2e.Logf("Gateway API management mode upgrade test cleanup complete")
}
