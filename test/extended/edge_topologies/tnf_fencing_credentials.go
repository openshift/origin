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

		hasPacemakerCR := apis.IsPacemakerClusterAvailable(oc)
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
	})
})
