// Package apis provides Kubernetes API utilities: CSR approval, BareMetalHost status checks, and Metal3 operations.
//
// Context convention for polling (wait.PollUntilContextTimeout):
//   - The parent passed to PollUntilContextTimeout is context.Background(); the wait package applies the poll timeout.
//   - The condition callback must use its ctx argument for all API calls so requests are cancelled when the poll ends.
//   - One-off API reads outside a poll (e.g. a final spot check after timeout) use context.WithTimeout(context.Background(), d).
//
// Helpers that only perform a single List/Get (e.g. diagnostics) also use a short WithTimeout on Background(); that is intentional and not a contradiction with the poll rules above.
package apis

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// NodeCSRUsernamePrefix is the CSR spec.username prefix for kubelet node certificates (machine-approver approves these).
const NodeCSRUsernamePrefix = "system:node:"

// NodeBootstrapperUsername is the spec.username for the node-bootstrapper service account (kube-apiserver-client-kubelet CSRs).
const NodeBootstrapperUsername = "system:serviceaccount:openshift-machine-config-operator:node-bootstrapper"

// LogNodeCSRStatus lists CSRs for the given node (spec.username == system:node:<nodeName>), logs pending vs approved
// counts, and returns true if at least one CSR for the node has been approved (e.g. by the machine-approver).
func LogNodeCSRStatus(oc *exutil.CLI, nodeName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	csrList, err := oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().List(ctx, v1.ListOptions{})
	if err != nil {
		e2e.Logf("Failed to list CSRs for node %s: %v", nodeName, err)
		return false
	}
	expectedUsername := NodeCSRUsernamePrefix + nodeName
	var pending, approved []string
	for _, csr := range csrList.Items {
		if csr.Spec.Username != expectedUsername {
			continue
		}
		approvedCond := false
		for _, c := range csr.Status.Conditions {
			if c.Type == certificatesv1.CertificateApproved && c.Status == corev1.ConditionTrue {
				approvedCond = true
				break
			}
		}
		if approvedCond {
			approved = append(approved, csr.Name)
		} else {
			pending = append(pending, csr.Name)
		}
	}
	e2e.Logf("Node %s CSRs: %d pending, %d approved by machine-approver (pending: %s; approved: %s)",
		nodeName, len(pending), len(approved), strings.Join(pending, ","), strings.Join(approved, ","))
	return len(approved) > 0
}

// HasApprovedNodeCSR returns true if the node has at least one approved CSR (machine-approver has approved).
func HasApprovedNodeCSR(oc *exutil.CLI, nodeName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	csrList, err := oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().List(ctx, v1.ListOptions{})
	if err != nil {
		return false
	}
	expectedUsername := NodeCSRUsernamePrefix + nodeName
	for _, csr := range csrList.Items {
		if csr.Spec.Username != expectedUsername {
			continue
		}
		for _, c := range csr.Status.Conditions {
			if c.Type == certificatesv1.CertificateApproved && c.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// ApproveCSRs monitors and approves pending CSRs until timeout or expected count reached.
//
//	approvedCount := ApproveCSRs(oc, 10*time.Minute, 1*time.Minute, 0)
func ApproveCSRs(oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration, expectedCSRCount int) int {
	startTime := time.Now()
	approvedCount := 0

	e2e.Logf("Starting CSR approval monitoring for %v", timeout)

	// Parent is Background; poll timeout is enforced inside PollUntilContextTimeout. Use ctx in the callback for List/Get/UpdateApproval.
	pollErr := wait.PollUntilContextTimeout(context.Background(), pollInterval, timeout, true, func(ctx context.Context) (done bool, err error) {
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

		approvedThisRound := 0
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
				approvedThisRound++
				approvedCount++
				e2e.Logf("Approved CSR %s (total approved: %d)", csrName, approvedCount)
			} else {
				e2e.Logf("Failed to approve CSR %s: %v", csrName, err)
			}
		}

		if len(pendingCSRs) > 0 {
			e2e.Logf("CSR iteration: %d pending, %d approved this round (total approved: %d, elapsed: %v)",
				len(pendingCSRs), approvedThisRound, approvedCount, time.Since(startTime))
		}

		// Check if we've reached the expected count
		if expectedCSRCount > 0 && approvedCount >= expectedCSRCount {
			e2e.Logf("All %d expected CSRs approved! (elapsed: %v)", approvedCount, time.Since(startTime))
			return true, nil
		}
		return false, nil
	})
	if pollErr != nil && !errors.Is(pollErr, wait.ErrWaitTimeout) {
		e2e.Logf("CSR approval monitoring stopped with error: %v", pollErr)
	}

	e2e.Logf("CSR approval monitoring complete: approved %d CSRs in %v", approvedCount, time.Since(startTime))
	return approvedCount
}

// getCSRRequestCommonName decodes the PEM-encoded spec.request and returns the Subject.CommonName, or "" on error.
func getCSRRequestCommonName(request []byte) string {
	block, _ := pem.Decode(request)
	if block == nil {
		return ""
	}
	cr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return ""
	}
	return cr.Subject.CommonName
}

// ApproveNodeBootstrapperCSRsForNode finds Pending kube-apiserver-client-kubelet CSRs from the node-bootstrapper
// that are for the given node (request CN system:node:<nodeName>) and approves them. Returns the number approved.
//
// This is a test workaround for a known product issue: cluster-machine-approver may not approve these CSRs when
// the replacement kubelet reuses an existing Node name, leaving kube-apiserver-client-kubelet CSRs Pending.
// Update the node-replacement spec comment with the OCPBUGS ID once filed.
//
// ctx should be the PollUntilContextTimeout condition context when called from a poll so CSR List/Update respect cancellation.
func ApproveNodeBootstrapperCSRsForNode(ctx context.Context, oc *exutil.CLI, nodeName string) (int, error) {
	csrList, err := oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().List(ctx, v1.ListOptions{})
	if err != nil {
		return 0, err
	}
	expectedCN := NodeCSRUsernamePrefix + nodeName
	approved := 0
	for i := range csrList.Items {
		csr := &csrList.Items[i]
		if csr.Spec.SignerName != certificatesv1.KubeAPIServerClientKubeletSignerName || csr.Spec.Username != NodeBootstrapperUsername {
			continue
		}
		hasApproved := false
		for _, c := range csr.Status.Conditions {
			if c.Type == certificatesv1.CertificateApproved && c.Status == corev1.ConditionTrue {
				hasApproved = true
				break
			}
		}
		if hasApproved {
			continue
		}
		cn := getCSRRequestCommonName(csr.Spec.Request)
		if cn != expectedCN {
			continue
		}
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:           certificatesv1.CertificateApproved,
			Status:         corev1.ConditionTrue,
			Reason:         "TNFNodeReplacement",
			Message:        "Approved by TNF node replacement test (workaround for machine-approver same-name replacement bug)",
			LastUpdateTime: v1.Now(),
		})
		_, err = oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, v1.UpdateOptions{})
		if err != nil {
			e2e.Logf("Failed to approve node-bootstrapper CSR %s for node %s: %v", csr.Name, nodeName, err)
			continue
		}
		approved++
		e2e.Logf("Approved node-bootstrapper CSR %s for node %s", csr.Name, nodeName)
	}
	return approved, nil
}

// nodeIsReady returns true if the node exists and NodeReady is True.
func nodeIsReady(ctx context.Context, oc *exutil.CLI, nodeName string) bool {
	getCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(getCtx, nodeName, v1.GetOptions{})
	if err != nil {
		return false
	}
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// WaitForAndApproveNodeBootstrapperCSR waits until a Pending kube-apiserver-client-kubelet CSR from the
// node-bootstrapper for the given node appears and approves it, or until the node becomes Ready.
// Success if: (1) test approves at least one matching CSR, or (2) node is Ready (CSR already approved by
// machine-approver, or race where approver wins mid-poll). Returns an error only on timeout when the node
// is still not Ready and no CSR was approved by this wait.
//
// See ApproveNodeBootstrapperCSRsForNode for the known machine-approver / same-node-name replacement bug.
func WaitForAndApproveNodeBootstrapperCSR(oc *exutil.CLI, nodeName string, timeout time.Duration) error {
	e2e.Logf("Waiting up to %v for node-bootstrapper CSR for node %s (or node Ready if cert already OK)", timeout, nodeName)
	var approved int
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		if nodeIsReady(ctx, oc, nodeName) {
			return true, nil
		}
		n, err := ApproveNodeBootstrapperCSRsForNode(ctx, oc, nodeName)
		if err != nil {
			e2e.Logf("ApproveNodeBootstrapperCSRsForNode: %v", err)
			return false, nil
		}
		approved += n
		if approved > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		spotCtx, spotCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer spotCancel()
		if nodeIsReady(spotCtx, oc, nodeName) {
			e2e.Logf("Node %s is Ready after CSR wait window; treating step as success (kubelet client cert OK)", nodeName)
			return nil
		}
		return err
	}
	if approved > 0 {
		e2e.Logf("Approved %d node-bootstrapper CSR(s) for node %s", approved, nodeName)
	} else {
		e2e.Logf("Node %s is Ready without test approving a bootstrapper CSR (machine-approver pre-approved or mid-poll race)", nodeName)
	}
	return nil
}
