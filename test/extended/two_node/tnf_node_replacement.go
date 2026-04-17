// TNF control-plane node replacement e2e: Ginkgo spec only (helpers live in tnf_node_replacement_*.go).
//
// Typical duration (informal): A successful run of this spec usually completes in about 40 minutes; the latest
// lab run reported 40m16s for the spec and ~44m for the full openshift-tests invocation (teardown/overhead).
// Expect roughly 35–45 minutes (±5 minutes) depending on cluster load, Ironic, and etcd recovery timing.
package two_node

import (
	"context"
	"flag"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][Suite:openshift/two-node][Slow][Serial][Disruptive][Requires:HypervisorSSHConfig] TNF", func() {
	var (
		testConfig TNFTestConfig
		oc         = exutil.NewCLIWithoutNamespace("").AsAdmin()
	)
	defer g.GinkgoRecover()
	g.BeforeEach(func() {
		// Set klog verbosity to 2 for detailed logging if not already set by user
		if vFlag := flag.Lookup("v"); vFlag != nil {
			// Only set if the flag hasn't been explicitly set by the user (still has default value)
			if vFlag.Value.String() == "0" {
				if err := flag.Set("v", "2"); err != nil {
					e2e.Logf("WARNING: Failed to set klog verbosity: %v", err)
				} else {
					e2e.Logf("Set klog verbosity to 2 for detailed logging")
				}
			} else {
				e2e.Logf("Using user-specified klog verbosity: %s", vFlag.Value.String())
			}
		}

		// Skip if cluster topology doesn't match
		utils.SkipIfNotTopology(oc, configv1.DualReplicaTopologyMode)

		// Create etcd client factory for validation
		etcdClientFactory := helpers.NewEtcdClientFactory(oc.KubeClient())

		// Check cluster health (includes etcd validation and node count) before running disruptive test
		// If unhealthy, this will skip and record for meta test to fail the suite
		e2e.Logf("Checking cluster health before running disruptive node replacement test")
		beStart := time.Now()
		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)
		e2e.Logf("Cluster health check passed: all operators healthy and all nodes ready")
		e2e.Logf("[stage timing] BeforeEach cluster health precondition: %v (SkipIfClusterIsNotHealthy: nodes+CO via IsClusterHealthyWithTimeout cap %v each; etcd/CEO checks cap %v; CO monitor poll %v inside IsClusterHealthyWithTimeout)",
			time.Since(beStart), utils.PreconditionClusterHealthyTimeout, utils.PreconditionEtcdHealthyTimeout, utils.FiveSecondPollInterval)

		beStart = time.Now()
		setupTestEnvironment(&testConfig, oc)
		e2e.Logf("[stage timing] BeforeEach setupTestEnvironment: %v (no poll timeout; hypervisor/node discovery)", time.Since(beStart))
		testConfig.Execution.SetupCompleted = true
	})

	g.AfterEach(func() {
		// Best-effort cleanup only: restore from backup when available, tidy SSH known_hosts, and poll for
		// healthy cluster operators. We log warnings on failure but do not fail the spec—recovery may be
		// incomplete if the cluster or hypervisor is left in a bad state.
		// Short-circuit if BeforeEach skipped before test setup completed
		// (e.g., due to precondition failures like unhealthy cluster)
		if !testConfig.Execution.SetupCompleted {
			e2e.Logf("Test was skipped before setup completed, skipping AfterEach cleanup")
			return
		}

		// Always attempt recovery if we have backup data
		if testConfig.Execution.GlobalBackupDir != "" {
			g.By("Attempting cluster recovery from backup")
			aeRecoverStart := time.Now()
			if recErr := recoverClusterFromBackup(&testConfig, oc); recErr != nil {
				e2e.Logf("WARNING: Cluster recovery stopped early: %v", recErr)
			}
			e2e.Logf("[stage timing] AfterEach recoverClusterFromBackup: %v (no poll timeout)", time.Since(aeRecoverStart))
		}
		// Clean up target node known_hosts only if it was created (after reprovisioning)
		if testConfig.TargetNode.KnownHostsPath != "" {
			core.CleanupRemoteKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.TargetNode.KnownHostsPath)
		}
		core.CleanupRemoteKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		core.CleanupLocalKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)

		// Wait for cluster operators to become healthy (regardless of test success/failure)
		g.By("Waiting for cluster operators to become healthy")
		e2e.Logf("Waiting up to %v for all cluster operators to become healthy", clusterOperatorStabilizationTimeout)
		aeCOStart := time.Now()
		err := core.PollUntil(func() (bool, error) {
			if err := utils.IsClusterHealthyWithTimeout(oc, utils.ThirtySecondPollInterval); err != nil {
				e2e.Logf("Cluster not yet healthy: %v", err)
				return false, nil
			}
			e2e.Logf("All cluster operators are healthy")
			return true, nil
		}, clusterOperatorStabilizationTimeout, utils.ThirtySecondPollInterval, "cluster operators to become healthy")
		e2e.Logf("[stage timing] AfterEach cluster operators healthy poll: %v (timeout cap: %v, poll: %v)", time.Since(aeCOStart), clusterOperatorStabilizationTimeout, utils.ThirtySecondPollInterval)
		if err != nil {
			e2e.Logf("WARNING: Cluster operators did not become healthy within %v: %v", clusterOperatorStabilizationTimeout, err)
			e2e.Logf("This may indicate the cluster is still recovering from the disruptive test")
		}
	})

	g.It("cluster recovers when a permanently failed node needing manual recovery is replaced", func() {
		initNodeReplacementLogDir(&testConfig)
		defer startCEOLogCapture(testConfig.Execution.NodeReplacementLogDir)()

		stageStart := time.Now()

		g.By("Backing up the target node's configuration")
		backupDir := backupTargetNodeConfiguration(&testConfig, oc)
		testConfig.Execution.GlobalBackupDir = backupDir // Store globally for recovery
		e2e.Logf("[stage timing] Backing up target node configuration: %v (no poll timeout; hypervisor backup)", time.Since(stageStart))
		stageStart = time.Now()

		g.By("Tracing OVN chassis vs host OVS (node-chassis-id follows local OVS when the Node name is reused)")
		logPreDestroyOVNChassisTrace(&testConfig, oc)
		e2e.Logf("[stage timing] Pre-destroy OVS identity (SSH + API): %v", time.Since(stageStart))
		stageStart = time.Now()

		g.By("Destroying the target VM")
		destroyVM(&testConfig)
		e2e.Logf("[stage timing] Destroying target VM: %v (no poll timeout; virsh/SSH)", time.Since(stageStart))
		stageStart = time.Now()

		g.By("Waiting for etcd to stop on the surviving node")
		waitForEtcdToStop(&testConfig)
		e2e.Logf("[stage timing] Waiting for etcd to stop: %v (timeout cap: %v)", time.Since(stageStart), etcdThreeMinutePollTimeout)
		stageStart = time.Now()

		g.By("Restoring etcd quorum on surviving node")
		restoreEtcdQuorum(&testConfig, oc)
		e2e.Logf("[stage timing] Restoring etcd quorum: %v (phase1 etcd start cap: %v, phase2 %d×%v)", time.Since(stageStart), etcdPhase1StartAfterStonithTimeout, stonithCleanupMaxAttempts, stonithCleanupRoundTimeout)
		stageStart = time.Now()

		g.By("Deleting OpenShift node references")
		deleteNodeReferences(&testConfig, oc)
		e2e.Logf("[stage timing] Deleting node references (BMH/Machine/Node + OVN SB chassis-del + etcd/KAS nodeStatus + installer pods): %v (BMH/Machine delete wait: %v, poll: %v)", time.Since(stageStart), bmhMachineDeleteWaitTimeout, bmhMachineDeletePollInterval)
		stageStart = time.Now()

		g.By("Recreating the target VM using backed up configuration")
		recreateTargetVM(&testConfig, backupDir)
		e2e.Logf("[stage timing] Recreating target VM: %v (VM start timeout: %v)", time.Since(stageStart), vmLibvirtRunningTimeout)
		stageStart = time.Now()

		g.By("Provisioning the target node with Ironic")
		provisionTargetNodeWithIronic(&testConfig, oc)
		e2e.Logf("[stage timing] Provisioning with Ironic (BMH create + wait): %v (timeout: %v)", time.Since(stageStart), bmhProvisioningTimeout)
		stageStart = time.Now()

		g.By("Waiting for Machine to have nodeRef (Node object created)")
		waitForMachineToHaveNodeRef(&testConfig, oc, machineNodeRefWaitTimeout)
		e2e.Logf("[stage timing] Waiting for Machine nodeRef: %v (timeout cap: %v, poll: %v)", time.Since(stageStart), machineNodeRefWaitTimeout, machineNodeRefPollInterval)
		stageStart = time.Now()

		// cluster-machine-approver may leave Pending kube-apiserver-client-kubelet CSRs for same-node-name
		// replacements (known product bug; add OCPBUGS-xxxx to apis.WaitForAndApproveNodeBootstrapperCSR when filed).
		g.By("Waiting for node CSR to be approved (approving node-bootstrapper CSR if machine-approver has not)")
		o.Expect(apis.WaitForAndApproveNodeBootstrapperCSR(context.Background(), oc, testConfig.TargetNode.Name, csrApprovalWaitTimeout)).To(o.Succeed(), "Approve node-bootstrapper CSR for replacement node when machine-approver has not (reused node name)")
		e2e.Logf("[stage timing] CSR approval: %v (timeout cap: %v)", time.Since(stageStart), csrApprovalWaitTimeout)
		stageStart = time.Now()

		g.By("Waiting for the replacement node to become Ready (network and containers)")
		readyTime, err := waitForNodeRecovery(&testConfig, oc, nodeReadyAfterCSRTimeout, utils.ThirtySecondPollInterval)
		o.Expect(err).To(o.BeNil(), "Expected replacement node %s to appear and become Ready", testConfig.TargetNode.Name)
		e2e.Logf("[stage timing] Node Ready: %v (timeout cap: %v, poll: %v)", time.Since(stageStart), nodeReadyAfterCSRTimeout, utils.ThirtySecondPollInterval)
		stageStart = time.Now()

		g.By("Bumping kube-apiserver / KCM / scheduler revision so static pod installers re-run on the replaced node")
		forceStaticPodRevisionBump(oc)
		e2e.Logf("[stage timing] Static pod revision bump: %v (includes settle sleep cap: %v between Trace and Normal patches)", time.Since(stageStart), staticPodRevisionBumpSettleWait)

		g.By("Preparing SSH known_hosts on hypervisor for the replacement node (static manifest check via SSH)")
		khStageStart := time.Now()
		targetKH, khErr := core.PrepareRemoteKnownHostsFile(testConfig.TargetNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
		o.Expect(khErr).To(o.BeNil(), "Prepare known_hosts on hypervisor for replacement node %s (%s) for SSH manifest check", testConfig.TargetNode.Name, testConfig.TargetNode.IP)
		testConfig.TargetNode.KnownHostsPath = targetKH
		e2e.Logf("[stage timing] SSH known_hosts prep (replacement node): %v (no poll timeout)", time.Since(khStageStart))
		stageStart = time.Now()

		g.By("Assert replacement host OVS differs from pre-replace chassis (stale annotation implies same host OVS, not stale API)")
		assertReplacementOVSNotPreReplaceIdentity(&testConfig, oc)
		e2e.Logf("[stage timing] Replacement host OVS freshness assert: %v", time.Since(stageStart))
		stageStart = time.Now()

		g.By("Verifying control-plane static pod manifests exist on the replacement node (apiserver, KCM, scheduler) via SSH")
		err = waitForControlPlaneStaticPodManifestsOnNode(&testConfig, testConfig.TargetNode.Name)
		o.Expect(err).To(o.BeNil(), "Replacement node %s must have kube-apiserver, kube-controller-manager, and kube-scheduler static pod manifests under /etc/kubernetes/manifests after revision bump; missing manifests mean installers have not written them yet",
			testConfig.TargetNode.Name)
		e2e.Logf("[stage timing] Static pod manifests on replacement node (SSH poll): %v (timeout cap: %v, poll: %v)", time.Since(stageStart), staticPodManifestsWaitTimeout, staticPodManifestsPollInterval)
		stageStart = time.Now()

		g.By("Verifying east-west connectivity (surviving node -> replacement node)")
		err = waitForEastWestConnectivity(oc, testConfig.SurvivingNode.Name, testConfig.TargetNode.Name, eastWestConnectivityTimeout)
		o.Expect(err).To(o.BeNil(), "East-west connectivity from %s to %s failed; check PodNetworkConnectivityCheck and ovnkube-node/control-plane pods (deleteNodeReferences cleared SB chassis for deleted node)",
			testConfig.SurvivingNode.Name, testConfig.TargetNode.Name)
		e2e.Logf("[stage timing] East-west connectivity: %v (timeout cap: %v, poll: %v)", time.Since(stageStart), eastWestConnectivityTimeout, eastWestConnectivityPollInterval)
		stageStart = time.Now()

		g.By("Verifying OVN SB port_bindings for both nodes (each node's chassis has pod bindings visible from the other)")
		verifyOVNSBPortBindingsAfterNodeReplacement(oc, testConfig.SurvivingNode.Name, testConfig.TargetNode.Name)
		e2e.Logf("[stage timing] OVN SB port_bindings check: %v (each ovn-sbctl list uses API/exec context cap: %v)", time.Since(stageStart), shortK8sClientTimeout)
		stageStart = time.Now()

		g.By("Restoring pacemaker cluster configuration")
		restorePacemakerCluster(&testConfig, oc, readyTime)
		e2e.Logf("[stage timing] Restoring pacemaker cluster (total): %v (see sub-lines from restorePacemakerCluster for CEO job vs pcs online caps)", time.Since(stageStart))
		stageStart = time.Now()

		g.By("Verifying the cluster is fully restored")
		verifyRestoredCluster(&testConfig, oc)
		e2e.Logf("[stage timing] Verify cluster restored (total): %v (see sub-lines for CO monitor cap)", time.Since(stageStart))

		g.By("Successfully completed node replacement process")
		e2e.Logf("Node replacement process completed. Backup files created in: %s", backupDir)
	})
})
