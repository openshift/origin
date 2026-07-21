package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-network-edge][Feature:Router][apigroup:route.openshift.io][OCPFeatureGate:IngressControllerMultipleHAProxyVersions]", func() {
	defer g.GinkgoRecover()

	// testsTimeout defines the maximum amount of time to wait for test operations to complete.
	const testsTimeout = 5 * time.Minute

	// defaultHAProxyVersion is the default HAProxy version for the current release.
	var defaultHAProxyVersion operatorv1.HAProxyVersion

	//alternateHAProxyVersion is the other accepted HAProxyVersion accepted in the current release
	var alternateHAProxyVersion operatorv1.HAProxyVersion

	// controllers is used to create new ingress controllers, and stores their reference so they can be removed after the test runs
	var controllers ingressControllers

	ctx := context.Background()
	oc := exutil.NewCLI("router-select-haproxy-version").AsAdmin()
	operatorClient := oc.AdminOperatorClient()

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			for _, ic := range controllers.items {
				exutil.DumpPodLogsStartingWithInNamespace(ic.controller.Name, ic.controller.Namespace, oc)
			}
		}
		var errs []error
		for _, ic := range controllers.items {
			err := operatorClient.OperatorV1().IngressControllers(ic.controller.Namespace).Delete(ctx, ic.controller.Name, *metav1.NewDeleteOptions(1))
			errs = append(errs, client.IgnoreNotFound(err))
		}
		o.Expect(errors.Join(errs...)).NotTo(o.HaveOccurred())
		controllers.items = nil
	})

	g.BeforeEach(func() {
		defaultIC, err := operatorClient.OperatorV1().IngressControllers("openshift-ingress-operator").Get(ctx, "default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(defaultIC.Status.EffectiveHAProxyVersion).NotTo(o.BeEmpty())
		defaultHAProxyVersion = defaultIC.Status.EffectiveHAProxyVersion

		if defaultHAProxyVersion == operatorv1.HAProxyVersion28 {
			alternateHAProxyVersion = operatorv1.HAProxyVersion32
		} else {
			alternateHAProxyVersion = operatorv1.HAProxyVersion28
		}
	})

	g.Describe("The HAProxy router with version selection", func() {

		// Ensure that the haproxyVersion field in the IngressController API does not accept unknown versions
		g.It("should reject invalid HAProxy versions", func() {
			versions := []operatorv1.HAProxyVersion{
				" ",                          // Empty becomes unset, but space is invalid
				"2.6",                        // one LTS before the oldest supported version, so always invalid
				"v" + defaultHAProxyVersion,  // v prefix is invalid
				defaultHAProxyVersion + ".0", // .z suffix is invalid, only x.y is supported
				" " + defaultHAProxyVersion,  // leading space is invalid
				defaultHAProxyVersion + " ",  // trailing space is invalid
			}
			for _, version := range versions {
				_, err := controllers.createIngressController(ctx, oc, testsTimeout, func(ic *operatorv1.IngressController) {
					ic.Spec.HAProxyVersion = version
				})
				o.Expect(err).To(o.Not(o.Succeed()))
				o.Expect(err.Error()).To(o.ContainSubstring(`Unsupported value: "%s": supported values: `, version))
			}
		})

		// Ensure that the ingress controller reverts back to the default version after unsetting the field with null
		g.It("should revert to default HAProxy version when field is cleared", func() {
			ingress, err := controllers.createIngressController(ctx, oc, testsTimeout, func(ic *operatorv1.IngressController) {
				ic.Spec.HAProxyVersion = alternateHAProxyVersion
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("confirm the HAProxy version matches the controller version")
			err = waitForHAProxyVersion(ctx, oc, ingress.Name, alternateHAProxyVersion)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Unset the version on the custom ingress controller")
			patch := []byte(`{"spec":{"haproxyVersion":null}}`)
			_, err = operatorClient.OperatorV1().IngressControllers(ingress.Namespace).Patch(ctx, ingress.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Confirm that the HAProxy version shows default version")
			err = waitForHAProxyVersion(ctx, oc, ingress.Name, defaultHAProxyVersion)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// Ensure that the running HAProxy version matches the version configured in the IngressController API
		g.It("should configure the same HAProxy version defined in the API", func() {
			versions := []operatorv1.HAProxyVersion{
				defaultHAProxyVersion,
				alternateHAProxyVersion,
			}
			for _, version := range versions {
				ingress, err := controllers.createIngressController(ctx, oc, testsTimeout, func(ic *operatorv1.IngressController) {
					ic.Spec.HAProxyVersion = version
				})
				o.Expect(err).To(o.Succeed())
				errPoll := wait.PollUntilContextTimeout(ctx, 2*time.Second, testsTimeout, true, func(ctx context.Context) (bool, error) {
					ic, err := operatorClient.OperatorV1().IngressControllers(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
					if err != nil {
						e2e.Logf("Failed to get the IngressController %s", ingress.Name)
						return false, nil
					}
					if ic.Status.EffectiveHAProxyVersion == version {
						e2e.Logf("EffectiveHAProxyVersion shows the expected version: %q", version)
						return true, nil
					}
					e2e.Logf("EffectiveHAProxyVersion: %q does not match the expected version %q", ic.Status.EffectiveHAProxyVersion, version)
					return false, nil
				})
				o.Expect(errPoll).NotTo(o.HaveOccurred(), "Timed out waiting for EffectiveHAProxyVersion")
				e2e.Logf("IngressController: %s matches the expected HAProxyVersion: %s", ingress.Name, string(version))
			}
		})

		// Ensure that if the version is unset, its value in the IngressController API remains undeclared, and the running HAProxy matches the default version
		g.It("should configure the default HAProxy if the version is unset", func() {
			// create a custom ingress controller with an unset HAProxyVersion
			ingress, err := controllers.createIngressController(ctx, oc, testsTimeout, nil)
			o.Expect(err).To(o.Succeed())

			//confirm that the ingresscontroller is unset
			o.Expect(ingress.Spec.HAProxyVersion).To(o.BeEmpty())

			var effectiveVersion operatorv1.HAProxyVersion
			err = wait.PollUntilContextTimeout(ctx, 2*time.Second, testsTimeout, true, func(ctx context.Context) (bool, error) {
				ic, err := operatorClient.OperatorV1().IngressControllers(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Failed to get the IngressController %s", ingress.Name)
					return false, nil
				}
				if ic.Status.EffectiveHAProxyVersion == "" {
					e2e.Logf("IngressController %s: EffectiveHAProxyVersion not yet set, waiting...", ingress.Name)
					return false, nil
				}
				effectiveVersion = ic.Status.EffectiveHAProxyVersion
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for EffectiveHAProxyVersion")
			o.Expect(effectiveVersion).To(o.Equal(defaultHAProxyVersion))
			e2e.Logf("IngressController: %s has the expected HAProxyVersion: %s", ingress.Name, string(defaultHAProxyVersion))
			err = waitForHAProxyVersion(ctx, oc, ingress.Name, defaultHAProxyVersion)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// Ensure that changing the HAProxy version of a custom IngressController does not affect the running HAProxy of the default router
		g.It("should maintain default router unchanged if updating the version on a custom IngressController", func() {
			g.By("Grab the default ingresscontroller HAProxyVersion field")
			ingressDefault, err := operatorClient.OperatorV1().IngressControllers("openshift-ingress-operator").Get(context.Background(), "default", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingressVersion := ingressDefault.Status.EffectiveHAProxyVersion

			g.By("Create a custom controller and patch it to an older version")
			ingress, err := controllers.createIngressController(ctx, oc, testsTimeout, nil)
			o.Expect(err).To(o.Succeed())

			patch := []byte(fmt.Sprintf(`{"spec":{"haproxyVersion":"%s"}}`, alternateHAProxyVersion))
			_, err = operatorClient.OperatorV1().IngressControllers(ingress.Namespace).Patch(ctx, ingress.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Confirm the HaProxy version matches")
			err = waitForHAProxyVersion(ctx, oc, ingress.Name, alternateHAProxyVersion)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Confirm the default router is still running the default HAProxy version")
			err = waitForHAProxyVersion(ctx, oc, ingressDefault.Name, ingressVersion)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

	})
})

type ingressControllers struct {
	items []*ingressController
}

type ingressController struct {
	controller types.NamespacedName
}

func (i *ingressControllers) createIngressController(ctx context.Context, oc *exutil.CLI, readyTimeout time.Duration, custom func(ic *operatorv1.IngressController)) (*operatorv1.IngressController, error) {
	operatorClient := oc.AdminOperatorClient()

	// ingress controller need to be created in operator's namespace, ...
	nsOperator := "openshift-ingress-operator"
	controllerName := names.SimpleNameGenerator.GenerateName("e2e-haproxy-version-")

	routeSelectorSet := labels.Set{"select": names.SimpleNameGenerator.GenerateName("haproxy-cfgmgr-")}

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
			NamespaceSelector: metav1.SetAsLabelSelector(labels.Set{corev1.LabelMetadataName: oc.Namespace()}),
			RouteSelector:     metav1.SetAsLabelSelector(routeSelectorSet),
			HAProxyVersion:    "",
		},
	}
	if custom != nil {
		custom(&ic)
	}
	ingress, err := operatorClient.OperatorV1().IngressControllers(nsOperator).Create(ctx, &ic, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	controller := types.NamespacedName{
		Namespace: nsOperator,
		Name:      controllerName,
	}
	ictr := ingressController{
		controller: controller,
	}
	i.items = append(i.items, &ictr)

	ingressControllerReady := []operatorv1.OperatorCondition{
		{Type: operatorv1.IngressControllerAvailableConditionType, Status: operatorv1.ConditionTrue},
		{Type: operatorv1.LoadBalancerManagedIngressConditionType, Status: operatorv1.ConditionFalse},
		{Type: operatorv1.DNSManagedIngressConditionType, Status: operatorv1.ConditionFalse},
		{Type: operatorv1.OperatorStatusTypeProgressing, Status: operatorv1.ConditionFalse},
	}

	// wait for the controller to be available
	err = shard.WaitForIngressControllerCondition(oc, readyTimeout, controller, ingressControllerReady...)
	if err != nil {
		return nil, err
	}

	return ingress, nil
}

// poll the router pods HAProxy Container to check that the version is correctly asserted
func waitForHAProxyVersion(ctx context.Context, oc *exutil.CLI, ingressName string, desiredVersion operatorv1.HAProxyVersion) error {
	if desiredVersion == "" {
		return fmt.Errorf("desiredVersion must not be empty")
	}
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, false, func(ctx context.Context) (bool, error) {
		haproxy, err := oc.Run("exec").Args("-n", "openshift-ingress", "-c", "haproxy", "deploy/router-"+ingressName, "--", "bash", "-c", "echo 'show version' | socat - /var/lib/haproxy/run/haproxy.sock").Output()
		if err != nil {
			e2e.Logf("Failed to extract the HAProxy Version from IngressController %s", ingressName)
			return false, nil
		}
		if !strings.HasPrefix(haproxy, string(desiredVersion)) {
			e2e.Logf("HAProxy version mismatch: got %q, waiting for %q", haproxy, desiredVersion)
			return false, nil
		}
		e2e.Logf("IngressController: %s HAProxy has the correct version: %s", ingressName, haproxy)
		return true, nil
	})
	return err
}
