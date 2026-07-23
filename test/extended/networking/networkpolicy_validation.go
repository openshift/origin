package networking

import (
	"context"
	"fmt"
	"slices"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network] NetworkPolicy", func() {
	oc := exutil.NewCLIWithoutNamespace("networkpolicy-validation")

	// Some CNIs like OVN-Kubernetes do not support named ports in NetworkPolicies.
	// When a named port is used, they may silently convert the rule
	// into an allow-all, effectively bypassing the intended restriction.
	g.It("should use numeric ports in all platform NetworkPolicies [apigroup:networking.k8s.io]", func() {
		// Known NetworkPolicies that still use named ports.
		// Each entry is "namespace/name".
		// TODO: remove entries as the owning teams fix them.
		networkPoliciesToSkip := []string{
			// https://redhat.atlassian.net/browse/OCPBUGS-99494
			"openshift-ingress/router-default",
			// https://redhat.atlassian.net/browse/OCPBUGS-99492
			"openshift-cluster-csi-drivers/allow-to-dns",
			// https://redhat.atlassian.net/browse/OCPBUGS-99492
			"openshift-cluster-storage-operator/allow-to-dns",
			// https://redhat.atlassian.net/browse/OCPBUGS-99493
			"openshift-cluster-olm-operator/allow-egress-to-openshift-dns",
			// https://redhat.atlassian.net/browse/OCPBUGS-99493
			"openshift-operator-lifecycle-manager/catalog-operator",
			// https://redhat.atlassian.net/browse/OCPBUGS-99493
			"openshift-operator-lifecycle-manager/olm-operator",
		}

		ctx := context.Background()

		nps, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list NetworkPolicies across all namespaces")
		o.Expect(nps.Items).NotTo(o.BeEmpty(), "expected at least one NetworkPolicy on the cluster")

		var violations []string
		for _, np := range nps.Items {
			// Only check platform namespaces. This won't catch broken (on OVN-Kubernetes e.g.) NPs
			// added by tests in ephemeral namespaces, but this will avoid false
			// positives from tests that intentionally use named ports
			// (e.g. verifying named-port flows or running on a CNI that
			// supports them).
			// For consistency, platform NetworkPolicies should use numeric ports across all CNIs.
			if !strings.HasPrefix(np.Namespace, "openshift-") && !strings.HasPrefix(np.Namespace, "kube-") && np.Namespace != "default" {
				continue
			}
			key := fmt.Sprintf("%s/%s", np.Namespace, np.Name)
			skip := slices.Contains(networkPoliciesToSkip, key)
			for _, rule := range np.Spec.Ingress {
				for _, p := range rule.Ports {
					if p.Port != nil && p.Port.Type != intstr.Int {
						msg := fmt.Sprintf("%s ingress has named port %q (skip=%v)", key, p.Port.StrVal, skip)
						e2e.Logf("%s", msg)
						if !skip {
							violations = append(violations, msg)
						}
					}
				}
			}
			for _, rule := range np.Spec.Egress {
				for _, p := range rule.Ports {
					if p.Port != nil && p.Port.Type != intstr.Int {
						msg := fmt.Sprintf("%s egress has named port %q (skip=%v)", key, p.Port.StrVal, skip)
						e2e.Logf("%s", msg)
						if !skip {
							violations = append(violations, msg)
						}
					}
				}
			}
		}
		o.Expect(violations).To(o.BeEmpty(), "some NetworkPolicies use named ports instead of numeric")
	})
})
