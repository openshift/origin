package router

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// HAProxyVersionUpgradeTest verifies that HAProxy version selection behaves
// as expected during upgrades.
// Mode is a test parameter that should define how the HAProxy version
// should be configured before the upgrade.
type HAProxyVersionUpgradeTest struct {
	Mode HAProxyUpgradeMode

	// internal state
	oc             *exutil.CLI
	operatorClient operatorv1client.Interface
	controllers    *ingressControllers
	versionConfig  haproxyVersionConfig
	pinnedVersion  operatorv1.HAProxyVersion
	precheckErr    error
	ic             types.NamespacedName
}

// HAProxyUpgradeMode is the mode of the HAProxy upgrade.
type HAProxyUpgradeMode string

const (
	// HAProxyUpgradeModeUnset defines the HAProxy version as unpinned before the upgrade.
	HAProxyUpgradeModeUnset HAProxyUpgradeMode = "unset"
	// HAProxyUpgradeModeDefault defines the HAProxy version with the default version before the upgrade.
	HAProxyUpgradeModeDefault HAProxyUpgradeMode = "default"
	// HAProxyUpgradeModeNonDefault defines the HAProxy version with a non default but supported version before the upgrade.
	HAProxyUpgradeModeNonDefault HAProxyUpgradeMode = "non-default"
)

func (h *HAProxyVersionUpgradeTest) Name() string {
	return "haproxy-version-upgrade-" + string(h.Mode)
}

func (h *HAProxyVersionUpgradeTest) DisplayName() string {
	return fmt.Sprintf("[sig-network-edge][Feature:Router][apigroup:route.openshift.io] Verify HAProxy %s version state during upgrade", h.Mode)
}

// Skip returns true when the test cannot safely run: the API lacks the haproxyVersion field, the upgrade
// is a multi-hop chain with a pinned version, or (for NonDefault) no safe non-default version is available.
func (h *HAProxyVersionUpgradeTest) Skip(upgctx upgrades.UpgradeContext) bool {
	framework.Logf("Upgrade config: %+v", upgctx)

	if h.Mode != HAProxyUpgradeModeUnset && len(upgctx.Versions) > 2 {
		// We could have a deprecation and dropping version in the middle of a
		// multi-hop upgrade, so we cannot safely run the test having HAProxy pinned.
		framework.Logf("skipping: cannot test a multi-hop upgrade with HAProxy pinned. mode=%q, versions=%d", h.Mode, len(upgctx.Versions))
		return true
	}

	ctx := context.Background()

	oc := exutil.NewCLIForMonitorTest(h.Name() + "-skip").AsAdmin()
	hasField, err := apiHasHAProxyVersionField(ctx, oc)
	if err != nil {
		h.precheckErr = fmt.Errorf("error checking for HAProxy version API: %w", err)
		return false
	}
	if !hasField {
		framework.Logf("skipping: IngressController API is missing the haproxyVersion field")
		return true
	}

	versions, err := getHAProxyVersionConfig(ctx, oc)
	if err != nil {
		h.precheckErr = fmt.Errorf("error getting HAProxy version config: %w", err)
		return false
	}
	framework.Logf("HAProxy version config: %+v", versions)

	if h.Mode == HAProxyUpgradeModeNonDefault && len(versions.getNonDefaultVersions()) == 0 {
		framework.Logf("skipping: cannot use non default: there are no non default versions")
		return true
	}

	if h.Mode == HAProxyUpgradeModeNonDefault && len(versions.getNonDefaultUpgradeableVersions()) == 0 {
		// Strictly, only a y-stream upgrade could drop this version, but Skip() cannot
		// reliably tell y-stream from z-stream before the upgrade completes (the target
		// may be given as a pull-spec, not a parseable Version), so we skip conservatively
		// regardless of upgrade type.
		framework.Logf("skipping: cannot use non default: the only available non default version is deprecated")
		return true
	}

	h.versionConfig = versions
	h.precheckErr = nil
	return false
}

// Setup configures all the test attributes and creates an IngressController
// resource that should be verified after the upgrade.
func (h *HAProxyVersionUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	o.Expect(h.precheckErr).NotTo(o.HaveOccurred(), "Skip() precheck failed: could not determine if HAProxy version upgrade test should run")

	g.By("Setting up HAProxy version test")

	h.oc = exutil.NewCLIWithFramework(f).AsAdmin()
	h.operatorClient = h.oc.AdminOperatorClient()
	h.controllers = &ingressControllers{}

	var haproxyVersion operatorv1.HAProxyVersion
	// Default and NonDefault cases should have their versions inverted in case
	// the default version is overridden, see getHAProxyVersionConfig().
	switch h.Mode {
	case HAProxyUpgradeModeUnset:
		haproxyVersion = ""
	case HAProxyUpgradeModeDefault:
		haproxyVersion = h.versionConfig.defaultVersion
	case HAProxyUpgradeModeNonDefault:
		haproxyVersion = h.versionConfig.getNonDefaultUpgradeableVersions()[0]
	default:
		framework.Failf("unsupported test mode: %q", h.Mode)
	}

	g.By("Creating the IngressController resource")

	ic, err := h.controllers.createIngressController(ctx, h.oc, func(controller *operatorv1.IngressController) {
		controller.Spec.HAProxyVersion = haproxyVersion
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "error creating IngressController resource")
	h.ic = types.NamespacedName{
		Namespace: ic.Namespace,
		Name:      ic.Name,
	}
	h.pinnedVersion = haproxyVersion

	framework.Logf("Created IngressController %s with spec.haproxyVersion=%q", h.ic.String(), haproxyVersion)

	g.By("Checking HAProxy version for Ingress " + h.ic.String())

	waitingVersion := haproxyVersion
	if waitingVersion == "" {
		waitingVersion = h.versionConfig.defaultVersion
	}
	err = waitForHAProxyVersion(ctx, h.oc, ic.Name, waitingVersion)
	o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy version from runtime API")
}

// Test verifies that the expected HAProxy version is found after the upgrade.
// Current version is read from the IngressController status and from the
// HAProxy's runtime API.
func (h *HAProxyVersionUpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	defer g.GinkgoRecover()

	g.By("Waiting for upgrade to complete")
	<-done

	err := waitForIngressControllerReady(h.oc, h.ic)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("error waiting for IngressController %s to be ready", h.ic.String()))

	g.By("Validating HAProxy version after upgrade")

	versions, err := getHAProxyVersionConfig(ctx, h.oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "error getting HAProxy version config")
	framework.Logf("HAProxy version config: %+v", versions)

	var expectedVersion operatorv1.HAProxyVersion
	switch h.Mode {
	case HAProxyUpgradeModeUnset:
		expectedVersion = versions.defaultVersion
	case HAProxyUpgradeModeDefault, HAProxyUpgradeModeNonDefault:
		expectedVersion = h.pinnedVersion
	default:
		framework.Failf("unsupported test mode: %q", h.Mode)
	}

	framework.Logf("Post-upgrade HAProxy version check: expected=%s", expectedVersion)

	const rollingOutTimeout = 15 * time.Minute
	err = waitForEffectiveHAProxyVersion(ctx, h.operatorClient, h.ic, expectedVersion, rollingOutTimeout)
	o.Expect(err).NotTo(o.HaveOccurred(), "error waiting for EffectiveHAProxyVersion")

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

// haproxyVersionConfig has HAProxy version configuration from the Ingress operator.
type haproxyVersionConfig struct {
	defaultVersion    operatorv1.HAProxyVersion
	deprecatedVersion operatorv1.HAProxyVersion
	availableVersions []operatorv1.HAProxyVersion
}

// getHAProxyVersionConfig parses the current Ingress operator configuration and extracts
// the HAProxy version configuration. The default version can be overridden if the cluster
// has an unsupported HAProxy version override annotation set.
func getHAProxyVersionConfig(ctx context.Context, oc *exutil.CLI) (haproxyVersionConfig, error) {
	kubeClient := oc.AdminKubeClient()
	operatorNamespace := "openshift-ingress-operator"
	operatorName := "ingress-operator"
	topology, err := exutil.GetControlPlaneTopology(oc)
	if err != nil {
		return haproxyVersionConfig{}, fmt.Errorf("error getting control plane topology: %w", err)
	}
	if *topology == configv1.ExternalTopologyMode {
		mgmtKubeconfig, hcpNamespace, err := exutil.GetHypershiftManagementClusterConfigAndNamespace()
		if err != nil {
			return haproxyVersionConfig{}, fmt.Errorf("error getting HyperShift management cluster config: %w", err)
		}
		mgmtConfig, err := exutil.GetClientConfig(mgmtKubeconfig)
		if err != nil {
			return haproxyVersionConfig{}, fmt.Errorf("error loading HyperShift management cluster config: %w", err)
		}
		if kubeClient, err = kubernetes.NewForConfig(mgmtConfig); err != nil {
			return haproxyVersionConfig{}, fmt.Errorf("error building HyperShift management cluster client: %w", err)
		}
		operatorNamespace = hcpNamespace
	}

	deploy, err := kubeClient.AppsV1().Deployments(operatorNamespace).Get(ctx, operatorName, metav1.GetOptions{})
	if err != nil {
		return haproxyVersionConfig{}, err
	}

	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) < 1 {
		return haproxyVersionConfig{}, fmt.Errorf("ingress-operator deployment is missing the operator container")
	}

	operator := containers[0]
	if operator.Name != operatorName {
		return haproxyVersionConfig{}, fmt.Errorf("ingress-operator deployment has an unexpected container name: %s", operator.Name)
	}

	// Checking if HAProxy version is being overridden:
	//
	//   oc annotate ingress.config cluster \
	//     unsupported.ingress.openshift.io/default-haproxy-version="${HAPROXY_VERSION}" \
	//     --overwrite
	//
	// https://github.com/openshift/release/blob/6833a2362a7e48156c4872d669a018b06123da2c/ci-operator/step-registry/ingress/conf/haproxy-version/ingress-conf-haproxy-version-commands.sh#L17-L19
	ingCluster, err := oc.AdminConfigClient().ConfigV1().Ingresses().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return haproxyVersionConfig{}, fmt.Errorf("error reading Ingress cluster config for HAProxy version override check: %w", err)
	}

	// missing ingress.config/cluster annotation means not overridden, so defaultVersion remains empty.
	var defaultVersion operatorv1.HAProxyVersion
	if overrideVersion := ingCluster.Annotations["unsupported.ingress.openshift.io/default-haproxy-version"]; overrideVersion != "" {
		framework.Logf("HAProxy default version overridden to %q via ingress.config/cluster annotation", overrideVersion)
		defaultVersion = operatorv1.HAProxyVersion(overrideVersion)
	}

	// Read default and deprecated versions from Env
	var deprecatedVersion operatorv1.HAProxyVersion
	for _, env := range operator.Env {
		switch env.Name {
		case "DEFAULT_HAPROXY_VERSION":
			if defaultVersion == "" {
				// update only if not being overridden.
				defaultVersion = operatorv1.HAProxyVersion(env.Value)
			}
		case "DEPRECATED_HAPROXY_VERSION":
			deprecatedVersion = operatorv1.HAProxyVersion(env.Value)
		}
	}
	if defaultVersion == "" {
		// envvar not found and version not overridden, so this is pre 4.23/5.0, assume "2.8"
		defaultVersion = "2.8"
	}
	if deprecatedVersion == "" {
		// envvar/flag not configured (e.g. HyperShift's asset doesn't set it at all),
		// so fall back to the operator's own compiled default.
		deprecatedVersion = operatorv1.HAProxyVersion28
	}

	// Read available versions from Command.
	// The available versions are configured this way:
	//
	// command:
	// - ...
	// - --haproxy-image
	// - "2.8=$(HAPROXY_28_IMAGE)"
	// - --haproxy-image
	// - "3.2=$(HAPROXY_32_IMAGE)"
	//
	var availableVersions []operatorv1.HAProxyVersion
	cmds := operator.Command
	for i := range cmds {
		if cmds[i] == "--haproxy-image" && len(cmds) > i+1 {
			// "2.8=$(HAPROXY_28_IMAGE)"
			value := cmds[i+1]
			// ["2.8", "$(HAPROXY_28_IMAGE)"]
			version := strings.Split(value, "=")
			availableVersions = append(availableVersions, operatorv1.HAProxyVersion(version[0]))
		}
	}
	if len(availableVersions) == 0 {
		// --haproxy-image not configured, so this is pre 4.23/5.0, assume [defaultVersion]
		availableVersions = []operatorv1.HAProxyVersion{defaultVersion}
	}

	// a sanity check ensures default and deprecated are valid versions.
	if !slices.Contains(availableVersions, defaultVersion) {
		return haproxyVersionConfig{},
			fmt.Errorf("the available versions list %v does not include the default version %q", availableVersions, defaultVersion)
	}
	if deprecatedVersion != "" && !slices.Contains(availableVersions, deprecatedVersion) {
		return haproxyVersionConfig{},
			fmt.Errorf("the available versions list %v does not include the deprecated version %q", availableVersions, deprecatedVersion)
	}

	return haproxyVersionConfig{
		defaultVersion:    defaultVersion,
		deprecatedVersion: deprecatedVersion,
		availableVersions: availableVersions,
	}, nil
}

// getNonDefaultVersions creates a list of non default versions, derived from the default and the available ones.
func (h *haproxyVersionConfig) getNonDefaultVersions() []operatorv1.HAProxyVersion {
	return slices.DeleteFunc(slices.Clone(h.availableVersions), func(v operatorv1.HAProxyVersion) bool {
		return v == h.defaultVersion
	})
}

// getNonDefaultUpgradeableVersions creates a list of non default and upgradeable versions, derived from the default, the deprecated, and the available ones.
func (h *haproxyVersionConfig) getNonDefaultUpgradeableVersions() []operatorv1.HAProxyVersion {
	return slices.DeleteFunc(slices.Clone(h.availableVersions), func(v operatorv1.HAProxyVersion) bool {
		return v == h.defaultVersion || v == h.deprecatedVersion
	})
}
