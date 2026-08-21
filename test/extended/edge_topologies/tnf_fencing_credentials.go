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
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	fencingHealthTimeout = time.Minute
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
		creds, err := apis.FindFencingCredentialsByNodeName(oc, bmcNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to find fencing credentials secret")
		framework.Logf("Found fencing credentials secret %s (address: %s, username: %s)",
			creds.SecretName, creds.Address, creds.Username)

		g.By("Parsing Redfish address from fencing credentials")
		redfishHost, redfishPort, redfishPath, err := apis.ParseRedfishAddress(creds.Address)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to parse Redfish address")
		framework.Logf("Redfish endpoint: host=%s port=%s path=%s", redfishHost, redfishPort, redfishPath)

		isSushy := apis.IsSushyEmulator(redfishPath)
		var hypervisorSSH *core.SSHConfig
		var hypervisorKnownHosts string
		if isSushy {
			if !exutil.HasHypervisorConfig() {
				g.Skip("sushy-tools detected but no hypervisor SSH config available")
			}
			sshCfg := exutil.GetHypervisorConfig()
			o.Expect(sshCfg).ToNot(o.BeNil(), "expected hypervisor config to parse")
			hypervisorSSH = &core.SSHConfig{
				IP:             sshCfg.HypervisorIP,
				User:           sshCfg.SSHUser,
				PrivateKeyPath: sshCfg.PrivateKeyPath,
			}
			var khErr error
			hypervisorKnownHosts, khErr = core.PrepareLocalKnownHostsFile(hypervisorSSH)
			o.Expect(khErr).ToNot(o.HaveOccurred(), "expected to prepare hypervisor known_hosts")
			framework.Logf("Using sushy-tools password change via hypervisor SSH (%s)", hypervisorSSH.IP)
		}

		changeBMCPassword := func(currentPw, newPw string) error {
			if isSushy {
				return apis.ChangeSushyToolsPassword(creds.Username, newPw, hypervisorSSH, hypervisorKnownHosts)
			}
			return apis.ChangeBMCPasswordViaRedfish(oc, bmcNode.Name, redfishHost, redfishPort,
				creds.Username, currentPw, newPw)
		}

		hasPacemakerCR, availErr := apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if hasPacemakerCR {
			g.By("Verifying PacemakerCluster CR is healthy before credential change")
			pc, pcErr := apis.GetPacemakerCluster(oc)
			o.Expect(pcErr).ToNot(o.HaveOccurred(), "expected to get PacemakerCluster CR")
			o.Expect(apis.ExpectClusterHealthy(pc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be healthy before credential change")
			o.Expect(apis.ExpectNodeFencingHealthy(pc, bmcNode.Name)).ToNot(o.HaveOccurred(),
				"expected fencing to be healthy for %s before credential change", bmcNode.Name)
		} else {
			framework.Logf("PacemakerCluster CRD not available, skipping CR health checks")
		}

		sslInsecure := creds.CertificateVerification == "Disabled"
		originalPassword := creds.Password
		newPassword, err := secureRandomString(32)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to generate a secure BMC password")
		nodeIdentifier := strings.TrimPrefix(creds.SecretName, "fencing-credentials-")

		scriptPath := "/etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-scripts/update-fencing-credentials.sh"
		bashCmd := scriptPath + ` --node "$1" --username "$2" --password "$3" --address "$4"`
		if sslInsecure {
			bashCmd += " --ssl-insecure"
		}

		// On sushy-tools, a single htpasswd file serves all BMC endpoints, so changing
		// the password affects all nodes. Fetch the survived node's credentials so we
		// can update its stonith device and secret in lockstep.
		var survivedNodeCreds *apis.FencingCredentials
		var survivedNodeIdentifier string
		var survivedBashCmd string
		if isSushy {
			survivedNodeCreds, err = apis.FindFencingCredentialsByNodeName(oc, survivedNode.Name)
			o.Expect(err).ToNot(o.HaveOccurred(), "expected to find survived node fencing credentials")
			survivedNodeIdentifier = strings.TrimPrefix(survivedNodeCreds.SecretName, "fencing-credentials-")
			survivedBashCmd = scriptPath + ` --node "$1" --username "$2" --password "$3" --address "$4"`
			if survivedNodeCreds.CertificateVerification == "Disabled" {
				survivedBashCmd += " --ssl-insecure"
			}
			framework.Logf("sushy-tools: will also update survived node %s credentials (secret: %s)",
				survivedNode.Name, survivedNodeCreds.SecretName)
		}

		bmcPasswordChanged := false
		g.DeferCleanup(func() {
			var cleanupFailed bool

			if bmcPasswordChanged {
				framework.Logf("Restoring original BMC password")
				if restoreErr := changeBMCPassword(newPassword, originalPassword); restoreErr != nil {
					fmt.Fprintf(g.GinkgoWriter, "Warning: failed to restore BMC password: %v\n", restoreErr)
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
				fmt.Fprintf(g.GinkgoWriter, "Warning: failed to restore fencing credentials via script: %v\noutput: %s\n",
					restoreErr, output)
			}

			if isSushy {
				framework.Logf("Restoring survived node %s fencing credentials (sushy-tools shares credentials)", survivedNode.Name)
				survivedOutput, survivedErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
					"bash", "-c", survivedBashCmd, "update-fencing-credentials",
					survivedNodeIdentifier, survivedNodeCreds.Username, scriptPassword, survivedNodeCreds.Address)
				if survivedErr != nil {
					fmt.Fprintf(g.GinkgoWriter, "Warning: failed to restore survived node fencing credentials: %v\noutput: %s\n",
						survivedErr, survivedOutput)
				}
			}
		})

		g.By(fmt.Sprintf("Changing BMC password on %s", bmcNode.Name))
		err = changeBMCPassword(originalPassword, newPassword)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to change BMC password")
		bmcPasswordChanged = true

		g.By(fmt.Sprintf("Validating new BMC credentials via fence_redfish on %s", bmcNode.Name))
		err = apis.ValidateBMCCredentials(oc, bmcNode.Name, redfishHost, redfishPort, redfishPath,
			creds.Username, newPassword, sslInsecure)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected new BMC credentials to be valid")

		g.By(fmt.Sprintf("Running update-fencing-credentials.sh on %s with new credentials", bmcNode.Name))
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, bmcNode.Name, "openshift-etcd",
			"bash", "-c", bashCmd, "update-fencing-credentials",
			nodeIdentifier, creds.Username, newPassword, creds.Address)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected update-fencing-credentials.sh to succeed")
		framework.Logf("update-fencing-credentials.sh output:\n%s", output)

		if isSushy {
			g.By(fmt.Sprintf("Updating survived node %s fencing credentials (sushy-tools shares credentials)", survivedNode.Name))
			survivedOutput, survivedErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
				"bash", "-c", survivedBashCmd, "update-fencing-credentials",
				survivedNodeIdentifier, survivedNodeCreds.Username, newPassword, survivedNodeCreds.Address)
			o.Expect(survivedErr).ToNot(o.HaveOccurred(),
				"expected update-fencing-credentials.sh for survived node to succeed")
			framework.Logf("update-fencing-credentials.sh output for survived node:\n%s", survivedOutput)
		}

		g.By("Validating pacemaker health after credential update")
		ctx, cancel := context.WithTimeout(context.Background(), fencingHealthTimeout)
		defer cancel()
		pcsOutput, err := services.PcsStatusViaDebug(ctx, oc, bmcNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected pcs status to succeed")
		failedActions := services.ExtractPcsFailedActions(pcsOutput)
		o.Expect(failedActions).To(o.BeEmpty(), "expected no failed pacemaker resource actions after credential update")

		g.By("Ensuring etcd members remain healthy after fencing credentials update")
		o.Eventually(func() error {
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivedNode.Name); err != nil {
				return err
			}
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, bmcNode.Name); err != nil {
				return err
			}
			return nil
		}, fencingHealthTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"etcd members should be healthy after fencing credentials update")

		if hasPacemakerCR {
			g.By("Verifying PacemakerCluster CR remains healthy after credential update")
			o.Eventually(func() error {
				pc, pcErr := apis.GetPacemakerCluster(oc)
				if pcErr != nil {
					return pcErr
				}
				if pcErr = apis.ExpectClusterHealthy(pc); pcErr != nil {
					return pcErr
				}
				if pcErr = apis.ExpectNodeFencingHealthy(pc, bmcNode.Name); pcErr != nil {
					return pcErr
				}
				return apis.ExpectNodeFencingHealthy(pc, survivedNode.Name)
			}, fencingHealthTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
				"expected PacemakerCluster to remain healthy after credential update")
		}
		g.By("Verifying PacemakerHealthCheckDegraded is not set after credential update")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, fencingHealthTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should not be set after credential update")
	})

	g.It("should not degrade when fencing is at risk but still available", func() {
		g.By("Finding a fencing agent to unmanage on the target node")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pcsOutput, err := services.PcsStatusViaDebug(ctx, oc, peerNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected pcs status to succeed")

		var stonithResourceName string
		for _, line := range strings.Split(pcsOutput, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.Contains(trimmed, "fence_") || !strings.Contains(trimmed, "Started") {
				continue
			}
			// Resource lines are bullet-prefixed (e.g. "* master-1_redfish\t(stonith:fence_redfish):\t Started master-1"),
			// so fields[0] is the "*" bullet and the resource name is fields[1]. Stonith resources are
			// named "<targetedNodeName>_redfish" — the node they fence, NOT the node they currently run
			// on (pcs may run either node's fencing agent on either surviving node). Matching on the
			// resource name itself avoids picking the wrong agent when both agents happen to be Started
			// on the same node (a valid pacemaker placement, not tied to which node they fence).
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				continue
			}
			resourceName := strings.TrimSuffix(fields[1], ":")
			if strings.HasPrefix(resourceName, targetNode.Name) {
				stonithResourceName = resourceName
				break
			}
		}
		if stonithResourceName == "" {
			g.Skip("Could not identify a started fencing agent for the target node — skipping negative test")
		}
		framework.Logf("Selected fencing agent to unmanage: %s", stonithResourceName)

		g.By(fmt.Sprintf("Unmanaging fencing agent %s to create FencingHealthy=False, FencingAvailable=True state", stonithResourceName))
		unmanageCmd := fmt.Sprintf("sudo pcs resource meta %s is-managed=false", stonithResourceName)
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, peerNode.Name, "default", "bash", "-c", unmanageCmd)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to unmanage fencing agent")

		g.DeferCleanup(func() {
			framework.Logf("Restoring management of fencing agent %s", stonithResourceName)
			manageCmd := fmt.Sprintf("sudo pcs resource meta %s is-managed=true 2>/dev/null; true", stonithResourceName)
			if _, restoreErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, peerNode.Name, "default", "bash", "-c", manageCmd); restoreErr != nil {
				fmt.Fprintf(g.GinkgoWriter, "Warning: failed to re-manage fencing agent: %v\n", restoreErr)
			}
		})

		pcAvailable, availErr := apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if pcAvailable {
			g.By("Waiting for PacemakerCluster to report FencingHealthy=False, FencingAvailable=True for target node")
			o.Eventually(func() error {
				pc, pcErr := apis.GetPacemakerCluster(oc)
				if pcErr != nil {
					return pcErr
				}
				if err := apis.ExpectNodeFencingUnhealthy(pc, targetNode.Name); err != nil {
					return err
				}
				return apis.ExpectNodeFencingAvailable(pc, targetNode.Name)
			}, 2*time.Minute, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
				"expected fencing to be at-risk (FencingHealthy=False) but still available (FencingAvailable=True) for target node")
		} else {
			framework.Logf("PacemakerCluster CRD not available, skipping CR fencing-state checks")
		}

		g.By("Verifying PacemakerHealthCheckDegraded stays False during fencing warning state")
		o.Consistently(func() error {
			return apis.ExpectPacemakerHealthCheckNotDegraded(oc)
		}, 3*time.Minute, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"PacemakerHealthCheckDegraded should stay False when fencing is at risk but still available")

		g.By(fmt.Sprintf("Re-managing fencing agent %s", stonithResourceName))
		manageCmd := fmt.Sprintf("sudo pcs resource meta %s is-managed=true", stonithResourceName)
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, peerNode.Name, "default", "bash", "-c", manageCmd)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to re-manage fencing agent")

		g.By("Verifying cluster returns to fully healthy state")
		pcAvailable, availErr = apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if pcAvailable {
			o.Eventually(func() error {
				pc, pcErr := apis.GetPacemakerCluster(oc)
				if pcErr != nil {
					return pcErr
				}
				return apis.ExpectClusterHealthy(pc)
			}, fencingHealthTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
				"expected PacemakerCluster to be healthy after re-managing fencing agent")
		}
	})

	g.It("should degrade when a node's fencing agent is completely unavailable", func() {
		g.By("Finding a fencing agent to disable on the target node")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pcsOutput, err := services.PcsStatusViaDebug(ctx, oc, peerNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected pcs status to succeed")

		var stonithResourceName string
		for _, line := range strings.Split(pcsOutput, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.Contains(trimmed, "fence_") || !strings.Contains(trimmed, "Started") {
				continue
			}
			// Resource lines are bullet-prefixed (e.g. "* master-1_redfish\t(stonith:fence_redfish):\t Started master-1"),
			// so fields[0] is the "*" bullet and the resource name is fields[1]. Stonith resources are
			// named "<targetedNodeName>_redfish" — the node they fence, NOT the node they currently run
			// on (pcs may run either node's fencing agent on either surviving node). Matching on the
			// resource name itself avoids picking the wrong agent when both agents happen to be Started
			// on the same node (a valid pacemaker placement, not tied to which node they fence).
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				continue
			}
			resourceName := strings.TrimSuffix(fields[1], ":")
			if strings.HasPrefix(resourceName, targetNode.Name) {
				stonithResourceName = resourceName
				break
			}
		}
		if stonithResourceName == "" {
			g.Skip("Could not identify a started fencing agent for the target node — skipping fencing disable test")
		}
		framework.Logf("Selected fencing agent to disable: %s", stonithResourceName)

		g.By(fmt.Sprintf("Disabling fencing agent %s to make fencing completely unavailable for %s", stonithResourceName, targetNode.Name))
		// pcs rejects `pcs resource disable/enable` for stonith resources ("This command
		// does not accept stonith resources") — must use `pcs stonith disable/enable` instead.
		disableCmd := fmt.Sprintf("sudo pcs stonith disable %s", stonithResourceName)
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, peerNode.Name, "default", "bash", "-c", disableCmd)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to disable fencing agent")

		g.DeferCleanup(func() {
			framework.Logf("Re-enabling fencing agent %s", stonithResourceName)
			enableCmd := fmt.Sprintf("sudo pcs stonith enable %s 2>/dev/null; true", stonithResourceName)
			if _, enableErr := exutil.DebugNodeRetryWithOptionsAndChroot(oc, peerNode.Name, "default", "bash", "-c", enableCmd); enableErr != nil {
				fmt.Fprintf(g.GinkgoWriter, "Warning: failed to re-enable fencing agent: %v\n", enableErr)
			}
		})

		g.By("Waiting for PacemakerHealthCheckDegraded=True due to fencing unavailable")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "fencing unavailable", healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when fencing is completely unavailable")

		pcAvailable, availErr := apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if pcAvailable {
			g.By("Verifying PacemakerCluster CR shows FencingAvailable=False for target node")
			o.Eventually(func() error {
				pc, pcErr := apis.GetPacemakerCluster(oc)
				if pcErr != nil {
					return pcErr
				}
				return apis.ExpectNodeFencingUnavailable(pc, targetNode.Name)
			}, 2*time.Minute, 10*time.Second).ShouldNot(o.HaveOccurred(),
				"expected FencingAvailable=False on PacemakerCluster CR for target node")
		} else {
			framework.Logf("PacemakerCluster CRD not available, skipping CR FencingAvailable=False check")
		}

		g.By(fmt.Sprintf("Re-enabling fencing agent %s", stonithResourceName))
		enableCmd := fmt.Sprintf("sudo pcs stonith enable %s", stonithResourceName)
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, peerNode.Name, "default", "bash", "-c", enableCmd)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to re-enable fencing agent")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear after re-enabling fencing")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after fencing is re-enabled")

		pcAvailable, availErr = apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if pcAvailable {
			g.By("Verifying cluster returns to fully healthy state")
			o.Eventually(func() error {
				pc, pcErr := apis.GetPacemakerCluster(oc)
				if pcErr != nil {
					return pcErr
				}
				if err := apis.ExpectClusterHealthy(pc); err != nil {
					return err
				}
				if err := apis.ExpectNodeFencingHealthy(pc, targetNode.Name); err != nil {
					return err
				}
				return apis.ExpectNodeFencingAvailable(pc, targetNode.Name)
			}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
				"expected PacemakerCluster to be healthy with FencingHealthy=True and FencingAvailable=True after re-enabling agent")
		} else {
			framework.Logf("PacemakerCluster CRD not available, skipping final CR health check")
		}
	})
})
