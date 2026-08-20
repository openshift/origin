package apiserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
)

// These tests verify the TLSAdherence feature gate and the spec.tlsAdherence field on
// apiservers/cluster (config.openshift.io/v1).  They are gated by [OCPFeatureGate:TLSAdherence]
// for automatic pre-run filtering and include [FeatureGate:TLSAdherence] in each It description
// so the test name matches the pattern queried by the openshift/api verify-feature-promotion
// CI check in Sippy.
var _ = g.Describe("[sig-api-machinery][OCPFeatureGate:TLSAdherence][Feature:TLSAdherence] TLSAdherence apiservers/cluster", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-adherence")

	// Set spec.tlsAdherence to each valid value on the live cluster and verify
	// that the legacy-adhering control-plane components continue to enforce the configured
	// TLS security profile at the wire level after each change.
	// [Serial] because it mutates a cluster-wide singleton.
	g.It("[FeatureGate:TLSAdherence] [Serial] should accept spec.tlsAdherence changes and legacy-adhering components should continue to enforce the cluster TLS profile [apigroup:config.openshift.io]", func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift || isHyperShift {
			g.Skip("control-plane port-forward checks are not applicable to MicroShift or HyperShift")
		}

		original, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get apiservers/cluster")

		// Restore the field after the test. The field cannot be cleared once set, so
		// when it was originally unset we fall back to LegacyAdheringComponentsOnly
		// (the documented default behaviour) rather than leaving the cluster at
		// StrictAllComponents.
		g.DeferCleanup(func(ctx context.Context) {
			restoreValue := original.Spec.TLSAdherence
			if restoreValue == "" {
				restoreValue = configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly
			}
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = restoreValue
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore spec.tlsAdherence")
		})

		// Derive expected TLS handshake outcomes from the current security profile.
		var tlsShouldWork, tlsShouldNotWork *tls.Config
		switch {
		case original.Spec.TLSSecurityProfile == nil,
			original.Spec.TLSSecurityProfile.Type == configv1.TLSProfileIntermediateType:
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
		case original.Spec.TLSSecurityProfile.Type == configv1.TLSProfileModernType:
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		default:
			g.Skip("wire-level checks are only defined for Intermediate and Modern TLS profiles")
		}

		// checkLegacyAdheringComponents verifies that kube-apiserver and the OpenShift
		// API servers honour the cluster-wide TLS profile at the wire level.
		checkLegacyAdheringComponents := func() {
			for _, target := range []struct{ name, namespace, port string }{
				{"apiserver", "openshift-kube-apiserver", "443"},
				{"api", "openshift-apiserver", "443"},
				{"api", "openshift-oauth-apiserver", "443"},
			} {
				g.By(fmt.Sprintf("checking %s/%s TLS at the wire", target.namespace, target.name))
				err := forwardPortAndExecute(target.name, target.namespace, target.port,
					func(port int) error { return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork) })
				o.Expect(err).NotTo(o.HaveOccurred(),
					"%s/%s must enforce the cluster-wide TLS profile", target.namespace, target.name)
			}
		}

		for _, value := range []configv1.TLSAdherencePolicy{
			configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			configv1.TLSAdherencePolicyStrictAllComponents,
		} {
			g.By(fmt.Sprintf("setting spec.tlsAdherence=%s", value))
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = value
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to set spec.tlsAdherence=%s", value)

			g.By(fmt.Sprintf("verifying legacy-adhering components enforce the TLS profile with spec.tlsAdherence=%s", value))
			checkLegacyAdheringComponents()
		}
	})

	// Verify that non-legacy-adhering components only honor the cluster TLS profile
	// when spec.tlsAdherence is StrictAllComponents. Under LegacyAdheringComponentsOnly each
	// component falls back to its built-in Intermediate default regardless of the cluster-wide
	// tlsSecurityProfile. We don't restart the components' pods ourselves: their owning
	// operators are expected to observe the apiservers/cluster change and roll out new pods on
	// their own, so we poll the wire-level TLS behavior until it reflects the new configuration.
	// [Serial] because it mutates a cluster-wide singleton.
	g.It("[FeatureGate:TLSAdherence] [Serial] should enforce the cluster TLS profile on non-legacy-adhering components only when StrictAllComponents is set [apigroup:config.openshift.io]", func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift || isHyperShift {
			g.Skip("non-legacy-adhering components are not applicable to MicroShift or HyperShift")
		}

		original, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		type nonLegacyComponent struct {
			name        string
			namespace   string
			serviceName string
			webhookPort string
		}

		candidates := []nonLegacyComponent{
			{
				name:        "cluster-control-plane-machine-set-operator",
				namespace:   "openshift-machine-api",
				serviceName: "control-plane-machine-set-operator",
				webhookPort: "9443",
			},
			{
				name:        "cluster-baremetal-operator",
				namespace:   "openshift-machine-api",
				serviceName: "baremetal-operator-webhook-service",
				webhookPort: "9443",
			},
		}

		// Collect only those candidates that are actually deployed on this cluster.
		present := make([]nonLegacyComponent, 0, len(candidates))
		for _, c := range candidates {
			_, err := oc.AdminKubeClient().CoreV1().Services(c.namespace).Get(ctx, c.serviceName, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				continue
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to look up service %s/%s", c.namespace, c.serviceName)
			present = append(present, c)
		}

		if len(present) == 0 {
			g.Skip("no non-legacy-adhering components with webhook services found on this cluster")
		}

		g.DeferCleanup(func(ctx context.Context) {
			restoreValue := original.Spec.TLSAdherence
			if restoreValue == "" {
				restoreValue = configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly
			}
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = restoreValue
			current.Spec.TLSSecurityProfile = original.Spec.TLSSecurityProfile
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore spec.tlsAdherence and spec.tlsSecurityProfile")
		})

		// Under LegacyAdheringComponentsOnly the component ignores the cluster-wide Modern
		// profile and uses its built-in Intermediate configuration (TLS 1.2 minimum).
		// TLS 1.2 must succeed; TLS 1.1 must fail (Intermediate enforces TLS 1.2+).
		legacyShouldWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		legacyShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}

		// Under StrictAllComponents the component must honor the cluster-wide Modern profile
		// (TLS 1.3-only). TLS 1.3 must succeed; TLS 1.2 must fail.
		strictShouldWork := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		strictShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		// checkPresentComponents polls each component's webhook TLS at the wire, rather than
		// checking once, because the component's owning operator is expected to observe the
		// apiservers/cluster change and restart the webhook's pod on its own; we don't force a
		// restart ourselves.
		checkPresentComponents := func(tlsShouldWork, tlsShouldNotWork *tls.Config) {
			for _, comp := range present {
				g.By(fmt.Sprintf("waiting for %s webhook to enforce the expected TLS profile", comp.name))
				o.Eventually(func() error {
					return forwardPortAndExecute(comp.serviceName, comp.namespace, comp.webhookPort,
						func(port int) error {
							return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
						})
				}, 3*time.Minute, 5*time.Second).Should(o.Succeed(),
					"%s must enforce the expected TLS profile once its operator picks up the new configuration", comp.name)
			}
		}

		g.By("setting spec.tlsSecurityProfile=Modern and spec.tlsAdherence=LegacyAdheringComponentsOnly")
		current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		current.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
			Type:   configv1.TLSProfileModernType,
			Modern: &configv1.ModernTLSProfile{},
		}
		current.Spec.TLSAdherence = configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly
		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verifying non-legacy components use Intermediate TLS (built-in default; ignores cluster Modern profile)")
		checkPresentComponents(legacyShouldWork, legacyShouldNotWork)

		g.By("setting spec.tlsAdherence=StrictAllComponents")
		current, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		current.Spec.TLSAdherence = configv1.TLSAdherencePolicyStrictAllComponents
		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verifying non-legacy components now enforce Modern TLS (honor cluster-wide profile)")
		checkPresentComponents(strictShouldWork, strictShouldNotWork)
	})

	// Verify that changing spec.tlsAdherence through all valid values does not degrade
	// any cluster operator.
	// [Serial] because it mutates a cluster-wide singleton.
	g.It("[FeatureGate:TLSAdherence] [Serial] should not degrade any cluster operators when spec.tlsAdherence is changed [apigroup:config.openshift.io]", func(ctx context.Context) {
		original, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.DeferCleanup(func(ctx context.Context) {
			restoreValue := original.Spec.TLSAdherence
			if restoreValue == "" {
				restoreValue = configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly
			}
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = restoreValue
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore spec.tlsAdherence")
		})

		// Pre-flight: if operators haven't already settled before we touch anything, skip
		// rather than reporting a false failure attributable to our changes.
		if err := operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 5); err != nil {
			g.Skip(fmt.Sprintf("cluster operators are not settled before spec.tlsAdherence is changed; skipping to avoid false attribution: %v", err))
		}

		for _, value := range []configv1.TLSAdherencePolicy{
			configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			configv1.TLSAdherencePolicyStrictAllComponents,
		} {
			g.By(fmt.Sprintf("setting spec.tlsAdherence=%s", value))
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = value
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// spec.tlsAdherence is a pure configuration change: no cluster operator should
			// ever become unavailable, degraded, or stuck progressing as a result of setting it.
			g.By(fmt.Sprintf("verifying cluster operators settle after setting spec.tlsAdherence=%s", value))
			err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
			o.Expect(err).NotTo(o.HaveOccurred(),
				"no cluster operator should become degraded when spec.tlsAdherence=%s is set", value)
		}
	})

	// Verify that when spec.tlsAdherence is genuinely omitted (never set), legacy-adhering
	// components still enforce the cluster-wide TLS profile, matching the documented "Omission
	// Semantics" (empty string treated the same as LegacyAdheringComponentsOnly).
	// [Serial] because it mutates a cluster-wide singleton.
	g.It("[FeatureGate:TLSAdherence] [Serial] should enforce the cluster TLS profile for legacy-adhering components when spec.tlsAdherence is omitted [apigroup:config.openshift.io]", func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift || isHyperShift {
			g.Skip("control-plane port-forward checks are not applicable to MicroShift or HyperShift")
		}

		original, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get apiservers/cluster")

		// tlsAdherence cannot be cleared once set (enforced by CEL validation), so this test only
		// makes sense on a cluster where it has never been set. If a previous run or another test
		// has already set it, skip rather than attempt an update we know will be rejected.
		if original.Spec.TLSAdherence != "" {
			g.Skip("spec.tlsAdherence is already set on this cluster and cannot be cleared; skipping omitted-value check")
		}

		// Derive expected TLS handshake outcomes from the current security profile, same as the
		// explicit-value test above.
		var tlsShouldWork, tlsShouldNotWork *tls.Config
		switch {
		case original.Spec.TLSSecurityProfile == nil,
			original.Spec.TLSSecurityProfile.Type == configv1.TLSProfileIntermediateType:
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
		case original.Spec.TLSSecurityProfile.Type == configv1.TLSProfileModernType:
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		default:
			g.Skip("wire-level checks are only defined for Intermediate and Modern TLS profiles")
		}

		g.By("verifying legacy-adhering components enforce the TLS profile with spec.tlsAdherence omitted")
		for _, target := range []struct{ name, namespace, port string }{
			{"apiserver", "openshift-kube-apiserver", "443"},
			{"api", "openshift-apiserver", "443"},
			{"api", "openshift-oauth-apiserver", "443"},
		} {
			g.By(fmt.Sprintf("checking %s/%s TLS at the wire", target.namespace, target.name))
			err := forwardPortAndExecute(target.name, target.namespace, target.port,
				func(port int) error { return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork) })
			o.Expect(err).NotTo(o.HaveOccurred(),
				"%s/%s must enforce the cluster-wide TLS profile even when spec.tlsAdherence is omitted", target.namespace, target.name)
		}
	})

	// Verify override precedence: a component with an explicit CR-level TLS override
	// (IngressController.spec.tlsSecurityProfile) keeps honoring its own setting even when
	// spec.tlsAdherence=StrictAllComponents and the cluster-wide profile differs.
	// [Serial] because it mutates cluster-wide and component-specific singletons.
	g.It("[FeatureGate:TLSAdherence] [Serial] should let IngressController honor its own tlsSecurityProfile override when spec.tlsAdherence=StrictAllComponents [apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift || isHyperShift {
			g.Skip("default IngressController checks are not applicable to MicroShift or HyperShift")
		}

		originalAPIServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get apiservers/cluster")

		originalIngress, err := oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Get(ctx, "default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get default ingresscontroller")

		g.DeferCleanup(func(ctx context.Context) {
			restoreValue := originalAPIServer.Spec.TLSAdherence
			if restoreValue == "" {
				restoreValue = configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly
			}
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = restoreValue
			current.Spec.TLSSecurityProfile = originalAPIServer.Spec.TLSSecurityProfile
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore apiservers/cluster")

			currentIngress, err := oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Get(ctx, "default", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			currentIngress.Spec.TLSSecurityProfile = originalIngress.Spec.TLSSecurityProfile
			_, err = oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Update(ctx, currentIngress, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore default ingresscontroller tlsSecurityProfile")
		})

		// Intermediate should keep working (TLS 1.2+); TLS 1.1-only must fail, proving the router
		// is enforcing its own Intermediate override rather than the cluster-wide Modern profile.
		overrideShouldWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		overrideShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}

		g.By("setting spec.tlsSecurityProfile=Modern and spec.tlsAdherence=StrictAllComponents on apiservers/cluster")
		currentAPIServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		currentAPIServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
			Type:   configv1.TLSProfileModernType,
			Modern: &configv1.ModernTLSProfile{},
		}
		currentAPIServer.Spec.TLSAdherence = configv1.TLSAdherencePolicyStrictAllComponents
		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, currentAPIServer, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("setting an explicit Intermediate tlsSecurityProfile override on the default ingresscontroller")
		currentIngress, err := oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Get(ctx, "default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		currentIngress.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
			Type:         configv1.TLSProfileIntermediateType,
			Intermediate: &configv1.IntermediateTLSProfile{},
		}
		_, err = oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Update(ctx, currentIngress, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the default ingresscontroller to report Available and non-Progressing after the override")
		err = shard.WaitForIngressControllerCondition(oc, 3*time.Minute,
			types.NamespacedName{Namespace: "openshift-ingress-operator", Name: "default"},
			operatorv1.OperatorCondition{Type: operatorv1.IngressControllerAvailableConditionType, Status: operatorv1.ConditionTrue},
			operatorv1.OperatorCondition{Type: operatorv1.OperatorStatusTypeProgressing, Status: operatorv1.ConditionFalse},
		)
		o.Expect(err).NotTo(o.HaveOccurred(), "default ingresscontroller did not settle after setting the tlsSecurityProfile override")

		g.By("verifying router-default continues to enforce its own Intermediate override, not the cluster-wide Modern profile")
		o.Eventually(func() error {
			return forwardPortAndExecute("router-default", "openshift-ingress", "443",
				func(port int) error { return checkTLSConnection(port, overrideShouldWork, overrideShouldNotWork) })
		}, 3*time.Minute, 5*time.Second).Should(o.Succeed(),
			"router-default must keep honoring its own tlsSecurityProfile override once its operator picks up the change")
	})
})
