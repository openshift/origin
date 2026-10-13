package apiserver

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// isOKD checks if the cluster is an OKD cluster by examining the version string
func isOKD(oc *exutil.CLI) (bool, error) {
	current, err := exutil.GetCurrentVersion(context.TODO(), oc.AdminConfig())
	if err != nil {
		return false, err
	}
	return strings.Contains(current, "okd-scos"), nil
}

var _ = g.Describe("[sig-api-machinery][Feature:FeatureGate][OCPFeatureGate:OKD]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("featuregate-okd")

	g.It("should reject OKD featureset on OCP clusters [apigroup:config.openshift.io]", func() {
		// Skip this test on OKD clusters - OKD featureset is allowed on OKD
		okdCluster, err := isOKD(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to determine if cluster is OKD")
		if okdCluster {
			g.Skip("Skipping test on OKD cluster - OKD featureset is allowed on OKD")
		}

		// Get current FeatureGate
		fgClient := oc.AdminConfigClient().ConfigV1().FeatureGates()
		fg, err := fgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get cluster FeatureGate")

		// Skip if current featureset is not Default - we can only modify featureset from Default
		if fg.Spec.FeatureSet != configv1.Default {
			g.Skip("Skipping test - featureset can only be changed when current featureset is Default")
		}

		// Attempt to set OKD featureset using dry-run
		fg.Spec.FeatureSet = configv1.OKD
		_, err = fgClient.Update(context.Background(), fg, metav1.UpdateOptions{
			DryRun: []string{metav1.DryRunAll},
		})

		// Expect validation error on OCP clusters
		o.Expect(err).To(o.HaveOccurred(), "OKD featureset should be rejected on OCP clusters")
		o.Expect(err.Error()).To(o.ContainSubstring("OKD featureset is not supported on OpenShift clusters"))
		o.Expect(k8serrors.IsInvalid(err)).To(o.BeTrue(), "Error should be an Invalid error")
	})
})
