// Package apis provides Kubernetes API utilities: CSR approval, BareMetalHost status checks, and Metal3 operations.
package apis

import (
	"context"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ApproveCSRs monitors and approves pending CSRs until timeout or expected count reached.
//
//	approvedCount := ApproveCSRs(oc, 10*time.Minute, 1*time.Minute, 0)
func ApproveCSRs(oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration, expectedCSRCount int) int {
	startTime := time.Now()
	approvedCount := 0

	e2e.Logf("Starting CSR approval monitoring for %v", timeout)

	wait.PollUntilContextTimeout(context.Background(), pollInterval, timeout, true, func(ctx context.Context) (done bool, err error) {
		csrList, err := oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().List(ctx, v1.ListOptions{})
		if err != nil {
			e2e.Logf("Failed to get CSRs: %v", err)
			return false, nil
		}
		pendingCSRs := []string{}
		for _, csr := range csrList.Items {
			if len(csr.Status.Conditions) == 0 {
				pendingCSRs = append(pendingCSRs, csr.Name)
			}
		}

		for _, csrName := range pendingCSRs {
			e2e.Logf("Approving CSR: %s", csrName)

			// Get the CSR
			csr, err := oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().Get(ctx, csrName, v1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to get CSR %s: %v", csrName, err)
				continue
			}

			// Add approval condition
			csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
				Type:           certificatesv1.CertificateApproved,
				Status:         corev1.ConditionTrue,
				Reason:         "AutoApproved",
				Message:        "Approved by two-node test automation",
				LastUpdateTime: v1.Now(),
			})

			// Update the approval
			_, err = oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csrName, csr, v1.UpdateOptions{})
			if err == nil {
				approvedCount++
				e2e.Logf("Approved CSR %s (total approved: %d)", csrName, approvedCount)
			} else {
				e2e.Logf("Failed to approve CSR %s: %v", csrName, err)
			}
		}

		// Continue monitoring until timeout
		if len(pendingCSRs) > 0 {
			e2e.Logf("Approved %d CSRs this iteration, continuing to monitor (elapsed: %v)", len(pendingCSRs), time.Since(startTime))
		}

		// Check if we've reached the expected count
		if expectedCSRCount > 0 && approvedCount >= expectedCSRCount {
			e2e.Logf("All %d expected CSRs approved! (elapsed: %v)", approvedCount, time.Since(startTime))
			return true, nil
		}
		return false, nil
	})

	e2e.Logf("CSR approval monitoring complete: approved %d CSRs in %v", approvedCount, time.Since(startTime))
	return approvedCount
}
