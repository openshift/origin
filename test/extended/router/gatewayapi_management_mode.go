package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	promclient "github.com/openshift/origin/test/extended/prometheus/client"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/prometheus/common/model"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	condutils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// ingressCRName is the singleton Ingress CR name
	ingressCRName = "cluster"

	// bundleVersionAnnotation is the annotation key used for CRD compliance checking
	bundleVersionAnnotation = "gateway.networking.k8s.io/bundle-version"

	// gwapiCRDVAPName is the ValidatingAdmissionPolicy protecting Gateway API CRDs
	gwapiCRDVAPName = "openshift-ingress-operator-gatewayapi-crd-admission"

	// Gateway API CRD names that should be installed
	gatewayClassCRDName = "gatewayclasses.gateway.networking.k8s.io"
	gatewayCRDName      = "gateways.gateway.networking.k8s.io"
	httpRouteCRDName    = "httproutes.gateway.networking.k8s.io"

	// Default timeout for mode transitions (may be adjusted for slow architectures)
	defaultModeTransitionTimeout = 3 * time.Minute
)

var _ = g.Describe("[sig-network-edge][OCPFeatureGate:GatewayAPIManagementMode][Feature:Router][apigroup:operator.openshift.io][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		oc                    = exutil.NewCLIWithPodSecurityLevel("gatewayapi-mgmt-mode", admissionapi.LevelBaseline)
		loadBalancerSupported bool
		managedDNS            bool
	)

	g.BeforeEach(func(ctx context.Context) {
		// Feature gate check is handled by [OCPFeatureGate:GatewayAPIManagementMode] label

		// Check platform support and skip conditions
		noOLM, err := isNoOLMFeatureGateEnabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		skip, reason, err := shouldSkipGatewayAPITests(oc, noOLM)
		o.Expect(err).NotTo(o.HaveOccurred())
		if skip {
			g.Skip(reason)
		}

		loadBalancerSupported, managedDNS = getPlatformCapabilities(oc)
	})

	g.It("should have default Managed state with CRDs, VAP, and Istio installed", func(ctx context.Context) {
		g.By("Verifying Ingress CR exists with Managed mode")
		ingress, err := getIngressCR(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get Ingress CR")

		mode := ingress.Spec.GatewayAPI.ManagementMode
		if mode == "" {
			mode = operatorv1alpha1.GatewayAPIManagementModeManaged
		}
		o.Expect(mode).To(o.Equal(operatorv1alpha1.GatewayAPIManagementModeManaged),
			"Expected Ingress CR to have Managed mode by default")

		g.By("Verifying Gateway API CRDs are installed with bundle-version annotation")
		assertGatewayAPICRDsInstalled(ctx, oc)

		g.By("Verifying ValidatingAdmissionPolicy is installed")
		err = assertVAPExists(ctx, oc, gwapiCRDVAPName)
		o.Expect(err).NotTo(o.HaveOccurred(), "VAP %s should exist in Managed mode", gwapiCRDVAPName)

		// Istio installation is driven by the gatewayclass controller
		// reconciling an existing GatewayClass, and is torn down again
		// once the last active GatewayClass is deleted. Since other
		// specs in this suite create and clean up their own
		// GatewayClasses, this test cannot assume Istio is already
		// running when it runs — it must ensure one exists itself.
		g.By("Ensuring a GatewayClass exists to trigger Istio installation")
		gatewayClass := buildGatewayClass("test-default-managed-state", "openshift.io/gateway-controller/v1")
		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(ctx, gatewayClass, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			_ = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(ctx, gatewayClass.Name, metav1.DeleteOptions{})
		})

		err = checkGatewayClassCondition(oc, gatewayClass.Name, string(gatewayapiv1.GatewayClassConditionStatusAccepted), metav1.ConditionTrue)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying Istio control plane is running")
		err = checkIstiodRunning(oc, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred(), "Istiod should be running in Managed mode")

		g.By("Verifying Ingress status conditions")
		err = checkIngressCondition(ctx, oc, "GatewayAPICRDsManaged", metav1.ConditionTrue, "")
		o.Expect(err).NotTo(o.HaveOccurred(), "GatewayAPICRDsManaged should be True")

		err = checkIngressCondition(ctx, oc, "GatewayAPICRDsPresent", metav1.ConditionTrue, "")
		o.Expect(err).NotTo(o.HaveOccurred(), "GatewayAPICRDsPresent should be True")

		err = checkIngressCondition(ctx, oc, "GatewayAPICRDsCompliant", metav1.ConditionTrue, "")
		o.Expect(err).NotTo(o.HaveOccurred(), "GatewayAPICRDsCompliant should be True")

		e2e.Logf("Successfully verified default Managed state")
	})

	g.It("should transition from Managed to Unmanaged and preserve Gateway resources", func(ctx context.Context) {
		// Create Gateway first while in Managed mode
		g.By("Creating GatewayClass and Gateway while in Managed mode")
		gatewayClass := buildGatewayClass("test-unmanaged-transition", "openshift.io/gateway-controller/v1")
		_, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(ctx, gatewayClass, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			_ = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(ctx, gatewayClass.Name, metav1.DeleteOptions{})
		})

		err = checkGatewayClassCondition(oc, gatewayClass.Name, string(gatewayapiv1.GatewayClassConditionStatusAccepted), metav1.ConditionTrue)
		o.Expect(err).NotTo(o.HaveOccurred())

		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-test-unmanaged.", 1)

		testGatewayName := "test-unmanaged-gateway-" + uuid.New().String()[:8]
		_, err = createAndCheckGateway(oc, testGatewayName, gatewayClass.Name, customDomain, loadBalancerSupported)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			_ = oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Delete(ctx, testGatewayName, metav1.DeleteOptions{})
			_ = waitForGatewayDeploymentDeletion(oc, testGatewayName)
		})

		// Restore Managed mode in cleanup
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleanup: Restoring Managed mode")
			err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("Transitioning to Unmanaged mode")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for transition to complete")
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying Managed condition is False with reason Unmanaged")
		err = checkIngressCondition(ctx, oc, "GatewayAPICRDsManaged", metav1.ConditionFalse, "Unmanaged")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying VAP is deleted")
		err = assertVAPDeleted(ctx, oc, gwapiCRDVAPName)
		o.Expect(err).NotTo(o.HaveOccurred(), "VAP should be deleted in Unmanaged mode")

		g.By("Verifying Istio control plane is stopped")
		waitForIstiodPodDeletion(oc)

		g.By("Verifying Gateway API CRDs are still present")
		assertGatewayAPICRDsInstalled(ctx, oc)

		g.By("Verifying Gateway resource still exists")
		_, err = oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Get(ctx, testGatewayName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Gateway should still exist in Unmanaged mode")

		g.By("Verifying GatewayClass still exists")
		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Get(ctx, gatewayClass.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "GatewayClass should still exist in Unmanaged mode")

		// Verify we can modify a Gateway API CRD without VAP protection
		g.By("Verifying CRDs can be modified without VAP protection")
		crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, httpRouteCRDName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		originalSpec := crd.Spec.DeepCopy()

		if crd.Annotations == nil {
			crd.Annotations = make(map[string]string)
		}
		crd.Annotations["test.openshift.io/unmanaged"] = "true"
		crd, err = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to modify CRD in Unmanaged mode")

		// Verify only annotations changed, not spec
		o.Expect(crd.Spec).To(o.Equal(*originalSpec), "CRD Spec should not be modified, only annotations")

		// Clean up the test annotation
		delete(crd.Annotations, "test.openshift.io/unmanaged")
		_, _ = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})

		e2e.Logf("Successfully transitioned to Unmanaged mode and verified Gateway resources preserved")
	})

	g.It("should transition from Unmanaged to Managed and deploy working Gateway with real workload", func(ctx context.Context) {
		// Transition to Unmanaged first
		g.By("Transitioning to Unmanaged mode")
		err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Restore Managed mode in cleanup
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleanup: Ensuring Managed mode for subsequent tests")
			err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("Transitioning back to Managed mode")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for transition to complete")
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, 5*time.Minute))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying VAP is recreated")
		err = assertVAPExists(ctx, oc, gwapiCRDVAPName)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Istio installation is driven by the gatewayclass controller
		// reconciling an existing GatewayClass; it is not triggered
		// proactively just by the mode transition. A GatewayClass must
		// exist before checking that Istio has (re)started.
		g.By("Creating GatewayClass and Gateway in Managed mode")
		gatewayClass := buildGatewayClass("test-managed-workload", "openshift.io/gateway-controller/v1")
		_, err = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(ctx, gatewayClass, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			_ = oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(ctx, gatewayClass.Name, metav1.DeleteOptions{})
		})

		err = checkGatewayClassCondition(oc, gatewayClass.Name, string(gatewayapiv1.GatewayClassConditionStatusAccepted), metav1.ConditionTrue)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying Istio control plane is restarted")
		err = checkIstiodRunning(oc, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		defaultIngressDomain, err := getDefaultIngressClusterDomainName(oc, 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		customDomain := strings.Replace(defaultIngressDomain, "apps.", "gw-managed-workload.", 1)

		testGatewayName := "test-managed-workload-gateway-" + uuid.New().String()[:8]
		_, err = createAndCheckGateway(oc, testGatewayName, gatewayClass.Name, customDomain, loadBalancerSupported)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			_ = oc.AdminGatewayApiClient().GatewayV1().Gateways(ingressNamespace).Delete(ctx, testGatewayName, metav1.DeleteOptions{})
			_ = waitForGatewayDeploymentDeletion(oc, testGatewayName)
		})

		var lbAddress string
		if loadBalancerSupported {
			g.By("Verifying LoadBalancer service is created")
			lbAddress = assertGatewayLoadbalancerReady(oc, testGatewayName, testGatewayName+"-"+gatewayClass.Name)
		}

		if managedDNS {
			g.By("Verifying DNS controller creates DNSRecord")
			assertDNSRecordStatus(oc, testGatewayName)
		}

		g.By("Creating HTTPRoute with backend pod")
		// Randomize the hostname (not just the Gateway/backend names) so a
		// retry against the same cluster never reuses a public DNS name
		// that a prior attempt's create/delete cycle may have left
		// negatively cached by an intermediate resolver.
		hostname := "test-workload-" + uuid.New().String()[:8] + "." + customDomain
		routeName := "test-workload-route"
		backendName := "echo-backend-" + testGatewayName
		createHttpRoute(oc, testGatewayName, routeName, hostname, backendName)
		g.DeferCleanup(func(ctx context.Context) {
			_ = oc.AdminGatewayApiClient().GatewayV1().HTTPRoutes(oc.Namespace()).Delete(ctx, routeName, metav1.DeleteOptions{})
		})

		g.By("Waiting for HTTPRoute to be accepted")
		_, err = assertHttpRouteSuccessful(oc, testGatewayName, routeName)
		o.Expect(err).NotTo(o.HaveOccurred())

		if loadBalancerSupported && managedDNS {
			g.By("Verifying HTTP connectivity works end-to-end")
			// Connect directly to the load balancer address with the
			// route's hostname as the Host header, the same technique
			// classic Route reachability checks use. This avoids
			// depending on public DNS propagation of the route's own
			// hostname, which can lag well behind the load balancer's
			// own (already-resolvable) address.
			assertHttpRouteConnectionViaAddress(lbAddress, hostname)
		}

		e2e.Logf("Successfully transitioned to Managed mode and deployed working Gateway with real workload")
	})

	g.It("should block takeover when non-compliant CRDs exist - bundle-version mismatch", func(ctx context.Context) {
		// Ensure we start in Managed mode, then go to Unmanaged
		g.By("Ensuring Managed mode first")
		err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Transitioning to Unmanaged mode")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Cleanup: restore Managed mode
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleanup: Restoring Managed mode")
			// First, restore CRD compliance by deleting if needed
			crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, httpRouteCRDName, metav1.GetOptions{})
			if err == nil {
				if bundleVer := crd.Annotations[bundleVersionAnnotation]; bundleVer == "v0.0.0-takeover-test" {
					e2e.Logf("Cleanup: Deleting non-compliant CRD")
					_ = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, httpRouteCRDName, metav1.DeleteOptions{})

					// Wait for CRD to be recreated
					o.Eventually(func() bool {
						crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, httpRouteCRDName, metav1.GetOptions{})
						if err != nil {
							return false
						}
						bundleVersion, found := crd.Annotations[bundleVersionAnnotation]
						return found && bundleVersion != "v0.0.0-takeover-test"
					}).WithTimeout(platformAwareTimeout(oc, 3*time.Minute)).WithPolling(5 * time.Second).Should(o.BeTrue())
				}
			}

			err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("Modifying Gateway API CRD bundle-version to make it non-compliant")
		var originalBundleVersion string
		o.Eventually(func() error {
			crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, httpRouteCRDName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			originalBundleVersion = crd.Annotations[bundleVersionAnnotation]
			if originalBundleVersion == "" {
				return fmt.Errorf("CRD missing bundle-version annotation")
			}

			crd.Annotations[bundleVersionAnnotation] = "v0.0.0-takeover-test"
			_, err = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
			return err
		}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).Should(o.Succeed())

		e2e.Logf("Modified CRD bundle-version from %s to v0.0.0-takeover-test", originalBundleVersion)

		g.By("Attempting to switch to Managed mode (should be blocked)")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying takeover is blocked (Managed=False, reason=TakeoverBlocked)")
		o.Eventually(func() error {
			return checkIngressCondition(ctx, oc, "GatewayAPICRDsManaged", metav1.ConditionFalse, "TakeoverBlocked")
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		err = checkIngressCondition(ctx, oc, "GatewayAPICRDsCompliant", metav1.ConditionFalse, "")
		o.Expect(err).NotTo(o.HaveOccurred(), "Compliant condition should be False")

		g.By("Deleting non-compliant CRD to allow CIO to recreate it")
		err = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, httpRouteCRDName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for CIO to recreate compliant CRD")
		o.Eventually(func() bool {
			crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, httpRouteCRDName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			bundleVersion, found := crd.Annotations[bundleVersionAnnotation]
			return found && bundleVersion != "v0.0.0-takeover-test"
		}).WithTimeout(platformAwareTimeout(oc, 3*time.Minute)).WithPolling(5 * time.Second).Should(o.BeTrue())

		g.By("Verifying takeover succeeds after restoring compliance")
		o.Eventually(func() error {
			return checkIngressCondition(ctx, oc, "GatewayAPICRDsManaged", metav1.ConditionTrue, "")
		}).WithTimeout(platformAwareTimeout(oc, 3*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		// Compliant is computed from a fresh CRD read in the same
		// reconcile that recreates the CRD; the informer cache backing
		// that read can lag the just-created object by one reconcile, so
		// this must poll rather than check once.
		o.Eventually(func() error {
			return checkIngressCondition(ctx, oc, "GatewayAPICRDsCompliant", metav1.ConditionTrue, "")
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		g.By("Verifying VAP is recreated")
		err = assertVAPExists(ctx, oc, gwapiCRDVAPName)
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Successfully blocked takeover with non-compliant CRD and recovered after fixing compliance")
	})

	g.It("should block takeover when unknown Gateway API CRDs exist", func(ctx context.Context) {
		// Ensure we start in Managed mode, then go to Unmanaged
		g.By("Ensuring Managed mode first")
		err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Transitioning to Unmanaged mode")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Cleanup: restore Managed mode. Registered immediately after the
		// Unmanaged transition succeeds, before any step that can fail, so
		// the cluster is never left stuck in Unmanaged mode for subsequent
		// tests if a later step in this test fails.
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleanup: Restoring Managed mode")
			err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// Create a mock unknown Gateway API CRD. The name must be
		// spec.names.plural+"."+spec.group, and since the group is a
		// protected "*.k8s.io" group, the apiserver also requires the
		// api-approved.kubernetes.io annotation to accept the CRD.
		mockCRDName := "invalids.gateway.networking.k8s.io"
		g.By(fmt.Sprintf("Creating mock unknown Gateway API CRD: %s", mockCRDName))
		mockCRD := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: mockCRDName,
				Annotations: map[string]string{
					bundleVersionAnnotation:      "v99.99.99-unknown",
					"api-approved.kubernetes.io": "https://github.com/kubernetes/enhancements/pull/1111",
				},
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "gateway.networking.k8s.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "invalids",
					Singular: "invalid",
					Kind:     "Invalid",
					ListKind: "InvalidList",
				},
				Scope: apiextensionsv1.ClusterScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type:                   "object",
										XPreserveUnknownFields: new(true),
									},
								},
							},
						},
					},
				},
			},
		}

		_, err = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Create(ctx, mockCRD, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleanup: Deleting mock CRD %s", mockCRDName)
			_ = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, mockCRDName, metav1.DeleteOptions{})
		})

		g.By("Attempting to switch to Managed mode (should be blocked)")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying takeover is blocked due to unknown CRD")
		o.Eventually(func() error {
			return checkIngressCondition(ctx, oc, "GatewayAPICRDsManaged", metav1.ConditionFalse, "TakeoverBlocked")
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		g.By("Deleting mock unknown CRD")
		err = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, mockCRDName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying takeover succeeds after removing unknown CRD")
		o.Eventually(func() error {
			return checkIngressCondition(ctx, oc, "GatewayAPICRDsManaged", metav1.ConditionTrue, "")
		}).WithTimeout(platformAwareTimeout(oc, 3*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		o.Eventually(func() error {
			return checkIngressCondition(ctx, oc, "GatewayAPICRDsCompliant", metav1.ConditionTrue, "")
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		e2e.Logf("Successfully blocked takeover with unknown Gateway API CRD and recovered after deletion")
	})

	g.It("should report correct metrics for management mode", func(ctx context.Context) {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("ingress operator metrics are not available in hosted cluster Prometheus on External/HyperShift topology")
		}
		g.By("Ensuring Managed mode")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating Prometheus client")
		prometheusClient, err := promclient.NewE2EPrometheusRouterClient(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying management_mode metric shows Managed=1 and Unmanaged=0")
		o.Eventually(func() error {
			result, _, err := prometheusClient.Query(ctx, `ingress_controller_gateway_api_management_mode{mode="Managed"}`, time.Now())
			if err != nil {
				return err
			}
			vector, ok := result.(model.Vector)
			if !ok || len(vector) == 0 {
				return fmt.Errorf("metric not found")
			}
			for _, sample := range vector {
				if float64(sample.Value) != 1 {
					return fmt.Errorf("expected Managed=1, got %v", sample.Value)
				}
			}
			return nil
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		o.Eventually(func() error {
			result, _, err := prometheusClient.Query(ctx, `ingress_controller_gateway_api_management_mode{mode="Unmanaged"}`, time.Now())
			if err != nil {
				return err
			}
			vector, ok := result.(model.Vector)
			if !ok || len(vector) == 0 {
				return fmt.Errorf("metric not found")
			}
			for _, sample := range vector {
				if float64(sample.Value) != 0 {
					return fmt.Errorf("expected Unmanaged=0, got %v", sample.Value)
				}
			}
			return nil
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		// Transition to Unmanaged and verify metrics change
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleanup: Restoring Managed mode")
			err := setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeManaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("Transitioning to Unmanaged mode")
		err = setManagementMode(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForManagementModeTransition(ctx, oc, operatorv1alpha1.GatewayAPIManagementModeUnmanaged, platformAwareTimeout(oc, defaultModeTransitionTimeout))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying metrics show Unmanaged=1 and Managed=0 after transition")
		o.Eventually(func() error {
			result, _, err := prometheusClient.Query(ctx, `ingress_controller_gateway_api_management_mode{mode="Unmanaged"}`, time.Now())
			if err != nil {
				return err
			}
			vector, ok := result.(model.Vector)
			if !ok || len(vector) == 0 {
				return fmt.Errorf("metric not found")
			}
			if float64(vector[0].Value) != 1 {
				return fmt.Errorf("expected Unmanaged=1, got %v", vector[0].Value)
			}
			return nil
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		o.Eventually(func() error {
			result, _, err := prometheusClient.Query(ctx, `ingress_controller_gateway_api_management_mode{mode="Managed"}`, time.Now())
			if err != nil {
				return err
			}
			vector, ok := result.(model.Vector)
			if !ok || len(vector) == 0 {
				return fmt.Errorf("metric not found")
			}
			if float64(vector[0].Value) != 0 {
				return fmt.Errorf("expected Managed=0, got %v", vector[0].Value)
			}
			return nil
		}).WithTimeout(platformAwareTimeout(oc, 2*time.Minute)).WithPolling(5 * time.Second).Should(o.Succeed())

		e2e.Logf("Successfully verified management mode metrics")
	})
})

// Helper functions

func getIngressCR(ctx context.Context, oc *exutil.CLI) (*operatorv1alpha1.Ingress, error) {
	ingressClient := oc.AdminOperatorClient().OperatorV1alpha1().Ingresses()
	return ingressClient.Get(ctx, ingressCRName, metav1.GetOptions{})
}

func setManagementMode(ctx context.Context, oc *exutil.CLI, mode operatorv1alpha1.GatewayAPIManagementMode) error {
	ingressClient := oc.AdminOperatorClient().OperatorV1alpha1().Ingresses()

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		ingress, err := ingressClient.Get(ctx, ingressCRName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		ingress.Spec.GatewayAPI.ManagementMode = mode

		_, err = ingressClient.Update(ctx, ingress, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

func waitForManagementModeTransition(ctx context.Context, oc *exutil.CLI, expectedMode operatorv1alpha1.GatewayAPIManagementMode, timeout time.Duration) error {
	ingressClient := oc.AdminOperatorClient().OperatorV1alpha1().Ingresses()

	var expectedConditionStatus metav1.ConditionStatus
	var expectedReason string

	if expectedMode == operatorv1alpha1.GatewayAPIManagementModeManaged {
		expectedConditionStatus = metav1.ConditionTrue
		expectedReason = "" // Any reason is acceptable for True
	} else {
		expectedConditionStatus = metav1.ConditionFalse
		expectedReason = "Unmanaged"
	}

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ingress, err := ingressClient.Get(ctx, ingressCRName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		managedCond := condutils.FindStatusCondition(ingress.Status.Conditions, "GatewayAPICRDsManaged")
		if managedCond == nil {
			return false, nil
		}

		if managedCond.Status != expectedConditionStatus {
			return false, nil
		}

		if expectedReason != "" && managedCond.Reason != expectedReason {
			return false, nil
		}

		return true, nil
	})
}

func checkIngressCondition(ctx context.Context, oc *exutil.CLI, conditionType string, expectedStatus metav1.ConditionStatus, expectedReason string) error {
	ingress, err := getIngressCR(ctx, oc)
	if err != nil {
		return err
	}

	cond := condutils.FindStatusCondition(ingress.Status.Conditions, conditionType)
	if cond == nil {
		return fmt.Errorf("condition %s not found", conditionType)
	}

	if cond.Status != expectedStatus {
		return fmt.Errorf("condition %s has status %v, expected %v", conditionType, cond.Status, expectedStatus)
	}

	if expectedReason != "" && cond.Reason != expectedReason {
		return fmt.Errorf("condition %s has reason %s, expected %s", conditionType, cond.Reason, expectedReason)
	}

	return nil
}

func assertVAPExists(ctx context.Context, oc *exutil.CLI, vapName string) error {
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, platformAwareTimeout(oc, 2*time.Minute), true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			lastErr = err
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		if lastErr != nil {
			return fmt.Errorf("VAP %s not found: %w", vapName, lastErr)
		}
		return fmt.Errorf("VAP %s not found: %w", vapName, err)
	}
	return nil
}

func assertVAPDeleted(ctx context.Context, oc *exutil.CLI, vapName string) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, platformAwareTimeout(oc, 2*time.Minute), true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

func assertGatewayAPICRDsInstalled(ctx context.Context, oc *exutil.CLI) {
	expectedCRDs := []string{
		gatewayClassCRDName,
		gatewayCRDName,
		httpRouteCRDName,
	}

	for _, crdName := range expectedCRDs {
		crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "CRD %s should be installed", crdName)

		bundleVersion, found := crd.Annotations[bundleVersionAnnotation]
		o.Expect(found).To(o.BeTrue(), "CRD %s should have bundle-version annotation", crdName)
		o.Expect(bundleVersion).NotTo(o.BeEmpty(), "CRD %s bundle-version should not be empty", crdName)
	}
}

// platformAwareTimeout adjusts timeout for slow architectures
func platformAwareTimeout(oc *exutil.CLI, baseTimeout time.Duration) time.Duration {
	platformType, err := platformidentification.GetJobType(context.Background(), oc.AdminConfig())
	if err != nil {
		return baseTimeout
	}

	// Double timeout on slow IBM architectures (Power and Z)
	if platformType.Architecture == platformidentification.ArchitecturePPC64le ||
		platformType.Architecture == platformidentification.ArchitectureS390 {
		return baseTimeout * 2
	}

	return baseTimeout
}
