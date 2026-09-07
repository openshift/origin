package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"

	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network-edge][Feature:Router][apigroup:route.openshift.io][OCPFeatureGate:IngressControllerMultipleHAProxyVersions]", func() {
	defer g.GinkgoRecover()

	// testsTimeout defines the maximum amount of time to wait for test operations to complete.
	const testsTimeout = 5 * time.Minute

	// versionConfig is the HAProxy version configuration in the current release.
	var versionConfig haproxyVersionConfig

	// alternateHAProxyVersion is one of the non default accepted HAProxyVersions in the current release.
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
		err := controllers.deleteAll(ctx, operatorClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		controllers.items = nil
	})

	g.BeforeEach(func() {
		hasField, err := apiHasHAProxyVersionField(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !hasField {
			g.Skip("IngressController CRD does not have haproxyVersion field — operator not yet updated")
		}

		versions, err := getHAProxyVersionConfig(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		nonDefaultVersions := versions.getNonDefaultVersions()
		if len(nonDefaultVersions) == 0 {
			g.Skip("IngressController has no non default versions available")
		}

		// update shared vars
		versionConfig = versions
		alternateHAProxyVersion = nonDefaultVersions[0]
	})

	g.Describe("The HAProxy router with version selection", func() {

		// Ensure that the haproxyVersion field in the IngressController API does not accept unknown versions
		g.It("should reject invalid HAProxy versions", func() {
			versions := []operatorv1.HAProxyVersion{
				" ",                                 // Empty becomes unset, but space is invalid
				"2.6",                               // one LTS before the oldest supported version, so always invalid
				"v" + versionConfig.defaultVersion,  // v prefix is invalid
				versionConfig.defaultVersion + ".0", // .z suffix is invalid, only x.y is supported
				" " + versionConfig.defaultVersion,  // leading space is invalid
				versionConfig.defaultVersion + " ",  // trailing space is invalid
			}
			for _, version := range versions {
				_, err := controllers.createIngressController(ctx, oc, func(ic *operatorv1.IngressController) {
					ic.Spec.HAProxyVersion = version
				})
				o.Expect(err).To(o.Not(o.Succeed()))
				o.Expect(err.Error()).To(o.ContainSubstring(`Unsupported value: "%s": supported values: `, version))
			}
		})

		// Ensure that the ingress controller reverts back to the default version after unsetting the field with null
		g.It("should revert to default HAProxy version when field is cleared", func() {
			ingress, err := controllers.createIngressController(ctx, oc, func(ic *operatorv1.IngressController) {
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
			err = waitForHAProxyVersion(ctx, oc, ingress.Name, versionConfig.defaultVersion)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// Ensure that the running HAProxy version matches the version configured in the IngressController API
		g.It("should configure the same HAProxy version defined in the API", func() {
			for _, version := range versionConfig.availableVersions {
				ingress, err := controllers.createIngressController(ctx, oc, func(ic *operatorv1.IngressController) {
					ic.Spec.HAProxyVersion = version
				})
				o.Expect(err).To(o.Succeed())
				errPoll := waitForEffectiveHAProxyVersion(ctx, operatorClient, types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}, version, testsTimeout)
				o.Expect(errPoll).NotTo(o.HaveOccurred(), "Timed out waiting for EffectiveHAProxyVersion")
				err = waitForHAProxyVersion(ctx, oc, ingress.Name, version)
				o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy version from runtime API")
				e2e.Logf("IngressController: %s matches the expected HAProxyVersion: %s", ingress.Name, string(version))
			}
		})

		// Ensure that if the version is unset, its value in the IngressController API remains undeclared, and the running HAProxy matches the default version
		g.It("should configure the default HAProxy if the version is unset", func() {
			// create a custom ingress controller with an unset HAProxyVersion
			ingress, err := controllers.createIngressController(ctx, oc, nil)
			o.Expect(err).To(o.Succeed())

			//confirm that the ingresscontroller is unset
			o.Expect(ingress.Spec.HAProxyVersion).To(o.BeEmpty())

			err = waitForEffectiveHAProxyVersion(ctx, operatorClient, types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}, versionConfig.defaultVersion, testsTimeout)
			o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for EffectiveHAProxyVersion")
			e2e.Logf("IngressController: %s has the expected HAProxyVersion: %s", ingress.Name, string(versionConfig.defaultVersion))
			err = waitForHAProxyVersion(ctx, oc, ingress.Name, versionConfig.defaultVersion)
			o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy version from runtime API")
		})

		// Ensure that changing the HAProxy version of a custom IngressController does not affect the running HAProxy of the default router
		g.It("should maintain default router unchanged if updating the version on a custom IngressController", func() {
			g.By("Grab the default ingresscontroller HAProxyVersion field")
			ingressDefault, err := operatorClient.OperatorV1().IngressControllers("openshift-ingress-operator").Get(context.Background(), "default", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingressVersion := ingressDefault.Status.EffectiveHAProxyVersion

			g.By("Create a custom controller and patch it to an older version")
			ingress, err := controllers.createIngressController(ctx, oc, nil)
			o.Expect(err).To(o.Succeed())

			patch := []byte(fmt.Sprintf(`{"spec":{"haproxyVersion":"%s"}}`, alternateHAProxyVersion))
			_, err = operatorClient.OperatorV1().IngressControllers(ingress.Namespace).Patch(ctx, ingress.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Confirm the HAProxy version matches")
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

func (i *ingressControllers) createIngressController(ctx context.Context, oc *exutil.CLI, custom func(ic *operatorv1.IngressController)) (*operatorv1.IngressController, error) {
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

	return ingress, waitForIngressControllerReady(ctx, oc, controller)
}

func (i *ingressControllers) deleteAll(ctx context.Context, operatorClient operatorv1client.Interface) error {
	errs := make([]error, len(i.items))
	wg := sync.WaitGroup{}
	for idx, ic := range i.items {
		wg.Go(func() {
			if err := deleteIngressControllerAndWait(ctx, operatorClient, ic.controller); err != nil {
				errs[idx] = fmt.Errorf("error during IngressController %s deletion: %w", ic.controller.String(), err)
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// waitForIngressControllerReady waits for the provided IngressController to be ready.
func waitForIngressControllerReady(ctx context.Context, oc *exutil.CLI, ic types.NamespacedName) error {
	ingressControllerReady := []operatorv1.OperatorCondition{
		{Type: operatorv1.IngressControllerAvailableConditionType, Status: operatorv1.ConditionTrue},
		{Type: operatorv1.LoadBalancerManagedIngressConditionType, Status: operatorv1.ConditionFalse},
		{Type: operatorv1.DNSManagedIngressConditionType, Status: operatorv1.ConditionFalse},
		{Type: operatorv1.OperatorStatusTypeProgressing, Status: operatorv1.ConditionFalse},
	}
	return shard.WaitForIngressControllerCondition(ctx, oc, 5*time.Minute, ic, ingressControllerReady...)
}

// waitForIngressControllerDeletion waits for an IngressController to be removed.
func waitForIngressControllerDeletion(ctx context.Context, operatorClient operatorv1client.Interface, ic types.NamespacedName) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, false, func(ctx context.Context) (done bool, err error) {
		_, err = operatorClient.OperatorV1().IngressControllers(ic.Namespace).Get(ctx, ic.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			e2e.Logf("IngressController %s has been deleted", ic.String())
			return true, nil
		}
		if err != nil {
			e2e.Logf("error reading IngressController %s: %s", ic.String(), err.Error())
		} else {
			e2e.Logf("waiting IngressController %s to be deleted", ic.String())
		}
		return false, nil
	})
}

// deleteIngressControllerAndWait deletes an IngressController and waits for it to be removed.
func deleteIngressControllerAndWait(ctx context.Context, operatorClient operatorv1client.Interface, ic types.NamespacedName) error {
	e2e.Logf("Deleting IngressController %s", ic.String())
	err := operatorClient.OperatorV1().IngressControllers(ic.Namespace).Delete(ctx, ic.Name, *metav1.NewDeleteOptions(1))
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("error deleting IngressController %s: %w", ic.String(), err)
	}
	if err := waitForIngressControllerDeletion(ctx, operatorClient, ic); err != nil {
		return fmt.Errorf("IngressController %s was not deleted: %w", ic.String(), err)
	}
	return nil
}

// poll the router pods HAProxy Container to check that the version is correctly asserted
func waitForHAProxyVersion(ctx context.Context, oc *exutil.CLI, ingressName string, desiredVersion operatorv1.HAProxyVersion) error {
	if desiredVersion == "" {
		return fmt.Errorf("desiredVersion must not be empty")
	}
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, false, func(ctx context.Context) (bool, error) {
		// running in the `router` container - the only one for 4.22 and earlier used on upgrade tests,
		// and a valid one for 4.23/5.0 and newer since it also has socat and the haproxy socket.
		haproxy, err := oc.Run("exec").Args("-n", "openshift-ingress", "-c", "router", "deploy/router-"+ingressName, "--", "bash", "-c", "echo 'show version' | socat - /var/lib/haproxy/run/haproxy.sock").Output()
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

// waitForEffectiveHAProxyVersion polls the provided IngressController until its EffectiveHAProxyVersion equals expectedVersion.
func waitForEffectiveHAProxyVersion(ctx context.Context, operatorClient operatorv1client.Interface, ingress types.NamespacedName, expectedVersion operatorv1.HAProxyVersion, testsTimeout time.Duration) error {
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, testsTimeout, true, func(ctx context.Context) (bool, error) {
		ic, err := operatorClient.OperatorV1().IngressControllers(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get the IngressController %s: %s", ingress.String(), err.Error())
			return false, nil
		}
		if ic.Status.EffectiveHAProxyVersion != expectedVersion {
			e2e.Logf("IngressController %s: HAProxy version %q does not match expected value %q", ingress.String(), ic.Status.EffectiveHAProxyVersion, expectedVersion)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for EffectiveHAProxyVersion to match expected version: %s", err.Error())
	}
	return nil
}

func apiHasHAProxyVersionField(ctx context.Context, oc *exutil.CLI) (bool, error) {
	apiExtClient, err := apiextensionsclient.NewForConfig(oc.AdminConfig())
	if err != nil {
		return false, err
	}

	crd, err := apiExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "ingresscontrollers.operator.openshift.io", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check if haproxyVersion field exists in the CRD schema
	for _, v := range crd.Spec.Versions {
		if v.Name == "v1" && v.Schema != nil && v.Schema.OpenAPIV3Schema != nil {
			if _, ok := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["haproxyVersion"]; ok {
				return true, nil
			}
		}
	}
	return false, nil
}
