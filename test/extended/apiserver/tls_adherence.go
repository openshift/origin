package apiserver

import (
	"context"
	"crypto/tls"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	exutil "github.com/openshift/origin/test/extended/util"
)

// tlsAdherenceFeatureGateName is the name of the TLSAdherence feature gate as it
// appears in featuregate/cluster .status.featureGates[].enabled[].name.
const tlsAdherenceFeatureGateName = configv1.FeatureGateName("TLSAdherence")

// isTLSAdherenceFeatureGateEnabled returns true when the TLSAdherence feature gate is
// listed as enabled in the featuregate/cluster status.
func isTLSAdherenceFeatureGateEnabled(ctx context.Context, oc *exutil.CLI) (bool, error) {
	fg, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get featuregate/cluster: %w", err)
	}
	for _, featureGateValues := range fg.Status.FeatureGates {
		for _, enabledGate := range featureGateValues.Enabled {
			if enabledGate.Name == tlsAdherenceFeatureGateName {
				return true, nil
			}
		}
	}
	return false, nil
}

// These tests verify the TLSAdherence feature gate and the spec.tlsAdherence field on
// apiservers/cluster (config.openshift.io/v1).  They are gated by [OCPFeatureGate:TLSAdherence]
// for automatic pre-run filtering and include [FeatureGate:TLSAdherence] in each It description
// so the test name matches the pattern queried by the openshift/api verify-feature-promotion
// CI check in Sippy.
var _ = g.Describe("[sig-api-machinery][OCPFeatureGate:TLSAdherence][Feature:TLSAdherence] TLSAdherence apiservers/cluster", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-adherence")

	g.BeforeEach(func(ctx context.Context) {
		enabled, err := isTLSAdherenceFeatureGateEnabled(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !enabled {
			g.Skip("TLSAdherence feature gate is not enabled on this cluster")
		}
	})

	g.It("[FeatureGate:TLSAdherence] should reject an invalid spec.tlsAdherence value on apiservers/cluster [apigroup:config.openshift.io]", func(ctx context.Context) {
		current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get apiservers/cluster")

		desired := current.DeepCopy()
		desired.Spec.TLSAdherence = configv1.TLSAdherencePolicy("InvalidValue")

		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, desired, metav1.UpdateOptions{
			DryRun: []string{metav1.DryRunAll},
		})
		o.Expect(err).To(o.HaveOccurred(),
			"apiservers/cluster should reject an invalid spec.tlsAdherence value")
		o.Expect(k8serrors.IsInvalid(err)).To(o.BeTrue(),
			"error should be a 422 Invalid, got: %v", err)
	})

	g.It("[FeatureGate:TLSAdherence] should accept and reflect all valid spec.tlsAdherence values on apiservers/cluster [apigroup:config.openshift.io]", func(ctx context.Context) {
		validValues := []configv1.TLSAdherencePolicy{
			configv1.TLSAdherencePolicyStrictAllComponents,
			configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
		}

		for _, value := range validValues {
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get apiservers/cluster")

			desired := current.DeepCopy()
			desired.Spec.TLSAdherence = value

			result, err := oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, desired, metav1.UpdateOptions{
				DryRun: []string{metav1.DryRunAll},
			})
			o.Expect(err).NotTo(o.HaveOccurred(),
				"apiservers/cluster should accept spec.tlsAdherence=%s", value)
			o.Expect(result.Spec.TLSAdherence).To(
				o.Equal(value),
				"apiservers/cluster should reflect spec.tlsAdherence=%s", value)
		}
	})

	g.It("[FeatureGate:TLSAdherence] should only permit cluster-admins to update apiservers/cluster [apigroup:config.openshift.io]", func(ctx context.Context) {
		sarClient := oc.AdminKubeClient().AuthorizationV1().SubjectAccessReviews()
		resourceAttrs := &authorizationv1.ResourceAttributes{
			Group:    "config.openshift.io",
			Resource: "apiservers",
			Name:     "cluster",
			Verb:     "update",
		}

		// A plain authenticated user must not be allowed.
		noAccessSAR, err := sarClient.Create(ctx, &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User:               "regularuser",
				Groups:             []string{"system:authenticated"},
				ResourceAttributes: resourceAttrs,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(noAccessSAR.Status.Allowed).To(o.BeFalse(),
			"a non-admin user should not be permitted to update apiservers/cluster")

		// A member of system:masters (cluster-admin) must be allowed.
		adminSAR, err := sarClient.Create(ctx, &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User:               "system:admin",
				Groups:             []string{"system:masters"},
				ResourceAttributes: resourceAttrs,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(adminSAR.Status.Allowed).To(o.BeTrue(),
			"a cluster-admin (system:masters) must be permitted to update apiservers/cluster")
	})

	// Test 4 – verify that authenticated non-admin users can read apiservers/cluster.
	// Components and operators need to be able to read spec.tlsAdherence even if they
	// cannot write it.
	g.It("[FeatureGate:TLSAdherence] should allow authenticated users to read apiservers/cluster [apigroup:config.openshift.io]", func(ctx context.Context) {
		sar, err := oc.AdminKubeClient().AuthorizationV1().SubjectAccessReviews().Create(ctx,
			&authorizationv1.SubjectAccessReview{
				Spec: authorizationv1.SubjectAccessReviewSpec{
					User:   "regularuser",
					Groups: []string{"system:authenticated"},
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Group:    "config.openshift.io",
						Resource: "apiservers",
						Name:     "cluster",
						Verb:     "get",
					},
				},
			}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sar.Status.Allowed).To(o.BeTrue(),
			"authenticated users must be able to read apiservers/cluster to observe spec.tlsAdherence")
	})

	// Test 5 – verify that an invalid spec.tlsAdherence value is rejected via PATCH as
	// well as UPDATE, confirming validation is enforced across HTTP verbs.
	g.It("[FeatureGate:TLSAdherence] should reject an invalid spec.tlsAdherence value sent via PATCH on apiservers/cluster [apigroup:config.openshift.io]", func(ctx context.Context) {
		_, err := oc.AdminConfigClient().ConfigV1().APIServers().Patch(ctx, "cluster",
			types.MergePatchType,
			[]byte(`{"spec":{"tlsAdherence":"InvalidValue"}}`),
			metav1.PatchOptions{DryRun: []string{metav1.DryRunAll}},
		)
		o.Expect(err).To(o.HaveOccurred(),
			"PATCH with an invalid spec.tlsAdherence value should be rejected")
		o.Expect(k8serrors.IsInvalid(err)).To(o.BeTrue(),
			"error should be a 422 Invalid, got: %v", err)
	})

	// Test 6 – set spec.tlsAdherence to each valid value on the live cluster and verify
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

		// Restore to the original tlsAdherence value when possible.
		// The field cannot be removed once set, so if it was unset we leave it at
		// LegacyAdheringComponentsOnly (the documented default behaviour).
		g.DeferCleanup(func(ctx context.Context) {
			if original.Spec.TLSAdherence == "" {
				return
			}
			current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			current.Spec.TLSAdherence = original.Spec.TLSAdherence
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
})
