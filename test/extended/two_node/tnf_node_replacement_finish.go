// TNF node replacement: pacemaker restore, cluster verification, BMH/Machine helpers, and API delete utilities.
package two_node

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func restorePacemakerCluster(testConfig *TNFTestConfig, oc *exutil.CLI, nodeReadyTime time.Time) {
	restorePCMStart := time.Now()
	// Prepare known hosts file for the target node now that it has been reprovisioned
	// The SSH key changed during reprovisioning, so we need to scan it again
	e2e.Logf("Preparing known_hosts for reprovisioned target node: %s", testConfig.TargetNode.IP)
	khStart := time.Now()
	targetNodeKnownHostsPath, err := core.PrepareRemoteKnownHostsFile(testConfig.TargetNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to prepare target node known hosts file after reprovisioning without error")
	testConfig.TargetNode.KnownHostsPath = targetNodeKnownHostsPath
	e2e.Logf("[stage timing] restorePacemakerCluster: known_hosts for reprovisioned target: %v (no poll timeout)", time.Since(khStart))

	// Both update-setup jobs are scheduled in parallel by the CEO; wait for both concurrently.
	// Job names include a hash suffix (e.g. tnf-update-setup-job-master-0-637363be), so we wait by node name.
	// Survivor's job does the work (add node, cluster start); target's job exits early.
	// Use the exact node Ready time so we only accept a survivor job run that started after the node was Ready.
	minPodCreationTime := nodeReadyTime
	e2e.Logf("Waiting for both CEO update-setup jobs (survivor and target) in parallel")
	ceoJobsStart := time.Now()
	var wg sync.WaitGroup
	var errSurvivor, errTarget error
	wg.Add(2)
	go func() {
		defer wg.Done()
		errSurvivor = services.WaitForSurvivorUpdateSetupJobCompletionByNode(oc, services.EtcdNamespace, testConfig.SurvivingNode.Name, minPodCreationTime, ceoUpdateSetupJobWaitTimeout, utils.ThirtySecondPollInterval)
	}()
	go func() {
		defer wg.Done()
		// Require the target node's job pod to be created after Ready so completion matches this replacement attempt.
		errTarget = services.WaitForUpdateSetupJobCompletionByNode(oc, services.EtcdNamespace, testConfig.TargetNode.Name, nodeReadyTime, ceoUpdateSetupJobWaitTimeout, utils.ThirtySecondPollInterval)
	}()
	wg.Wait()
	e2e.Logf("[stage timing] restorePacemakerCluster: CEO update-setup jobs (survivor+target parallel wall): %v (per-job timeout cap: %v, poll: %v)", time.Since(ceoJobsStart), ceoUpdateSetupJobWaitTimeout, utils.ThirtySecondPollInterval)
	o.Expect(errSurvivor).To(o.BeNil(), "Expected survivor update-setup job for node %s to complete (run after replacement node Ready)", testConfig.SurvivingNode.Name)
	o.Expect(errTarget).To(o.BeNil(), "Expected update-setup job for node %s to complete without error", testConfig.TargetNode.Name)

	// Verify both nodes are online in the pacemaker cluster
	e2e.Logf("Verifying both nodes are online in pacemaker cluster")
	nodeNames := []string{testConfig.TargetNode.Name, testConfig.SurvivingNode.Name}
	pcsStart := time.Now()
	err = services.WaitForNodesOnline(nodeNames, testConfig.SurvivingNode.IP, pacemakerNodesOnlineTimeout, utils.ThirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected both nodes %v to be online in pacemaker cluster", nodeNames)
	e2e.Logf("[stage timing] restorePacemakerCluster: pacemaker nodes online (pcs via SSH): %v (timeout cap: %v, poll: %v)", time.Since(pcsStart), pacemakerNodesOnlineTimeout, utils.ThirtySecondPollInterval)
	e2e.Logf("Both nodes %v are online in pacemaker cluster", nodeNames)
	e2e.Logf("[stage timing] restorePacemakerCluster (total): %v", time.Since(restorePCMStart))
}

// verifyRestoredCluster verifies that the cluster is fully restored and healthy
func verifyRestoredCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	verifyStart := time.Now()
	e2e.Logf("Verifying cluster restoration: checking node status and cluster operators")

	// Step 1: Verify both nodes are in Ready state
	e2e.Logf("Verifying both nodes are in Ready state")
	nodeSpotStart := time.Now()

	// Check target node
	o.Expect(utils.IsNodeReady(oc, testConfig.TargetNode.Name)).To(o.BeTrue(), "Expected target node %s to be in Ready state", testConfig.TargetNode.Name)
	e2e.Logf("Target node %s is Ready", testConfig.TargetNode.Name)

	// Check surviving node
	o.Expect(utils.IsNodeReady(oc, testConfig.SurvivingNode.Name)).To(o.BeTrue(), "Expected surviving node %s to be in Ready state", testConfig.SurvivingNode.Name)
	e2e.Logf("Surviving node %s is Ready", testConfig.SurvivingNode.Name)
	e2e.Logf("[stage timing] verifyRestoredCluster: node Ready spot-checks (two Gets): %v (no explicit API timeout on Get)", time.Since(nodeSpotStart))

	// Step 2: Verify all cluster operators are available (not degraded or progressing)
	e2e.Logf("Verifying all cluster operators are available")
	coStart := time.Now()
	coOutput, err := utils.MonitorClusterOperators(oc, clusterOperatorStabilizationTimeout, utils.FiveSecondPollInterval)
	o.Expect(err).To(o.BeNil(), "Expected all cluster operators to be available")
	e2e.Logf("[stage timing] verifyRestoredCluster: cluster operators available: %v (timeout cap: %v, poll: %v)", time.Since(coStart), clusterOperatorStabilizationTimeout, utils.FiveSecondPollInterval)
	e2e.Logf("All cluster operators are available and healthy")

	// Log final status
	e2e.Logf("Cluster verification completed successfully:")
	e2e.Logf("  - Target node %s is Ready", testConfig.TargetNode.Name)
	e2e.Logf("  - Surviving node %s is Ready", testConfig.SurvivingNode.Name)
	e2e.Logf("  - All cluster operators are available")
	e2e.Logf("\nFinal cluster operators status:\n%s", coOutput)
	e2e.Logf("[stage timing] verifyRestoredCluster (total): %v", time.Since(verifyStart))
}

// ========================================
// Helper Functions for Main Test
// ========================================

// gvrForResourceType returns the GVR for BMH or Machine (used for API-based delete/patch).
func gvrForResourceType(resourceType string) (schema.GroupVersionResource, error) {
	switch resourceType {
	case bmhResourceType:
		return apis.BMHGVR, nil
	case machineResourceType:
		return apis.MachineGVR, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unknown resource type %s", resourceType)
	}
}

// runMachineEtcdPreDrainHookStripper removes CEO's EtcdQuorumOperator preDrain hook from the Machine on a ticker until ctx
// is cancelled. First strip runs immediately, then every machinePreDrainHookStripPollInterval. This avoids MAO blocking
// drain when CEO keeps the hook while etcd still lists the node IP as a learner (see strip comment in deleteOcResourceWithRetry).
func runMachineEtcdPreDrainHookStripper(ctx context.Context, oc *exutil.CLI, namespace, machineName string) {
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		e2e.Logf("[Machine preDrain hook strip] dynamic client: %v", err)
		return
	}
	stripOnce := func() {
		c, cancel := context.WithTimeout(ctx, shortK8sClientTimeout)
		defer cancel()
		stripped, err := stripEtcdQuorumOperatorPreDrainHook(c, dyn, namespace, machineName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				e2e.Logf("[Machine preDrain hook strip] strip attempt: %v", err)
			}
			return
		}
		if stripped {
			e2e.Logf("[Machine preDrain hook strip] removed EtcdQuorumOperator preDrain hook from Machine %s/%s (workaround for CEO/learner deadlock)", namespace, machineName)
		}
	}
	stripOnce()
	ticker := time.NewTicker(machinePreDrainHookStripPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stripOnce()
		}
	}
}

// stripEtcdQuorumOperatorPreDrainHook removes spec.lifecycleHooks.preDrain entries matching etcdMachinePreDrainHookName/Owner.
// Returns (true, nil) if the Machine was updated, (false, nil) if the hook was absent or preDrain missing.
func stripEtcdQuorumOperatorPreDrainHook(ctx context.Context, dyn dynamic.Interface, namespace, machineName string) (bool, error) {
	u, err := dyn.Resource(apis.MachineGVR).Namespace(namespace).Get(ctx, machineName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	hooks, found, err := unstructured.NestedSlice(u.Object, "spec", "lifecycleHooks", "preDrain")
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	newHooks := make([]interface{}, 0, len(hooks))
	removed := false
	for _, h := range hooks {
		m, ok := h.(map[string]interface{})
		if !ok {
			newHooks = append(newHooks, h)
			continue
		}
		name, _, _ := unstructured.NestedString(m, "name")
		owner, _, _ := unstructured.NestedString(m, "owner")
		if name == etcdMachinePreDrainHookName && owner == etcdMachinePreDrainHookOwner {
			removed = true
			continue
		}
		newHooks = append(newHooks, h)
	}
	if !removed {
		return false, nil
	}
	if err := unstructured.SetNestedSlice(u.Object, newHooks, "spec", "lifecycleHooks", "preDrain"); err != nil {
		return false, err
	}
	_, err = dyn.Resource(apis.MachineGVR).Namespace(namespace).Update(ctx, u, metav1.UpdateOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

// machineAPIDeleteDiagnosticPodPrefixes matches controller pods in openshift-machine-api whose logs explain stuck BMH/Machine deletes.
var machineAPIDeleteDiagnosticPodPrefixes = []string{
	"machine-api-controllers-",
	"baremetal-operator-",
	"metal3-baremetal-operator-",
	"ironic-",
}

// clusterBaremetalOperatorDiagnosticPodPrefixes matches pods in openshift-cluster-baremetal-operator.
var clusterBaremetalOperatorDiagnosticPodPrefixes = []string{"cluster-baremetal-operator-"}

// logBMHMachineDeletionHighLevel logs finalizers, deletion timestamps, and BMH provisioning state to the test log.
func logBMHMachineDeletionHighLevel(oc *exutil.CLI, testConfig *TNFTestConfig) {
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		e2e.Logf("[BMH/Machine delete diagnostics] dynamic client: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	if u, err := dyn.Resource(apis.MachineGVR).Namespace(machineAPINamespace).Get(ctx, testConfig.TargetNode.MachineName, metav1.GetOptions{}); err == nil {
		fin, _, _ := unstructured.NestedStringSlice(u.Object, "metadata", "finalizers")
		dt, _, _ := unstructured.NestedString(u.Object, "metadata", "deletionTimestamp")
		phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
		e2e.Logf("[BMH/Machine delete diagnostics] Machine %s status.phase=%s metadata.deletionTimestamp=%s metadata.finalizers=%v",
			testConfig.TargetNode.MachineName, phase, dt, fin)
	} else {
		e2e.Logf("[BMH/Machine delete diagnostics] Machine get: %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel2()
	if u, err := dyn.Resource(apis.BMHGVR).Namespace(machineAPINamespace).Get(ctx2, testConfig.TargetNode.BMHName, metav1.GetOptions{}); err == nil {
		fin, _, _ := unstructured.NestedStringSlice(u.Object, "metadata", "finalizers")
		dt, _, _ := unstructured.NestedString(u.Object, "metadata", "deletionTimestamp")
		st, _, _ := unstructured.NestedMap(u.Object, "status")
		prov, _, _ := unstructured.NestedMap(st, "provisioning")
		state, _, _ := unstructured.NestedString(prov, "state")
		e2e.Logf("[BMH/Machine delete diagnostics] BareMetalHost %s status.provisioning.state=%s metadata.deletionTimestamp=%s metadata.finalizers=%v",
			testConfig.TargetNode.BMHName, state, dt, fin)
	} else {
		e2e.Logf("[BMH/Machine delete diagnostics] BareMetalHost get: %v", err)
	}
}

func writeBMHMachineDeletionDiagnosticFile(dir, name string, content []byte) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0644); err != nil {
		e2e.Logf("[BMH/Machine delete diagnostics] write %s: %v", path, err)
		return
	}
	e2e.Logf("[BMH/Machine delete diagnostics] wrote %s", path)
}

func ocGetWriteDiagnosticFile(oc *exutil.CLI, dir, file string, ocArgs ...string) {
	out, err := oc.AsAdmin().Run(ocArgs[0]).WithoutNamespace().Args(ocArgs[1:]...).Output()
	if err != nil {
		e2e.Logf("[BMH/Machine delete diagnostics] oc %v failed: %v", ocArgs, err)
		return
	}
	writeBMHMachineDeletionDiagnosticFile(dir, file, []byte(out))
}

func podNameHasAnyPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func dumpPodLogsByNamePrefixes(oc *exutil.CLI, namespace string, namePrefixes []string, artifactSubdir string) {
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("[BMH/Machine delete diagnostics] list pods in %s: %v", namespace, err)
		return
	}
	for _, pod := range podList.Items {
		if !podNameHasAnyPrefix(pod.Name, namePrefixes) {
			continue
		}
		tail := fmt.Sprintf("%d", machineDeletionDiagnosticsLogTailLines)
		out, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-n", namespace, "--all-containers=true", "--tail="+tail).Output()
		if err != nil {
			e2e.Logf("[BMH/Machine delete diagnostics] logs %s/%s: %v", namespace, pod.Name, err)
			continue
		}
		safe := strings.ReplaceAll(pod.Name, "/", "_")
		writeBMHMachineDeletionDiagnosticFile(artifactSubdir, fmt.Sprintf("logs-%s-%s.txt", namespace, safe), []byte(out))
	}
}

// gatherBMHMachineDeletionFailureDiagnostics writes API snapshots and controller logs when BMH or Machine delete
// polling hits bmhMachineDeleteWaitTimeout. Use this to explain finalizers, Ironic deprovision, and machine-controller waits.
func gatherBMHMachineDeletionFailureDiagnostics(oc *exutil.CLI, testConfig *TNFTestConfig, timedOutOp string) {
	if testConfig == nil {
		return
	}
	e2e.Logf("[BMH/Machine delete diagnostics] gathering state after timeout: %s", timedOutOp)
	logBMHMachineDeletionHighLevel(oc, testConfig)

	var sub string
	if testConfig != nil && testConfig.Execution.NodeReplacementLogDir != "" {
		sub = testConfig.Execution.NodeReplacementLogDir
	} else {
		artifactDir := os.Getenv("ARTIFACT_DIR")
		if artifactDir == "" {
			e2e.Logf("[BMH/Machine delete diagnostics] ARTIFACT_DIR unset; skipping artifact files (see log lines above)")
			return
		}
		// Spec should set NodeReplacementLogDir at start; this fallback timestamps a sibling folder if not.
		sub = filepath.Join(artifactDir, nodeReplacementLogsRootDirName, time.Now().Format("20060102-150405"))
		if err := os.MkdirAll(sub, 0755); err != nil {
			e2e.Logf("[BMH/Machine delete diagnostics] mkdir %s: %v", sub, err)
			return
		}
	}
	if y, err := apis.GetMachineYAML(oc, testConfig.TargetNode.MachineName, machineAPINamespace); err != nil {
		e2e.Logf("[BMH/Machine delete diagnostics] GetMachineYAML: %v", err)
	} else {
		writeBMHMachineDeletionDiagnosticFile(sub, "machine.yaml", y)
	}
	if y, err := apis.GetBMHYAML(oc, testConfig.TargetNode.BMHName, machineAPINamespace); err != nil {
		e2e.Logf("[BMH/Machine delete diagnostics] GetBMHYAML: %v", err)
	} else {
		writeBMHMachineDeletionDiagnosticFile(sub, "baremetalhost.yaml", y)
	}
	ocGetWriteDiagnosticFile(oc, sub, "events-openshift-machine-api.yaml", "get", "events", "-n", machineAPINamespace, "-o", "yaml", "--sort-by=.lastTimestamp")
	ocGetWriteDiagnosticFile(oc, sub, "clusteroperator-machine-api.yaml", "get", "clusteroperator", "machine-api", "-o", "yaml")
	ocGetWriteDiagnosticFile(oc, sub, "machines-openshift-machine-api.yaml", "get", "machines.machine.openshift.io", "-n", machineAPINamespace, "-o", "yaml")
	ocGetWriteDiagnosticFile(oc, sub, "baremetalhosts-openshift-machine-api.yaml", "get", "baremetalhost", "-n", machineAPINamespace, "-o", "yaml")
	ocGetWriteDiagnosticFile(oc, sub, "events-openshift-cluster-baremetal-operator.yaml", "get", "events", "-n", clusterBaremetalOperatorNamespace, "-o", "yaml", "--sort-by=.lastTimestamp")

	dumpPodLogsByNamePrefixes(oc, machineAPINamespace, machineAPIDeleteDiagnosticPodPrefixes, sub)
	dumpPodLogsByNamePrefixes(oc, clusterBaremetalOperatorNamespace, clusterBaremetalOperatorDiagnosticPodPrefixes, sub)

	e2e.Logf("[BMH/Machine delete diagnostics] artifact directory: %s", sub)
}

// deleteOcResourceWithRetry deletes an OpenShift resource (BMH, Machine) via the cluster API.
//
// Issues Delete with a non-cancelled context (no per-request deadline) so a slow apiserver does not abort the
// request mid-flight; then polls until the object is gone or bmhMachineDeleteWaitTimeout elapses. Delete is
// retried while the object still exists (idempotent). Existence checks use deleteGetTimeout. No force-delete.
// testConfig is used only on timeout: gatherBMHMachineDeletionFailureDiagnostics writes logs and API snapshots
// under testConfig.Execution.NodeReplacementLogDir (same directory as CEO log capture when set).
func deleteOcResourceWithRetry(oc *exutil.CLI, resourceType, resourceName, namespace string, testConfig *TNFTestConfig) error {
	gvr, err := gvrForResourceType(resourceType)
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	// Workaround: cluster-etcd-operator may keep the EtcdQuorumOperator preDrain hook while etcd still lists the
	// node's IP as a learner (podman-etcd RA can re-add the peer as a learner on the survivor). MAO then blocks
	// drain forever. Strip the hook periodically during the delete window so machine-controller can proceed.
	// Product fix: CEO should clear the hook when only a learner matches (see bug report / etcd BZ).
	if resourceType == machineResourceType {
		stripCtx, stripCancel := context.WithCancel(context.Background())
		var stripWG sync.WaitGroup
		stripWG.Add(1)
		go func() {
			defer stripWG.Done()
			runMachineEtcdPreDrainHookStripper(stripCtx, oc, namespace, resourceName)
		}()
		defer func() {
			stripCancel()
			stripWG.Wait()
		}()
	}

	opName := fmt.Sprintf("delete %s %s", resourceType, resourceName)
	start := time.Now()
	deadline := start.Add(bmhMachineDeleteWaitTimeout)
	attempt := 0
	for {
		if time.Now().After(deadline) {
			gatherBMHMachineDeletionFailureDiagnostics(oc, testConfig, opName)
			return fmt.Errorf("%s: resource %s/%s still present after %v", opName, namespace, resourceName, bmhMachineDeleteWaitTimeout)
		}
		attempt++
		// Use Background for Delete so we do not cancel an in-progress delete request before the apiserver responds.
		delErr := dyn.Resource(gvr).Namespace(namespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
		if delErr != nil {
			if apierrors.IsNotFound(delErr) {
				e2e.Logf("deleteOcResourceWithRetry Delete %s/%s: NotFound (already deleted)", namespace, resourceName)
			} else {
				e2e.Logf("deleteOcResourceWithRetry Delete %s/%s failed (continuing with existence check): %v", namespace, resourceName, delErr)
			}
		}
		getCtx, getCancel := context.WithTimeout(context.Background(), deleteGetTimeout)
		stillThere := resourceExists(getCtx, dyn, gvr, resourceName, namespace)
		getCancel()
		if !stillThere {
			e2e.Logf("deleteOcResourceWithRetry %s: %s/%s removed from API after attempt %d (elapsed %v)",
				opName, namespace, resourceName, attempt, time.Since(start))
			return nil
		}
		e2e.Logf("deleteOcResourceWithRetry %s: %s/%s still exists after attempt %d; sleeping %v before retry",
			opName, namespace, resourceName, attempt, bmhMachineDeletePollInterval)
		time.Sleep(bmhMachineDeletePollInterval)
	}
}

// resourceExists returns true if the resource exists. Only apierrors.IsNotFound is treated as absent; other
// errors (e.g. API timeout, network) are logged and treated as "still exists". Callers must pass a timeout-bounded
// context (e.g. context.WithTimeout with deleteGetTimeout for delete retry).
func resourceExists(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, name, namespace string) bool {
	_, err := dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return true
	}
	if apierrors.IsNotFound(err) {
		return false
	}
	e2e.Logf("resourceExists Get %s/%s failed (treating as still exists): %v", namespace, name, err)
	return true
}

// baremetalOperatorDeploymentCandidates is the order of deployment names to try when resolving the BMO deployment
// (metal3/dev-scripts use metal3-baremetal-operator; standard OCP uses baremetal-operator).
var baremetalOperatorDeploymentCandidates = []string{baremetalOperatorDeploymentNameMetal3, baremetalOperatorDeploymentName}

// waitForBaremetalOperatorDeploymentReady waits until the baremetal-operator deployment has at least one ready replica.
// Call before requesting BMH/Machine deletes so BMO is running and can process the delete (reconcile and remove finalizers after Ironic cleanup).
// Discovers the deployment by trying metal3-baremetal-operator (metal3/dev-scripts) then baremetal-operator (standard OCP).
func waitForBaremetalOperatorDeploymentReady(oc *exutil.CLI, timeout time.Duration) {
	e2e.Logf("Waiting for baremetal-operator deployment in %s to have a ready replica (timeout: %v)", machineAPINamespace, timeout)
	err := core.PollUntil(func() (bool, error) {
		for _, name := range baremetalOperatorDeploymentCandidates {
			// Bounded Get so a stuck apiserver request cannot block this PollUntil body forever (PollUntil interval 10s).
			getCtx, getCancel := context.WithTimeout(context.Background(), 9*time.Second)
			dep, err := oc.AdminKubeClient().AppsV1().Deployments(machineAPINamespace).Get(getCtx, name, metav1.GetOptions{})
			getCancel()
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				e2e.Logf("Failed to get deployment %s/%s: %v", machineAPINamespace, name, err)
				return false, nil
			}
			ready := dep.Status.ReadyReplicas >= 1
			if !ready {
				e2e.Logf("Deployment %s/%s has %d ready replicas (want >= 1), continuing to poll", machineAPINamespace, name, dep.Status.ReadyReplicas)
				return false, nil
			}
			e2e.Logf("Deployment %s/%s has at least one ready replica", machineAPINamespace, name)
			return true, nil
		}
		e2e.Logf("No baremetal-operator deployment found in %s (tried %v), continuing to poll", machineAPINamespace, baremetalOperatorDeploymentCandidates)
		return false, nil
	}, timeout, 10*time.Second, fmt.Sprintf("baremetal-operator deployment in %s to have ready replica", machineAPINamespace))
	o.Expect(err).To(o.BeNil(), "Expected baremetal-operator deployment to be ready before requesting BMH/Machine deletes (tried %v)", baremetalOperatorDeploymentCandidates)
}

// waitForBaremetalOperatorWebhookReady waits until the BareMetalHost validating webhook service has at least one endpoint.
// The API server cannot create BMHs until the webhook is reachable; after a disruptive test the baremetal-operator pod
// may be rescheduling onto the survivor, so we wait before creating the replacement BMH.
func waitForBaremetalOperatorWebhookReady(oc *exutil.CLI, timeout time.Duration) {
	e2e.Logf("Waiting for BareMetalHost validating webhook service %s/%s to have endpoints (timeout: %v)", machineAPINamespace, baremetalOperatorWebhookServiceName, timeout)
	err := core.PollUntil(func() (bool, error) {
		// Bounded Get; PollUntil uses baremetalWebhookPollInterval (15s).
		getCtx, getCancel := context.WithTimeout(context.Background(), 14*time.Second)
		defer getCancel()
		ep, err := oc.AdminKubeClient().CoreV1().Endpoints(machineAPINamespace).Get(getCtx, baremetalOperatorWebhookServiceName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get endpoints for %s: %v", baremetalOperatorWebhookServiceName, err)
			return false, nil
		}
		var total int
		for _, subset := range ep.Subsets {
			total += len(subset.Addresses)
		}
		if total > 0 {
			e2e.Logf("BareMetalHost validating webhook service has %d endpoint(s)", total)
			return true, nil
		}
		e2e.Logf("BareMetalHost validating webhook service has no endpoints yet, continuing to poll")
		return false, nil
	}, timeout, baremetalWebhookPollInterval, fmt.Sprintf("%s/%s to have endpoints", machineAPINamespace, baremetalOperatorWebhookServiceName))
	o.Expect(err).To(o.BeNil(), "Expected BareMetalHost validating webhook to be ready (service with endpoints) before creating BMH")
}

// logPacemakerStatus logs the pacemaker cluster status for verification purposes.
// This is a non-fatal operation - if it fails, a warning is logged but execution continues.
//
// Parameters:
//   - context: describes why the status is being checked (e.g., "verification after pcs debug-start")
//
// Example usage:
//
//	logPacemakerStatus(testConfig, "verification after pcs debug-start")
func logPacemakerStatus(testConfig *TNFTestConfig, context string) {
	e2e.Logf("Getting pacemaker status on %s for %s", testConfig.SurvivingNode.Name, context)
	pcsStatusOutput, _, err := services.PcsStatusFull(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to get pacemaker status on %s: %v", testConfig.SurvivingNode.Name, err)
	} else {
		e2e.Logf("Pacemaker status on %s:\n%s", testConfig.SurvivingNode.Name, pcsStatusOutput)
	}
}

// waitForAPIResponsive waits for the Kubernetes API to become responsive.
// This function will cause a test failure if the API does not respond within the timeout.
//
// Primary use case: Verifying API restoration after etcd quorum restoration.
//
// Parameters:
//   - timeout: maximum time to wait for API responsiveness
//
// The function polls every 15 seconds until the API responds or timeout is reached.
func waitForAPIResponsive(oc *exutil.CLI, timeout time.Duration) {
	e2e.Logf("Waiting for the Kubernetes API to be responsive (timeout: %v)", timeout)
	err := core.PollUntil(func() (bool, error) {
		if utils.IsAPIResponding(oc) {
			e2e.Logf("Kubernetes API is responding")
			return true, nil
		}
		e2e.Logf("Kubernetes API not yet responding, continuing to poll")
		return false, nil
	}, timeout, utils.FiveSecondPollInterval, "Kubernetes API to be responsive")
	o.Expect(err).To(o.BeNil(), "Expected Kubernetes API to be responsive within timeout")
}

// waitForEtcdResourceToStop waits for etcd resource to stop on the surviving node
func waitForEtcdResourceToStop(testConfig *TNFTestConfig, timeout time.Duration) error {
	e2e.Logf("Waiting for etcd resource to stop on surviving node: %s (timeout: %v)", testConfig.SurvivingNode.Name, timeout)

	return core.RetryWithOptions(func() error {
		// Check etcd resource status on the surviving node
		e2e.Logf("Polling etcd resource status on node %s", testConfig.SurvivingNode.Name)
		output, _, err := services.PcsResourceStatus(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			e2e.Logf("Failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		}

		e2e.Logf("Etcd resource status on %s:\n%s", testConfig.SurvivingNode.Name, output)

		// Check if etcd is stopped (not started) on the surviving node
		// We expect to see "Stopped: [ master-X ]" or no "Started:" line for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNode.Name) {
				e2e.Logf("etcd is still started on surviving node %s (found line: %s)", testConfig.SurvivingNode.Name, line)
				return fmt.Errorf("etcd is still started on surviving node %s", testConfig.SurvivingNode.Name)
			}
		}

		// If we get here, etcd is not started on the surviving node
		e2e.Logf("etcd has stopped on surviving node: %s", testConfig.SurvivingNode.Name)
		return nil
	}, core.RetryOptions{
		Timeout:      timeout,
		PollInterval: utils.FiveSecondPollInterval,
	}, fmt.Sprintf("etcd stop on %s", testConfig.SurvivingNode.Name))
}

// waitForEtcdToStart waits for etcd to start on the surviving node
func waitForEtcdToStart(testConfig *TNFTestConfig, timeout, pollInterval time.Duration) error {
	e2e.Logf("Waiting for etcd to start on surviving node: %s (timeout: %v, poll interval: %v)", testConfig.SurvivingNode.Name, timeout, pollInterval)

	return core.RetryWithOptions(func() error {
		// Check etcd resource status on the surviving node
		e2e.Logf("Polling etcd resource status on node %s", testConfig.SurvivingNode.Name)
		output, _, err := services.PcsResourceStatus(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			e2e.Logf("Failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		}

		e2e.Logf("Etcd resource status on %s:\n%s", testConfig.SurvivingNode.Name, output)

		// Check if etcd is started on the surviving node
		// We expect to see "Started: [ master-X ]" for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNode.Name) {
				e2e.Logf("etcd has started on surviving node: %s (found line: %s)", testConfig.SurvivingNode.Name, line)
				return nil
			}
		}

		e2e.Logf("etcd is not started yet on surviving node %s", testConfig.SurvivingNode.Name)
		return fmt.Errorf("etcd is not started on surviving node %s", testConfig.SurvivingNode.Name)
	}, core.RetryOptions{
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, fmt.Sprintf("etcd start on %s", testConfig.SurvivingNode.Name))
}

// updateAndCreateBMH creates a new BareMetalHost from template
func updateAndCreateBMH(testConfig *TNFTestConfig, oc *exutil.CLI, newUUID, newMACAddress string) {
	e2e.Logf("Creating BareMetalHost with UUID: %s, MAC: %s", newUUID, newMACAddress)

	// Authority is the host:port part of a URL (net.JoinHostPort brackets IPv6 per RFC 3986).
	redfishAuthority := net.JoinHostPort(testConfig.Execution.RedfishIP, redfishPort)
	e2e.Logf("BMC address authority: %s", redfishAuthority)

	// Create BareMetalHost from template with placeholder substitution
	err := core.CreateResourceFromTemplate(oc, bmhTemplatePath, map[string]string{
		"{BMH_NAME}":          testConfig.TargetNode.BMHName,
		"{REDFISH_AUTHORITY}": redfishAuthority,
		"{UUID}":              newUUID,
		"{CREDENTIALS_NAME}":  testConfig.TargetNode.BMCSecretName,
		"{BOOT_MAC_ADDRESS}":  newMACAddress,
	})
	o.Expect(err).To(o.BeNil(), "Expected to create BareMetalHost without error")

	e2e.Logf("Successfully created BareMetalHost: %s", testConfig.TargetNode.BMHName)
}

// waitForBMHProvisioning waits for the BareMetalHost to be provisioned
func waitForBMHProvisioning(ctx context.Context, testConfig *TNFTestConfig, oc *exutil.CLI) error {
	bmhName := testConfig.TargetNode.BMHName
	e2e.Logf("Waiting for BareMetalHost %s to be provisioned...", bmhName)

	deadline := time.Now().Add(bmhProvisioningTimeout)
	pollInterval := utils.ThirtySecondPollInterval
	lastState := ""

	// Poll explicitly (not core.PollUntil): that helper treats any checker error as retriable until timeout,
	// so terminal BMH errors would still burn the full wait. Fail fast when ErrorType is set.
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		bmh, err := apis.GetBMH(oc, bmhName, machineAPINamespace)
		if err != nil {
			e2e.Logf("Error getting BareMetalHost %s: %v", bmhName, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}
			continue
		}

		lastState = string(bmh.Status.Provisioning.State)
		e2e.Logf("BareMetalHost %s current state: %s", bmhName, lastState)
		if bmh.Status.ErrorMessage != "" {
			e2e.Logf("BareMetalHost %s error message: %s", bmhName, bmh.Status.ErrorMessage)
		}
		if bmh.Status.ErrorType != "" {
			o.Expect(fmt.Errorf("BareMetalHost %s in terminal error state %s: %s",
				bmhName, bmh.Status.ErrorType, bmh.Status.ErrorMessage)).To(o.BeNil(),
				"BareMetalHost provisioning failed (terminal ErrorType)")
			return nil
		}
		if lastState == string(metal3v1alpha1.StateProvisioned) {
			e2e.Logf("BareMetalHost %s is provisioned", bmhName)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return fmt.Errorf("BareMetalHost %s not provisioned within %v (last state %q)",
		bmhName, bmhProvisioningTimeout, lastState)
}

// reapplyDetachedAnnotation reapplies the detached annotation to the BareMetalHost via the cluster API.
func reapplyDetachedAnnotation(testConfig *TNFTestConfig, oc *exutil.CLI, ctx context.Context) {
	e2e.Logf("Applying detached annotation to BareMetalHost: %s", testConfig.TargetNode.BMHName)
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	o.Expect(err).To(o.BeNil(), "Expected to create dynamic client")
	getCtx, getCancel := context.WithTimeout(ctx, shortK8sClientTimeout)
	u, err := dyn.Resource(apis.BMHGVR).Namespace(machineAPINamespace).Get(getCtx, testConfig.TargetNode.BMHName, metav1.GetOptions{})
	getCancel()
	o.Expect(err).To(o.BeNil(), "Expected to get BMH %s", testConfig.TargetNode.BMHName)
	ann := u.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	ann[bmhDetachedAnnotationKey] = ""
	u.SetAnnotations(ann)
	updCtx, updCancel := context.WithTimeout(ctx, shortK8sClientTimeout)
	_, err = dyn.Resource(apis.BMHGVR).Namespace(machineAPINamespace).Update(updCtx, u, metav1.UpdateOptions{})
	updCancel()
	o.Expect(err).To(o.BeNil(), "Expected to apply detached annotation to BMH %s without error", testConfig.TargetNode.BMHName)
	e2e.Logf("Successfully applied detached annotation to BareMetalHost: %s", testConfig.TargetNode.BMHName)
}

// recreateMachine recreates the Machine resource from template. Uses API to check if Machine already exists.
func recreateMachine(testConfig *TNFTestConfig, oc *exutil.CLI) {
	e2e.Logf("Recreating Machine: %s", testConfig.TargetNode.MachineName)
	exists, err := apis.MachineExists(oc, testConfig.TargetNode.MachineName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to check Machine existence")
	if exists {
		e2e.Logf("Machine %s already exists, skipping recreation", testConfig.TargetNode.MachineName)
		return
	}
	// Create Machine from template (oc create -f is still used by CreateResourceFromTemplate; template is static YAML)
	err = core.CreateResourceFromTemplate(oc, machineTemplatePath, map[string]string{
		"{BMH_NAME}":     testConfig.TargetNode.BMHName,
		"{MACHINE_NAME}": testConfig.TargetNode.MachineName,
		"{MACHINE_HASH}": testConfig.TargetNode.MachineHash,
	})
	o.Expect(err).To(o.BeNil(), "Expected to create Machine without error")
	e2e.Logf("Successfully recreated Machine: %s", testConfig.TargetNode.MachineName)
}
