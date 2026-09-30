package two_node

import (
	"fmt"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	longRecoveryTimeout = 10 * time.Minute
	crmAttributeName    = "learner_node"
	etcdCloneResource   = "etcd-clone"
)

// getMigrationThreshold returns the explicit threshold, or empty when the resource uses the default.
func getMigrationThreshold(oc *exutil.CLI, nodeName string) (string, error) {
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c",
		"sudo crm_resource --resource etcd --meta --get-parameter migration-threshold 2>/dev/null; echo RC=$?")
	rc := extractValue(output, "RC=")
	if rc == "6" {
		return "", nil
	}
	if err != nil || rc != "0" {
		return "", fmt.Errorf("could not read etcd migration-threshold: %v (output: %s)", err, output)
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "RC=") {
			return line, nil
		}
	}
	return "", nil
}

func extractValue(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func setMigrationThreshold(oc *exutil.CLI, nodeName, value string) error {
	cmd := fmt.Sprintf("sudo crm_resource --resource etcd --meta --set-parameter migration-threshold --parameter-value %s", value)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to set migration-threshold to %s: %v (output: %s)", value, err, output)
	}
	framework.Logf("Set etcd migration-threshold to %s", value)
	return nil
}

// restoreMigrationThreshold restores the original value after a test, removing the override when it was unset.
func restoreMigrationThreshold(oc *exutil.CLI, nodeName, originalValue string) {
	var cmd string
	if originalValue == "" {
		cmd = "sudo crm_resource --resource etcd --meta --delete-parameter migration-threshold 2>/dev/null; true"
	} else {
		cmd = fmt.Sprintf("sudo crm_resource --resource etcd --meta --set-parameter migration-threshold --parameter-value %s 2>/dev/null; true", originalValue)
	}
	if _, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", cmd); err != nil {
		framework.Logf("Warning: failed to restore migration-threshold: %v", err)
	}
}
