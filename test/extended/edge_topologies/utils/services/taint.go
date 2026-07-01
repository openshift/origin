// Package services provides taint/untaint utilities for TNF (Two Node with Fencing) fencing alert validation.
package services

import (
	"context"
	"fmt"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// OutOfServiceTaintKey is the Kubernetes taint key applied to fenced nodes.
	OutOfServiceTaintKey = "node.kubernetes.io/out-of-service"
	// OutOfServiceTaintValue is the taint value indicating the node was shut down.
	OutOfServiceTaintValue = "nodeshutdown"
	// OutOfServiceAnnotationKey tracks which component applied the out-of-service taint.
	OutOfServiceAnnotationKey = "node.kubernetes.io/out-of-service-applied-by"
	// OutOfServiceAnnotationValue identifies pacemaker as the taint originator.
	OutOfServiceAnnotationValue = "pacemaker"

	// TaintScriptLogTag is the syslog tag used by /usr/local/bin/taint-fenced-node.sh.
	TaintScriptLogTag = "taint-fenced-node"
	// UntaintScriptLogTag is the syslog tag used by /usr/local/bin/untaint-fenced-node.sh.
	UntaintScriptLogTag = "untaint-fenced-node"
	// TaintAlertLogTag is the syslog tag used by /var/lib/pacemaker/alerts/tnf-taint-alert.sh.
	TaintAlertLogTag = "tnf-taint-alert"
	// UntaintAlertLogTag is the syslog tag used by /var/lib/pacemaker/alerts/tnf-untaint-alert.sh.
	UntaintAlertLogTag = "tnf-untaint-alert"

	// TaintSuccessLog is the message logged after the taint and annotation are applied.
	TaintSuccessLog = "Successfully tainted and annotated"
	// UntaintSuccessLog is the message logged after the taint and annotation are removed.
	UntaintSuccessLog = "Successfully untainted and removed annotation"
	// TaintAlertFencingLog is the message logged when the taint alert fires on a successful fence.
	TaintAlertFencingLog = "Fencing succeeded"
	// UntaintAlertRejoinLog is the message logged when the untaint alert fires on node rejoin.
	UntaintAlertRejoinLog = "rejoined cluster (membership)"

	// TaintAlertID is the pacemaker alert ID for the taint agent.
	TaintAlertID = "tnf-taint-alert"
	// UntaintAlertID is the pacemaker alert ID for the untaint agent.
	UntaintAlertID = "tnf-untaint-alert"

	// TaintAlertScriptPath is the path to the taint alert script on the node.
	TaintAlertScriptPath = "/var/lib/pacemaker/alerts/tnf-taint-alert.sh"
	// UntaintAlertScriptPath is the path to the untaint alert script on the node.
	UntaintAlertScriptPath = "/var/lib/pacemaker/alerts/tnf-untaint-alert.sh"

	// TaintServiceUnitFmt is the systemd template unit for tainting a fenced node.
	TaintServiceUnitFmt = "taint-node@%s.service"
	// UntaintServiceUnitFmt is the systemd template unit for untainting a recovered node.
	UntaintServiceUnitFmt = "untaint-node@%s.service"
)

// HasOutOfServiceTaint returns true if the node has the out-of-service taint
// with the expected key, value, and NoExecute effect.
func HasOutOfServiceTaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == OutOfServiceTaintKey &&
			taint.Value == OutOfServiceTaintValue &&
			taint.Effect == corev1.TaintEffectNoExecute {
			return true
		}
	}
	return false
}

// HasOutOfServiceAnnotation returns true if the node has the pacemaker
// out-of-service annotation.
func HasOutOfServiceAnnotation(node *corev1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	return node.Annotations[OutOfServiceAnnotationKey] == OutOfServiceAnnotationValue
}

// JournalGrepViaDebug searches the systemd journal on a node for log entries
// matching the given syslog tag and pattern, scoped to entries after sinceTimestamp.
// Returns matching lines or an error if the debug command fails.
//
//	output, err := JournalGrepViaDebug(oc, "master-0", "taint-fenced-node", "Successfully tainted", "2024-01-01 00:00:00")
func JournalGrepViaDebug(oc *exutil.CLI, nodeName, tag, pattern, sinceTimestamp string) (string, error) {
	cmd := fmt.Sprintf(
		`journalctl -t %s --since '%s' --no-pager | grep -F %q | tail -5`,
		tag, sinceTimestamp, pattern)
	return exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
		"bash", "-c", cmd)
}

// GetTimestampViaDebug captures a UTC timestamp from a node, used to scope
// journal log searches to entries emitted after this point.
//
//	ts, err := GetTimestampViaDebug(oc, "master-0")
func GetTimestampViaDebug(oc *exutil.CLI, nodeName string) (string, error) {
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
		"bash", "-c", "date -u '+%Y-%m-%d %H:%M:%S'")
	if err != nil {
		return "", fmt.Errorf("failed to get timestamp from %s: %v", nodeName, err)
	}
	return strings.TrimSpace(output), nil
}

// PcsAlertConfigViaDebug runs "pcs alert config" on a node via a debug container
// and returns the output showing all registered pacemaker alert agents.
//
//	output, err := PcsAlertConfigViaDebug(oc, "master-0")
func PcsAlertConfigViaDebug(oc *exutil.CLI, nodeName string) (string, error) {
	return exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default",
		"bash", "-c", "sudo pcs alert config")
}

// FetchNodeObject retrieves a fresh node object from the Kubernetes API.
// Use this instead of a cached node when checking mutable state like taints or annotations.
//
//	node, err := FetchNodeObject(oc, "master-0")
func FetchNodeObject(oc *exutil.CLI, nodeName string) (*corev1.Node, error) {
	return oc.AdminKubeClient().CoreV1().Nodes().Get(
		context.Background(), nodeName, metav1.GetOptions{})
}

// SystemdServiceJournalGrep searches the systemd journal for a specific unit,
// filtering by pattern, scoped to entries after sinceTimestamp.
func SystemdServiceJournalGrep(oc *exutil.CLI, nodeName, unitName, pattern, sinceTimestamp string) (string, error) {
	cmd := fmt.Sprintf(
		`journalctl -u %s --since '%s' --no-pager | grep -F %q | tail -5`,
		unitName, sinceTimestamp, pattern)
	return exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
		"bash", "-c", cmd)
}

// RemoveTaintAndAnnotation removes the out-of-service taint and pacemaker annotation
// from a node. Retries on conflict up to 3 times. Errors are logged but not returned
// (best-effort cleanup).
func RemoveTaintAndAnnotation(oc *exutil.CLI, nodeName string) {
	for attempt := 0; attempt < 3; attempt++ {
		node, err := FetchNodeObject(oc, nodeName)
		if err != nil {
			e2e.Logf("Cleanup: could not fetch node %s: %v", nodeName, err)
			return
		}

		changed := false

		var filtered []corev1.Taint
		for _, t := range node.Spec.Taints {
			if t.Key == OutOfServiceTaintKey {
				changed = true
				continue
			}
			filtered = append(filtered, t)
		}
		node.Spec.Taints = filtered

		if node.Annotations != nil {
			if _, exists := node.Annotations[OutOfServiceAnnotationKey]; exists {
				delete(node.Annotations, OutOfServiceAnnotationKey)
				changed = true
			}
		}

		if !changed {
			return
		}

		_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(
			context.Background(), node, metav1.UpdateOptions{})
		if err == nil {
			e2e.Logf("Cleanup: removed out-of-service taint and annotation from %s", nodeName)
			return
		}
		e2e.Logf("Cleanup: attempt %d failed to update node %s: %v", attempt+1, nodeName, err)
	}
	e2e.Logf("Cleanup: exhausted retries for node %s", nodeName)
}
