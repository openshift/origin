package utils

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Pacemaker-related constants
const (
	pcsResourceDebugStart = "sudo OCF_RESKEY_CRM_meta_notify_start_resource='etcd' pcs resource debug-start etcd --full"

	// PCS commands
	pcsClusterNodeRemove = "sudo pcs cluster node remove %s"
	pcsClusterNodeAdd = "sudo pcs cluster node add %s addr=%s --start --enable"
)

// restoreEtcdQuorumOnSurvivor restores etcd quorum on the surviving node
func RestoreEtcdQuorumOnSurvivor(nodeName, nodeIP, string, oc *exutil.CLI, sshConfig *SSHConfig) {
	g.GinkgoT().Logf("Restoring etcd quorum on surviving node: %s", nodeName)

	// SSH to hypervisor, then to surviving node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> surviving node
	chainedCommand := fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, survivingNodeKnownHostsPath, nodeIP, pcsResourceDebugStart)

	output, err := ExecuteSSHCommand(chainedCommand, sshConfig)
	if err != nil {
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Failed to restore etcd quorum on %s: %v, output: %s", nodeName, err, output))
	}

	// Log pacemaker status to check if etcd has been started on the survivor
	pcsStatusOutput, err := ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "sudo pcs status"`, survivingNodeKnownHostsPath, nodeIP), sshConfig)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to get pacemaker status on survivor %s: %v", nodeIP, err)
	} else {
		g.GinkgoT().Logf("[DEBUG] Pacemaker status on survivor %s:\n%s", nodeIP, pcsStatusOutput)
	}

	g.GinkgoT().Logf("Successfully restored etcd quorum on surviving node: %s", nodeName)
}

// restorePacemakerCluster restores the pacemaker cluster configuration
func RestoreEtcdRevision(nodeName, nodeIP string, sshConfig *SSHConfig, oc *exutil.CLI) {
	// Create the revision.json file on the new node using constants
	revisionScript := fmt.Sprintf(`
		%s
		%s
		echo '%s' | sudo tee -a /var/lib/etcd/revision.json
		%s
	`, mkdirEtcdDir, fmt.Sprintf(chmodEtcdDir, etcdDirPermissions), fmt.Sprintf(revisionJSONTemplate, nodeIP, etcdPort), fmt.Sprintf(chmodRevisionJSON, etcdFilePermissions))
	ExecuteSSHScript(nodeName, nodeIP, "revision-setup", revisionScript, sshConfig)

	// Redeploy etcd with a force redeployment reason
	forceRedeploymentReason := fmt.Sprintf("recovery-%s", time.Now().Format(time.RFC3339Nano))
	_, err := oc.AsAdmin().Run("patch").Args("etcd", "cluster", "-p", fmt.Sprintf(`{"spec": {"forceRedeploymentReason": "%s"}}`, forceRedeploymentReason), "--type=merge").Output()
	o.Expect(err).To(o.BeNil(), "Expected to redeploy etcd without error")
}

func CycleRemovedNode(targetNodeName, targetNodeIP, survivingNodeName, survivingNodeIP string, sshConfig *SSHConfig) {
	// Remove and re-add the node in pacemaker using constants
	pcsScript := fmt.Sprintf(`
		%s
		%s
	`, fmt.Sprintf(pcsClusterNodeRemove, targetNodeName), fmt.Sprintf(pcsClusterNodeAdd, targetNodeName, targetNodeIP))
	ExecuteSSHScript(survivingNodeName, survivingNodeIP, "pcs-setup", pcsScript, sshConfig)
}

func DeleteNodeJobs(authJobName, afterSetupJobName string, oc *exutil.CLI) {
	// Delete the old tnf-auth-job using dynamic name
	_, err := oc.AsAdmin().Run("delete").Args("job", authJobName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete old %s without error", authJobName)

	// Delete the old tnf-after-setup-job using dynamic name
	_, err = oc.AsAdmin().Run("delete").Args("job", afterSetupJobName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete old %s without error", afterSetupJobName)
}

// Global variables that need to be accessible from utils
var (
	survivingNodeKnownHostsPath string
	etcdNamespace = "openshift-etcd"
	mkdirEtcdDir = "sudo mkdir /var/lib/etcd"
	chmodEtcdDir = "sudo chmod %o /var/lib/etcd"
	revisionJSONTemplate = `{"clusterId":"0","raftIndex":{"https://%s:%s":0},"maxRaftIndex":0,"created":""}`
	etcdDirPermissions = 0766
	etcdFilePermissions = 0644
	etcdPort = "2379"
	chmodRevisionJSON = "sudo chmod %o /var/lib/etcd/revision.json"
)


// retryOperationWithTimeout retries an operation until it succeeds or times out
func retryOperationWithTimeout(operation func() error, timeout, pollInterval time.Duration, operationName string) error {
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		err := operation()
		if err == nil {
			g.GinkgoT().Logf("Operation %s succeeded after %v", operationName, time.Since(startTime))
			return nil
		}

		g.GinkgoT().Logf("Operation %s failed, retrying in %v: %v", operationName, pollInterval, err)
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("operation %s failed after %v timeout", operationName, timeout)
}

// Getter function for survivingNodeKnownHostsPath
func GetSurvivingNodeKnownHostsPath() string {
	return survivingNodeKnownHostsPath
}