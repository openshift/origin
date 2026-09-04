package router

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "github.com/openshift/api/operator/v1"
	securityv1 "github.com/openshift/api/security/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"

	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
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

		apiExtClient, err := apiextensionsclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		crd, err := apiExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "ingresscontrollers.operator.openshift.io", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check if haproxyVersion field exists in the CRD schema
		hasField := false
		for _, v := range crd.Spec.Versions {
			if v.Name == "v1" && v.Schema != nil && v.Schema.OpenAPIV3Schema != nil {
				if _, ok := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["haproxyVersion"]; ok {
					hasField = true
				}
			}
		}
		if !hasField {
			g.Skip("IngressController CRD does not have haproxyVersion field — operator not yet updated")
		}

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

// applyHAProxySidecarToPod receives a pre-configured router pod and applies the HAProxy
// sidecar container on it, along with the needed configuration for the router to work.
// Use this when creating the router directly via the Pod API; use applyHAProxySidecarToPodTemplate()
// when going through a PodTemplateSpec (ReplicaSet/Deployment) instead.
func applyHAProxySidecarToPod(ctx context.Context, oc *exutil.CLI, routerPodSpec *corev1.PodSpec) error {
	if len(routerPodSpec.Containers) == 0 {
		return fmt.Errorf("provided router pod has no containers")
	}
	routerContainer := &routerPodSpec.Containers[0]

	defaultDeployment, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-ingress").Get(ctx, "router-default", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if len(defaultDeployment.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("router-default deployment has no containers")
	}

	routerPodSpec.ShareProcessNamespace = ptr.To(true)
	routerPodSpec.AutomountServiceAccountToken = ptr.To(false)
	routerPodSpec.InitContainers = defaultDeployment.Spec.Template.Spec.InitContainers

	// Defaults the ROUTER_HAPROXY_ADMIN_UNIX_SOCKET if the provided router container does not configure it.
	// https://github.com/openshift/cluster-ingress-operator/blob/5bf72fcc4534d9ba2c4d65d29cdb8b01c83cf550/pkg/operator/controller/ingress/deployment.go#L1379-L1383
	const haproxySocketEnvvar = "ROUTER_HAPROXY_ADMIN_UNIX_SOCKET"
	const haproxySocketValue = "/var/lib/haproxy/run/admin.sock"
	hasHAProxySocketEnvvar := slices.ContainsFunc(routerContainer.Env, func(e corev1.EnvVar) bool {
		return e.Name == haproxySocketEnvvar
	})
	if !hasHAProxySocketEnvvar {
		routerContainer.Env = append(routerContainer.Env, corev1.EnvVar{
			Name:  haproxySocketEnvvar,
			Value: haproxySocketValue,
		})
	}

	// We want to copy only these volumes from a running router: they are required for the
	// haproxy sidecar to run. Other volumes - default-certificate and service-ca-bundle
	// depend on other resources the testing namespace does not provide.
	sidecarVolumes := []string{"haproxy-config", "kube-api-access"}

	for i := range routerPodSpec.InitContainers {
		container := &routerPodSpec.InitContainers[i]
		container.VolumeMounts = slices.DeleteFunc(container.VolumeMounts, func(v corev1.VolumeMount) bool {
			return !slices.Contains(sidecarVolumes, v.Name)
		})
	}

	for _, volume := range defaultDeployment.Spec.Template.Spec.Volumes {
		if volume.Name == "kube-api-access" && volume.Projected != nil {
			// This is the service account volume. Our testing namespace and pod do not have the
			// proper configuration to allow the router container read the mounted token as 0400, the
			// ingress operator default. This is simpler than configuring the correct permissions,
			// acceptable since these tests aren't asserting on token hardening.
			//
			// Overriding to nil (defaults to 0644) gives the router read access to the token.
			volume.Projected.DefaultMode = nil
		}
		if slices.Contains(sidecarVolumes, volume.Name) {
			routerPodSpec.Volumes = append(routerPodSpec.Volumes, volume)
		}
	}
	for _, volumeMount := range defaultDeployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if slices.Contains(sidecarVolumes, volumeMount.Name) {
			routerPodSpec.Containers[0].VolumeMounts = append(routerPodSpec.Containers[0].VolumeMounts, volumeMount)
		}
	}
	return nil
}

// applyHAProxySidecarToPodTemplate receives a pre-configured router pod template and applies
// the HAProxy sidecar container on it, just like applyHAProxySidecarToPod(). It also grants
// the namespace's default service account access to the restricted SCC, and pins the pod
// template to require it - needed because pods created by a controller are SCC-checked
// against their own service account, not the caller's identity, and restricted-v2 (the
// cluster default) forbids the privilege escalation the sidecar needs.
func applyHAProxySidecarToPodTemplate(ctx context.Context, oc *exutil.CLI, namespace string, routerPodTemplateSpec *corev1.PodTemplateSpec) error {
	err := applyHAProxySidecarToPod(ctx, oc, &routerPodTemplateSpec.Spec)
	if err != nil {
		return err
	}

	if routerPodTemplateSpec.Annotations == nil {
		routerPodTemplateSpec.Annotations = make(map[string]string)
	}
	// The restricted-v2 scc preempts restricted, so we must pin to restricted.
	routerPodTemplateSpec.Annotations[securityv1.RequiredSCCAnnotation] = "restricted"

	_, err = oc.AdminKubeClient().RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "router",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: "default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "system:router",
		},
	}, metav1.CreateOptions{})
	if client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	// The router typically runs with allowPrivilegeEscalation enabled; however, all service accounts are assigned
	// to restricted-v2 scc by default, which disallows privilege escalation. The restricted policy permits
	// privilege escalation.
	_, err = oc.AdminKubeClient().RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "router-restricted",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: "default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "system:openshift:scc:restricted",
		},
	}, metav1.CreateOptions{})
	if client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	return nil
}
