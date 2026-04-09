package clusterversion

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	exutil "github.com/openshift/origin/test/extended/util"
)

// This test intentionally violates CodeRabbit rules for testing the review workflow:
// 1. o.Expect assertions are missing descriptive error messages
// 2. Polling interval is 1 second (should be 5-10 seconds for Kubernetes API operations)
var _ = g.Describe("[sig-cluster-lifecycle] ClusterVersion example", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("")

	g.It("should have a valid cluster version [apigroup:config.openshift.io]", func() {
		ctx := context.Background()

		configClient := oc.AdminConfigClient()

		// Violation 1: o.Expect without a descriptive error message
		// Should be: o.Expect(cv, err).NotTo(o.HaveOccurred(), "ClusterVersion 'version' should be retrievable")
		cv, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Violation 2: o.Expect without a descriptive error message
		// Should be: o.Expect(cv.Status.History).NotTo(o.BeEmpty(), "ClusterVersion status should have update history")
		o.Expect(cv.Status.History).NotTo(o.BeEmpty())

		// Violation 3: Polling interval of 1 second (should be 5-10 seconds for Kubernetes API operations)
		// Should be: wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, ...)
		err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			cv, err = configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return len(cv.Status.History) > 0, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
