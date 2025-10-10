// Package apis provides Kubernetes API utilities: CSR approval, BareMetalHost status checks, and Metal3 operations.
package apis

import (
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/klog/v2"
)

// CSRListResponse represents the response from 'oc get csr -o json'
type CSRListResponse struct {
	APIVersion string    `json:"apiVersion"`
	Items      []CSRItem `json:"items"`
	Kind       string    `json:"kind"`
}

// CSRItem represents a single CSR in the list
type CSRItem struct {
	Metadata CSRMetadata `json:"metadata"`
	Status   CSRStatus   `json:"status"`
}

// CSRMetadata represents the metadata of a CSR
type CSRMetadata struct {
	Name              string            `json:"name"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// CSRStatus represents the status of a CSR
type CSRStatus struct {
	Conditions []CSRCondition `json:"conditions,omitempty"`
}

// CSRCondition represents a condition in a CSR status
type CSRCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ApproveCSRs monitors and approves pending CSRs until timeout or expected count reached.
//
//	approvedCount := ApproveCSRs(oc, 10*time.Minute, 1*time.Minute, 0)
func ApproveCSRs(oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration, expectedCSRCount int) int {
	startTime := time.Now()
	approvedCount := 0

	klog.V(2).Infof("Starting CSR approval monitoring for %v", timeout)

	for time.Since(startTime) < timeout {
		// Get pending CSRs
		csrOutput, err := oc.AsAdmin().Run("get").Args("csr", "-o", "json").Output()
		if err != nil {
			klog.V(4).Infof("Failed to get CSRs: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Parse the JSON response
		var csrList CSRListResponse
		if err := utils.UnmarshalJSON(csrOutput, &csrList); err != nil {
			klog.Warningf("Failed to parse CSR list JSON: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Find CSRs that need approval (no conditions means pending)
		pendingCSRs := []string{}
		for _, csr := range csrList.Items {
			if len(csr.Status.Conditions) == 0 {
				pendingCSRs = append(pendingCSRs, csr.Metadata.Name)
			}
		}

		// Approve pending CSRs
		for _, csrName := range pendingCSRs {
			klog.V(2).Infof("Approving CSR: %s", csrName)
			_, err = oc.AsAdmin().Run("adm").Args("certificate", "approve", csrName).Output()
			if err == nil {
				approvedCount++
				klog.V(2).Infof("Approved CSR %s (total approved: %d)", csrName, approvedCount)
			} else {
				klog.V(4).Infof("Failed to approve CSR %s: %v", csrName, err)
			}
		}

		// Continue monitoring until timeout
		if len(pendingCSRs) > 0 {
			klog.V(4).Infof("Approved %d CSRs this iteration, continuing to monitor (elapsed: %v)", len(pendingCSRs), time.Since(startTime))
		}

		// Check if we've reached the expected count
		if expectedCSRCount > 0 && approvedCount >= expectedCSRCount {
			klog.V(2).Infof("All %d expected CSRs approved! (elapsed: %v)", approvedCount, time.Since(startTime))
			return approvedCount
		}

		time.Sleep(pollInterval)
	}

	klog.V(2).Infof("CSR approval monitoring complete: approved %d CSRs in %v", approvedCount, time.Since(startTime))
	return approvedCount
}
