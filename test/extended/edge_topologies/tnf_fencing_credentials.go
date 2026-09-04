// Package edge_topologies tests fencing credential rotation for two-node clusters.
//
// Credential Types:
//   - Pacemaker secrets (openshift-etcd/fencing-credentials-*): Used for STONITH fencing via CEO.
//   - BMC secrets (openshift-machine-api): Used for node provisioning via Baremetal Operator.
//
// Tests validate that invalid pacemaker fencing credentials are correctly rejected without
// impacting live STONITH configuration.
package edge_topologies

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	mathrand "math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	etcdv1 "github.com/openshift/api/etcd/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	fencingHealthTimeout = time.Minute

	// fencingUnhealthyDetectionTimeout bounds how long we wait for a stale fencing credential to surface as a broken agent.
	fencingUnhealthyDetectionTimeout = 5 * time.Minute

	// fencingCredentialPollInterval polls on the minute, every minute.
	fencingCredentialPollInterval = time.Minute
)

func secureRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial] Fencing credentials", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory    *helpers.EtcdClientFactoryImpl
		peerNode, targetNode corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())

		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)

		hasPacemakerCR, availErr := apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if !hasPacemakerCR {
			g.Skip("PacemakerCluster CRD not available")
		}

		nodes, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(nodes.Items).To(o.HaveLen(2), "Expected exactly two nodes for dual-replica fencing test")

		randomIndex := mathrand.Intn(len(nodes.Items))
		peerNode = nodes.Items[randomIndex]
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]

		g.DeferCleanup(func() {
			logFinalClusterStatus([]corev1.Node{peerNode, targetNode})
		})
	})

	g.It("should update fencing credentials and validate stonith health", func() {
		bmcNode := targetNode
		survivedNode := peerNode

		g.By(fmt.Sprintf("Reading current fencing credentials for node %s", bmcNode.Name))
		creds, err := apis.FindPacemakerFencingCredentialsByNodeName(oc, bmcNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to find fencing credentials secret")
		framework.Logf("Found fencing credentials secret %s (address: %s, username: %s)",
			creds.SecretName, creds.Address, creds.Username)

		g.By("Parsing Redfish address from fencing credentials")
		redfishHost, redfishPort, redfishPath, err := apis.ParseRedfishAddress(creds.Address)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to parse Redfish address")
		framework.Logf("Redfish endpoint: host=%s port=%s path=%s", redfishHost, redfishPort, redfishPath)

		// This test rotates the BMC password by editing the sushy-tools htpasswd file on the
		// hypervisor. Real BMC hardware is not exercised in CI, so only sushy-tools is supported.
		if !apis.IsSushyEmulator(redfishPath) {
			g.Skip("test requires sushy-tools BMC emulator")
		}
		if !exutil.HasHypervisorConfig() {
			g.Skip("sushy-tools detected but no hypervisor SSH config available")
		}

		sshCfg := exutil.GetHypervisorConfig()
		o.Expect(sshCfg).ToNot(o.BeNil(), "expected hypervisor config to parse")
		hypervisorSSH := &core.SSHConfig{
			IP:             sshCfg.HypervisorIP,
			User:           sshCfg.SSHUser,
			PrivateKeyPath: sshCfg.PrivateKeyPath,
		}
		hypervisorKnownHosts, err := core.PrepareLocalKnownHostsFile(hypervisorSSH)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to prepare hypervisor known_hosts")
		framework.Logf("Using sushy-tools password change via hypervisor SSH (%s)", hypervisorSSH.IP)

		changeBMCPassword := func(newPw string) error {
			return apis.ChangeSushyToolsPassword(creds.Username, newPw, hypervisorSSH, hypervisorKnownHosts)
		}

		g.By("Verifying PacemakerCluster CR baseline is fully healthy before credential change")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be fully healthy before credential change")

		// Capture the stonith resource name while the agent is still Started.
		g.By(fmt.Sprintf("Identifying the fencing agent for %s", bmcNode.Name))
		stonithResourceName, err := findStonithResourceName(oc, &bmcNode)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to identify the started fencing agent for the target node")
		framework.Logf("Target node %s is fenced by stonith resource %s", bmcNode.Name, stonithResourceName)

		sslInsecure := creds.CertificateVerification == "Disabled"
		originalPassword := creds.Password
		newPassword, err := secureRandomString(32)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to generate a secure BMC password")
		nodeIdentifier := strings.TrimPrefix(creds.SecretName, apis.PacemakerFencingSecretPrefix)

		scriptPath := "/etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-scripts/update-fencing-credentials.sh"
		bashCmd := scriptPath + ` --node "$1" --username "$2" --password "$3" --address "$4"`
		if sslInsecure {
			bashCmd += " --ssl-insecure"
		}

		// sushy-tools uses a single htpasswd file for all BMC endpoints, so changing the
		// password affects both nodes. Update the survived node's stonith device and secret too.
		survivedNodeCreds, err := apis.FindPacemakerFencingCredentialsByNodeName(oc, survivedNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to find survived node fencing credentials")
		survivedNodeIdentifier := strings.TrimPrefix(survivedNodeCreds.SecretName, apis.PacemakerFencingSecretPrefix)
		survivedBashCmd := scriptPath + ` --node "$1" --username "$2" --password "$3" --address "$4"`
		if survivedNodeCreds.CertificateVerification == "Disabled" {
			survivedBashCmd += " --ssl-insecure"
		}
		framework.Logf("Will also update survived node %s credentials (secret: %s)",
			survivedNode.Name, survivedNodeCreds.SecretName)

		bmcPasswordChanged := false
		g.DeferCleanup(func() {
			var cleanupFailed bool

			if bmcPasswordChanged {
				framework.Logf("Restoring original BMC password")
				if restoreErr := changeBMCPassword(originalPassword); restoreErr != nil {
					framework.Logf("Warning: failed to restore BMC password: %v", restoreErr)
					cleanupFailed = true
				}
			} else {
				framework.Logf("Skipping BMC password restore because the password change did not complete")
			}

			scriptPassword := originalPassword
			if bmcPasswordChanged && cleanupFailed {
				scriptPassword = newPassword
			}

			framework.Logf("Re-running update-fencing-credentials.sh with original credentials")
			output, restoreErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, bmcNode.Name, "openshift-etcd",
				"bash", "-c", bashCmd, "update-fencing-credentials",
				nodeIdentifier, creds.Username, scriptPassword, creds.Address)
			if restoreErr != nil {
				framework.Logf("Warning: failed to restore fencing credentials via script: %v\noutput: %s",
					restoreErr, output)
			}

			framework.Logf("Restoring survived node %s fencing credentials", survivedNode.Name)
			survivedOutput, survivedErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
				"bash", "-c", survivedBashCmd, "update-fencing-credentials",
				survivedNodeIdentifier, survivedNodeCreds.Username, scriptPassword, survivedNodeCreds.Address)
			if survivedErr != nil {
				framework.Logf("Warning: failed to restore survived node fencing credentials: %v\noutput: %s",
					survivedErr, survivedOutput)
			}
		})

		g.By(fmt.Sprintf("Changing BMC password on %s", bmcNode.Name))
		err = changeBMCPassword(newPassword)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to change BMC password")
		bmcPasswordChanged = true

		g.By(fmt.Sprintf("Validating new BMC credentials via fence_redfish on %s", bmcNode.Name))
		err = apis.ValidateBMCCredentials(oc, bmcNode.Name, redfishHost, redfishPort, redfishPath,
			creds.Username, newPassword, sslInsecure)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected new BMC credentials to be valid")

		// Confirm pacemaker detects the stale credential before repairing it, so the post-repair health
		// assertion is not vacuous. Stage 1: pcs must report the fencing agent's monitor failing.
		g.By(fmt.Sprintf("Confirming pcs reports a failed monitor action for fencing agent %s", stonithResourceName))
		o.Eventually(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), fencingHealthTimeout)
			defer cancel()
			status, statusErr := services.PcsStatusViaDebug(ctx, oc, survivedNode.Name)
			if statusErr != nil {
				return statusErr
			}
			failed := services.ExtractPcsFailedActions(status)
			if !strings.Contains(failed, stonithResourceName) {
				return fmt.Errorf("pcs has not yet reported a failed action for fencing agent %s; failed actions section: %q", stonithResourceName, failed)
			}
			return nil
		}, fencingUnhealthyDetectionTimeout, fencingCredentialPollInterval).ToNot(o.HaveOccurred(),
			"expected pcs to report the fencing agent's monitor failing after the BMC password changed")

		// Stage 2: the PacemakerCluster CR must report the node's fencing unhealthy and unavailable and the
		// cluster unhealthy in a single snapshot.
		g.By(fmt.Sprintf("Waiting for PacemakerCluster CR to report node %s fencing unhealthy+unavailable and the cluster unhealthy", bmcNode.Name))
		o.Eventually(func() error {
			pc, pcErr := apis.GetPacemakerCluster(oc)
			if pcErr != nil {
				return pcErr
			}
			if err := apis.ExpectNodeFencingUnhealthy(pc, bmcNode.Name); err != nil {
				return err
			}
			if err := apis.ExpectNodeFencingUnavailable(pc, bmcNode.Name); err != nil {
				return err
			}
			return apis.ExpectClusterCondition(pc, etcdv1.ClusterHealthyConditionType, metav1.ConditionFalse)
		}, fencingUnhealthyDetectionTimeout, fencingCredentialPollInterval).ToNot(o.HaveOccurred(),
			"expected PacemakerCluster CR to report the node's fencing unhealthy and unavailable and the cluster unhealthy")

		g.By(fmt.Sprintf("Running update-fencing-credentials.sh on %s with new credentials", bmcNode.Name))
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, bmcNode.Name, "openshift-etcd",
			"bash", "-c", bashCmd, "update-fencing-credentials",
			nodeIdentifier, creds.Username, newPassword, creds.Address)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected update-fencing-credentials.sh to succeed")
		framework.Logf("update-fencing-credentials.sh output:\n%s", output)

		g.By(fmt.Sprintf("Updating survived node %s fencing credentials", survivedNode.Name))
		survivedOutput, survivedErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
			"bash", "-c", survivedBashCmd, "update-fencing-credentials",
			survivedNodeIdentifier, survivedNodeCreds.Username, newPassword, survivedNodeCreds.Address)
		o.Expect(survivedErr).ToNot(o.HaveOccurred(),
			"expected update-fencing-credentials.sh for survived node to succeed")
		framework.Logf("update-fencing-credentials.sh output for survived node:\n%s", survivedOutput)

		// Recovery is not instant, so wait for the baseline after both repairs rather than reading it once.
		g.By("Waiting for PacemakerCluster CR baseline to become fully healthy after the credential update")
		o.Eventually(func() error {
			return apis.ExpectPacemakerBaseline(oc)
		}, healthCheckRecoveryTimeout, fencingCredentialPollInterval).ToNot(o.HaveOccurred(),
			"expected PacemakerCluster to return to full health after credential update")
	})

	g.It("should preserve fencing agent health when BMC secret contains invalid credentials", func() {
		bmcNode := targetNode

		// 1. Verify fencing is healthy.
		g.By("Verifying fencing is healthy before updating the fencing credentials secret")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(),
			"expected PacemakerCluster (including per-node fencing health) to be healthy before the update")

		creds, err := apis.FindPacemakerFencingCredentialsByNodeName(oc, bmcNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to find fencing credentials secret")
		ns := apis.EtcdNamespace
		secretName := creds.SecretName
		originalPassword := []byte(creds.Password)

		defer func() {
			o.Expect(apis.RestorePacemakerFencingPassword(oc, ns, secretName, originalPassword)).ToNot(o.HaveOccurred(),
				fmt.Sprintf("expected to restore original fencing password in %s/%s", ns, secretName))
		}()

		// 2. Change the openshift-etcd fencing secret for one node to a bogus value.
		g.By(fmt.Sprintf("Updating the fencing credentials secret for %s with an invalid password", bmcNode.Name))
		jobBoundary := time.Now()
		o.Expect(apis.UpdatePacemakerFencingPassword(oc, ns, secretName, []byte("invalid-password"))).ToNot(o.HaveOccurred(),
			"expected to update fencing credentials secret")

		// 3 & 4. Wait for the fencing job to be triggered and verify it refused the invalid credentials,
		// leaving live stonith untouched.
		g.By("Verifying the fencing job is triggered and refuses the invalid credentials")
		o.Expect(apis.WaitForPacemakerFencingRejection(oc, ns, bmcNode.Name, jobBoundary, ceoUpdateSetupJobWaitTimeout)).
			ToNot(o.HaveOccurred(), "expected the fencing job to reject the invalid fencing credentials")

		// 5. With the invalid Secret still present (restoration is deferred), both nodes' fencing health
		// must remain preserved: the rejected update must not have disturbed the live stonith config.
		g.By("Verifying both nodes' fencing health is preserved after the rejected update")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(),
			"expected PacemakerCluster (including per-node fencing health) to remain healthy after the rejected update")
	})
})
