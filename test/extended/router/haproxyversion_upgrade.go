package router

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// HAProxyVersionUpgradeTest verifies if HAProxy version selection behaves
// as expected during upgrades.
// Pinned is a test parameter that should define whether the test uses a
// pinned version during upgrades, or it should upgrade leaving version unset.
type HAProxyVersionUpgradeTest struct {
	Pinned bool
	//
	oc             *exutil.CLI
	operatorClient operatorv1client.Interface
	controllers    *ingressControllers
	precheckErr    error
	ic             types.NamespacedName
	pinnedVersion  operatorv1.HAProxyVersion // only used if Pinned is true
}

func (h *HAProxyVersionUpgradeTest) Name() string {
	if h.Pinned {
		return "haproxy-pinned-version-upgrade"
	}
	return "haproxy-unset-version-upgrade"
}

func (h *HAProxyVersionUpgradeTest) DisplayName() string {
	if h.Pinned {
		return "[sig-network-edge][Feature:Router][apigroup:route.openshift.io] Verify HAProxy pinned version state during upgrade"
	}
	return "[sig-network-edge][Feature:Router][apigroup:route.openshift.io] Verify HAProxy unset version state during upgrade"
}

// Skip defines if the test should be skipped. HAProxy version test is skipped
// if the HAProxy version field cannot be found in the API.
func (h *HAProxyVersionUpgradeTest) Skip(_ upgrades.UpgradeContext) bool {
	oc := exutil.NewCLIForMonitorTest(h.Name() + "-skip").AsAdmin()
	hasField, err := apiHasHAProxyVersionField(context.Background(), oc)
	if err != nil {
		h.precheckErr = fmt.Errorf("error checking for HAProxy version API: %w", err)
		return false
	}

	h.precheckErr = nil
	return !hasField
}

// Setup configures all the test attributes and creates an IngressController
// resource that should be verified after the upgrade.
func (h *HAProxyVersionUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	o.Expect(h.precheckErr).NotTo(o.HaveOccurred(), "Skip() precheck failed: could not determine if HAProxy version upgrade test should run")

	g.By("Setting up HAProxy version test")

	h.oc = exutil.NewCLIWithFramework(f).AsAdmin()
	h.operatorClient = h.oc.AdminOperatorClient()
	h.controllers = &ingressControllers{}

	var customIngress func(*operatorv1.IngressController)
	versions, err := getHAProxyVersionParams(ctx, h.oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy versions")
	if h.Pinned {
		customIngress = func(ic *operatorv1.IngressController) {
			ic.Spec.HAProxyVersion = versions.defaultVersion
		}
		h.pinnedVersion = versions.defaultVersion
	} else {
		h.pinnedVersion = ""
	}

	g.By("Creating the IngressController resource")

	const createControllerTimeout = 2 * time.Minute
	ic, err := h.controllers.createIngressController(ctx, h.oc, createControllerTimeout, customIngress)
	o.Expect(err).NotTo(o.HaveOccurred(), "error creating IngressController resource")
	h.ic = types.NamespacedName{
		Namespace: ic.Namespace,
		Name:      ic.Name,
	}

	g.By("Checking HAProxy version for Ingress " + ic.Name)

	err = waitForHAProxyVersion(ctx, h.oc, ic.Name, versions.defaultVersion)
	o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy version from runtime API")
}

// Test verifies if the expected HAProxy version is found after the upgrade.
// Current version is read from the IngressController status and from the
// HAProxy's runtime API.
func (h *HAProxyVersionUpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	g.By("Waiting for upgrade to complete")
	<-done

	g.By("Validating HAProxy version after upgrade")

	var expectedVersion operatorv1.HAProxyVersion
	if h.Pinned {
		expectedVersion = h.pinnedVersion
	} else {
		versions, err := getHAProxyVersionParams(ctx, h.oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy versions")
		expectedVersion = versions.defaultVersion
	}

	const rollingOutTimeout = 15 * time.Minute
	err := wait.PollUntilContextTimeout(ctx, time.Second, rollingOutTimeout, true, func(ctx context.Context) (ready bool, err error) {
		ic, err := h.operatorClient.OperatorV1().IngressControllers(h.ic.Namespace).Get(ctx, h.ic.Name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("error getting IngressController resource: %s", err.Error())
			return false, nil
		}
		if ic.Status.EffectiveHAProxyVersion != expectedVersion {
			framework.Logf("HAProxy version from IngressResource status %q does not match expected value %q", ic.Status.EffectiveHAProxyVersion, expectedVersion)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "timed out waiting for EffectiveHAProxyVersion to match expected version")

	g.By("Validating HAProxy version from runtime API")

	err = waitForHAProxyVersion(ctx, h.oc, h.ic.Name, expectedVersion)
	o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy version from runtime API")
}

// Teardown removes the configured IngressController after the test runs.
func (h *HAProxyVersionUpgradeTest) Teardown(ctx context.Context, f *framework.Framework) {
	if h.operatorClient == nil {
		framework.Logf("Skipping cleanup because setup did not initialize test resources")
		return
	}
	if err := h.controllers.deleteAll(ctx, h.operatorClient); err != nil {
		framework.Logf("error deleting IngressController resource: %s", err.Error())
	}
}

// haproxyVersionParams has HAProxy version parameters from the Ingress operator.
type haproxyVersionParams struct {
	defaultVersion operatorv1.HAProxyVersion
}

// getHAProxyVersionParams parses the current Ingress operator configuration
// and extracts HAProxy version parameters.
func getHAProxyVersionParams(ctx context.Context, oc *exutil.CLI) (haproxyVersionParams, error) {
	operatorNamespace := "openshift-ingress-operator"
	operatorName := "ingress-operator"
	deploy, err := oc.AdminKubeClient().AppsV1().Deployments(operatorNamespace).Get(ctx, operatorName, metav1.GetOptions{})
	if err != nil {
		return haproxyVersionParams{}, err
	}

	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) < 1 {
		return haproxyVersionParams{}, fmt.Errorf("ingress-operator deployment is missing the operator container")
	}

	operator := containers[0]
	if operator.Name != "ingress-operator" {
		return haproxyVersionParams{}, fmt.Errorf("ingress-operator deployment has an unexpected container name: %s", operator.Name)
	}

	defaultVersion := func() string {
		for _, env := range operator.Env {
			if env.Name == "DEFAULT_HAPROXY_VERSION" {
				return env.Value
			}
		}
		// DEFAULT_HAPROXY_VERSION envvar not found, so this is pre 4.23/5.0, assume "2.8"
		return "2.8"
	}()

	return haproxyVersionParams{
		defaultVersion: operatorv1.HAProxyVersion(defaultVersion),
	}, nil
}
