// TNF node replacement: OVN/SB diagnostics, VM disk/recreate, provisioning, east–west, static pods, OVN-K recovery.
package two_node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func nodeOVNChassisIDFromNode(node *corev1.Node) string {
	if node == nil || node.Annotations == nil {
		return ""
	}
	return node.Annotations[ovnNodeChassisIDAnnotation]
}

func trimOVSQuotedValue(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"`)
}

// distinctNonEmpty returns unique non-empty strings from ss (order preserved).
func distinctNonEmpty(ss ...string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// readHostOVSSystemID runs ovs-vsctl on a node via two-hop SSH (hypervisor → node).
func readHostOVSSystemID(nodeIP string, testConfig *TNFTestConfig, remoteKnownHostsPath string) (string, error) {
	if remoteKnownHostsPath == "" {
		return "", fmt.Errorf("remoteKnownHostsPath is empty for SSH to node %s", nodeIP)
	}
	stdout, _, err := core.ExecuteRemoteSSHCommand(nodeIP, ovsSystemIDGetCmd, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		return "", err
	}
	return trimOVSQuotedValue(stdout), nil
}

// logPreDestroyOVNChassisTrace records facts for interpreting k8s.ovn.org/node-chassis-id during replacement:
// the Node object is removed from the API, but node-chassis-id is published from **this node's** OVS system-id.
// A chassis id on the Node that does not match a fresh host means local OVS still has that identity—not etcd caching.
// SB Chassis rows for an unreachable peer are inspected from the **survivor's** ovnkube-node/sbdb view, not by
// survivor OVS taking on the peer's system-id.
func logPreDestroyOVNChassisTrace(testConfig *TNFTestConfig, oc *exutil.CLI) {
	targetKH, err := core.PrepareRemoteKnownHostsFile(testConfig.TargetNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		e2e.Logf("%s pre-destroy: skip target OVS SSH (known_hosts: %v)", ovnStaleChassisLogPrefix, err)
		return
	}
	defer func() {
		_ = core.CleanupRemoteKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, targetKH)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testConfig.TargetNode.Name, metav1.GetOptions{})
	cancel()
	chassisFromAPI := ""
	if err == nil && node != nil {
		chassisFromAPI = nodeOVNChassisIDFromNode(node)
	} else {
		e2e.Logf("%s pre-destroy: get node %s: %v", ovnStaleChassisLogPrefix, testConfig.TargetNode.Name, err)
	}

	targetOVS, tErr := readHostOVSSystemID(testConfig.TargetNode.IP, testConfig, targetKH)
	if tErr != nil {
		e2e.Logf("%s pre-destroy: target %s OVS: %v", ovnStaleChassisLogPrefix, testConfig.TargetNode.Name, tErr)
	} else {
		testConfig.Execution.PreReplacementTargetOVSSystemID = targetOVS
	}

	survivorOVS, sErr := readHostOVSSystemID(testConfig.SurvivingNode.IP, testConfig, testConfig.SurvivingNode.KnownHostsPath)
	if sErr != nil {
		e2e.Logf("%s pre-destroy: survivor %s OVS: %v", ovnStaleChassisLogPrefix, testConfig.SurvivingNode.Name, sErr)
	}

	apiVsTarget := "n/a"
	if chassisFromAPI != "" && tErr == nil && targetOVS != "" {
		if chassisFromAPI == targetOVS {
			apiVsTarget = "api_chassis_eq_target_ovs"
		} else {
			apiVsTarget = "api_chassis_ne_target_ovs"
		}
	}
	e2e.Logf("%s pre-destroy: target=%q api_chassis=%q ovs=%q (%s); survivor=%q ovs=%q",
		ovnStaleChassisLogPrefix, testConfig.TargetNode.Name, chassisFromAPI, targetOVS, apiVsTarget,
		testConfig.SurvivingNode.Name, survivorOVS)
	e2e.Logf("%s model: annotation=local OVS on that node; SB ghost chassis for deleted hostname=bad SB view (survivor ovnkube-node/sbdb), not peer OVS copied onto survivor", ovnStaleChassisLogPrefix)
}

// logSurvivorOVSAfterChassisCleanup logs survivor OVS after SB chassis-del (post Node delete; ovnkube-node was not restarted—sbdb must stay up for chassis-del).
func logSurvivorOVSAfterChassisCleanup(testConfig *TNFTestConfig) {
	id, err := readHostOVSSystemID(testConfig.SurvivingNode.IP, testConfig, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("%s post-chassis-cleanup: survivor %s OVS read: %v", ovnStaleChassisLogPrefix, testConfig.SurvivingNode.Name, err)
		return
	}
	pre := testConfig.Execution.PreReplacementChassisID
	e2e.Logf("%s post-chassis-cleanup: survivor=%q ovs=%q pre_replace_target_chassis=%q (survivor ovs should differ from deleted target's chassis)",
		ovnStaleChassisLogPrefix, testConfig.SurvivingNode.Name, id, pre)
	if pre != "" && id == pre {
		e2e.Logf("%s WARN survivor ovs equals pre_replace chassis %q (unexpected for two nodes)", ovnStaleChassisLogPrefix, pre)
	}
}

// assertReplacementOVSNotPreReplaceIdentity fails if reprovisioned host OVS still matches any captured pre-replace id.
func assertReplacementOVSNotPreReplaceIdentity(testConfig *TNFTestConfig, oc *exutil.CLI) {
	o.Expect(testConfig.TargetNode.KnownHostsPath).NotTo(o.BeEmpty(), "replacement known_hosts required for OVS assert")
	newID, err := readHostOVSSystemID(testConfig.TargetNode.IP, testConfig, testConfig.TargetNode.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "OVS read on replacement %s", testConfig.TargetNode.Name)

	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	node, errN := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testConfig.TargetNode.Name, metav1.GetOptions{})
	o.Expect(errN).To(o.BeNil(), "get node %s", testConfig.TargetNode.Name)
	ann := nodeOVNChassisIDFromNode(node)

	e2e.Logf("%s post-reprovision: node=%q ovs=%q api_chassis=%q pre_api=%q pre_ovs=%q",
		ovnStaleChassisLogPrefix, testConfig.TargetNode.Name, newID, ann,
		testConfig.Execution.PreReplacementChassisID, testConfig.Execution.PreReplacementTargetOVSSystemID)

	for _, old := range distinctNonEmpty(testConfig.Execution.PreReplacementChassisID, testConfig.Execution.PreReplacementTargetOVSSystemID) {
		o.Expect(newID).NotTo(o.Equal(old), "replacement OVS must differ from pre-replace id %q (same id => annotation can match 'erased' cluster state while host identity persisted)", old)
	}
	if ann != "" && newID != "" && ann != newID {
		e2e.Logf("%s api_chassis %q != host ovs %q (OVN converging)", ovnStaleChassisLogPrefix, ann, newID)
	}
}

// removePreReplacementChassisFromNodeAndRecycleOVNKubeNode clears k8s.ovn.org/node-chassis-id when it still equals
// preReplacementChassisID and deletes ovnkube-node pod(s) on that node so OVN-K re-reads OVS and re-publishes SB/API state.
// Used as recovery so the replacement can become Ready and the cluster can reach a stable OVN posture; it does not
// change host OVS external_ids:system-id if the OS/OVS layer reused the same id.
func removePreReplacementChassisFromNodeAndRecycleOVNKubeNode(oc *exutil.CLI, targetNodeName, preReplacementChassisID string) bool {
	if preReplacementChassisID == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, targetNodeName, metav1.GetOptions{})
	if err != nil || node == nil {
		return false
	}
	chassis := nodeOVNChassisIDFromNode(node)
	if chassis != preReplacementChassisID {
		return false
	}
	e2e.Logf("[OVN chassis recovery] node %s has node-chassis-id %q matching pre-replacement; removing annotation and recycling ovnkube-node on target", targetNodeName, chassis)
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	delete(node.Annotations, ovnNodeChassisIDAnnotation)
	_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		e2e.Logf("[OVN chassis recovery] Failed to update node %s to remove chassis annotation: %v", targetNodeName, err)
		return false
	}
	e2e.Logf("[OVN chassis recovery] Removed node-chassis-id from %s; deleting ovnkube-node pod(s) on target", targetNodeName)
	pods, lerr := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=ovnkube-node",
		FieldSelector: "spec.nodeName=" + targetNodeName,
	})
	if lerr == nil {
		for i := range pods.Items {
			_ = oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).Delete(ctx, pods.Items[i].Name, metav1.DeleteOptions{})
		}
		if len(pods.Items) > 0 {
			e2e.Logf("[OVN chassis recovery] Deleted %d ovnkube-node pod(s) on %s", len(pods.Items), targetNodeName)
		}
	}
	return true
}

// clearStaleChassisAnnotationOnReplacementIfNeeded polls for a non-empty node-chassis-id; if it matches the pre-delete
// chassis name, strips k8s.ovn.org/node-chassis-id and recycles ovnkube-node on that node so OVN-K re-registers from
// local OVS. Targets the case where the guest reused the old OVS external_ids:system-id (stale disk identity).
// Invoked from recoverClusterFromBackup step 8 when the spec has failed (see header there). waitForNodeRecovery may
// also strip during its Ready poll if the same mismatch appears earlier in the main flow.
func clearStaleChassisAnnotationOnReplacementIfNeeded(oc *exutil.CLI, targetNodeName, preReplacementChassisID string) {
	if preReplacementChassisID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*stonithCleanupRoundTimeout)
	defer cancel()
	_ = core.PollUntil(func() (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, targetNodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) || err != nil {
			return false, nil
		}
		chassis := nodeOVNChassisIDFromNode(node)
		if chassis == "" {
			return false, nil
		}
		if chassis != preReplacementChassisID {
			return true, nil
		}
		if removePreReplacementChassisFromNodeAndRecycleOVNKubeNode(oc, targetNodeName, preReplacementChassisID) {
			return true, nil
		}
		return false, nil
	}, 2*stonithCleanupRoundTimeout, 15*time.Second, "replacement node to appear and recover stale pre-replacement chassis id if needed")
}

// logReplacementOVNChassisStaleIfPresent logs when k8s.ovn.org/node-chassis-id still equals preReplacementChassisID.
// Recovery must not fail the spec on this alone: we continue with east-west / SB / cluster checks (best-effort stability).
func logReplacementOVNChassisStaleIfPresent(oc *exutil.CLI, nodeName, preReplacementChassisID, contextHint string) {
	if preReplacementChassisID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		e2e.Logf("%s %s: get node %s: %v", ovnStaleChassisLogPrefix, contextHint, nodeName, err)
		return
	}
	ch := nodeOVNChassisIDFromNode(node)
	if ch == "" {
		e2e.Logf("%s %s: node %s empty api_chassis (pre=%q)", ovnStaleChassisLogPrefix, contextHint, nodeName, preReplacementChassisID)
		return
	}
	if ch == preReplacementChassisID {
		e2e.Logf("%s %s: node %s api_chassis=%q still equals pre_replace (annotation follows **local OVS**; not etcd remembering deleted Node)", ovnStaleChassisLogPrefix, contextHint, nodeName, ch)
		return
	}
	e2e.Logf("%s %s: node %s api_chassis=%q ok (changed from pre %q)", ovnStaleChassisLogPrefix, contextHint, nodeName, ch, preReplacementChassisID)
}

// logNodeOVNChassisWebhookDiag logs Node uid, lifecycle, and k8s.ovn.org/node-chassis-id. Use when debugging
// network-node-identity webhook denials: ovnkube may update chassis-id after an OVN restart while the Node
// annotation still reflects an older value (SB can list a different chassis; a new ovnkube instance may pick a new id).
func logNodeOVNChassisWebhookDiag(oc *exutil.CLI, nodeName, phase string) {
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			e2e.Logf("[node-chassis-id webhook diag] phase=%q node=%s: Node object not present (NotFound)", phase, nodeName)
		} else {
			e2e.Logf("[node-chassis-id webhook diag] phase=%q node=%s: Get failed: %v", phase, nodeName, err)
		}
		return
	}
	chassis := nodeOVNChassisIDFromNode(node)
	e2e.Logf("[node-chassis-id webhook diag] phase=%q node=%s uid=%s resourceVersion=%s created=%s deletionTimestamp=%v node-chassis-id=%q",
		phase, nodeName, node.UID, node.ResourceVersion, node.CreationTimestamp.UTC().Format(time.RFC3339), node.DeletionTimestamp, chassis)
}

// quotePathForShell returns path safe for use inside a single-quoted remote shell command (escapes single quotes).
func quotePathForShell(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// hypervisorDiskPathSetForLiveVM returns the set of canonical (filepath.Clean) backing file paths for all
// file- or volume-backed disks in the domain XML for vmName.
func hypervisorDiskPathSetForLiveVM(testConfig *TNFTestConfig, vmName string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	xmlOutput, err := services.VirshDumpXML(vmName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("dumpxml %s: %w", vmName, err)
	}
	refs, err := services.ExtractDiskSourceRefs(xmlOutput)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		p, err := resolveDiskSourceRefPath(ref, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("resolve disk ref %+v: %w", ref, err)
		}
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("empty resolved path for ref %+v", ref)
		}
		out[filepath.Clean(p)] = struct{}{}
	}
	return out, nil
}

// mustNotCollideWithSurvivorDisk fails the spec if path is one of the surviving VM's disk paths recorded at setup.
func mustNotCollideWithSurvivorDisk(testConfig *TNFTestConfig, path string) {
	if testConfig.Execution.SurvivorLibvirtDiskPaths == nil {
		return
	}
	p := filepath.Clean(strings.TrimSpace(path))
	if p == "" || p == "." {
		return
	}
	if _, ok := testConfig.Execution.SurvivorLibvirtDiskPaths[p]; ok {
		o.Expect(false).To(o.BeTrue(),
			"refusing to touch disk at %s: path matches surviving VM %s (%s) libvirt disk; target dumpxml/vol-path must not alias the survivor's backing store",
			path, testConfig.SurvivingNode.Name, testConfig.SurvivingNode.VMName)
	}
}

// resolveDiskSourceRefPath resolves libvirt disk source refs (file or volume) to concrete hypervisor file paths.
func resolveDiskSourceRefPath(ref services.DiskSourceRef, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	switch ref.Type {
	case "file":
		path := strings.TrimSpace(ref.FilePath)
		if path == "" {
			return "", fmt.Errorf("file-backed disk source has empty file path")
		}
		return path, nil
	case "volume":
		pool := strings.TrimSpace(ref.Pool)
		vol := strings.TrimSpace(ref.Volume)
		if pool == "" || vol == "" {
			return "", fmt.Errorf("volume-backed disk source missing pool/volume (pool=%q volume=%q)", pool, vol)
		}
		cmd := fmt.Sprintf("sudo virsh -c qemu:///system vol-path %s --pool %s", quotePathForShell(vol), quotePathForShell(pool))
		stdout, stderr, err := core.ExecuteSSHCommand(cmd, sshConfig, knownHostsPath)
		if err != nil {
			return "", fmt.Errorf("resolve vol-path pool=%s volume=%s: %v (stderr=%q)", pool, vol, err, strings.TrimSpace(stderr))
		}
		path := strings.TrimSpace(stdout)
		if path == "" {
			return "", fmt.Errorf("resolved empty vol-path for pool=%s volume=%s", pool, vol)
		}
		return path, nil
	default:
		return "", fmt.Errorf("unsupported disk source type %q", ref.Type)
	}
}

// getDiskSizeFromHypervisor runs qemu-img info on the hypervisor and returns a size string (e.g. "100G") for the disk at path.
// Uses sudo so it can read root-owned disk images in the libvirt pool. If the command fails or parsing fails, returns defaultSize.
func getDiskSizeFromHypervisor(path, defaultSize string, sshConfig *core.SSHConfig, knownHostsPath string) string {
	qPath := quotePathForShell(path)
	cmd := fmt.Sprintf("sudo qemu-img info --output=json %s", qPath)
	stdout, _, err := core.ExecuteSSHCommand(cmd, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("getDiskSizeFromHypervisor: qemu-img info failed for %s: %v, using default %s", path, err, defaultSize)
		return defaultSize
	}
	var info struct {
		VirtualSize int64 `json:"virtual-size"`
	}
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		e2e.Logf("getDiskSizeFromHypervisor: failed to parse qemu-img output for %s: %v, using default %s", path, err, defaultSize)
		return defaultSize
	}
	if info.VirtualSize <= 0 {
		return defaultSize
	}
	// Round up to next GB
	sizeGB := (info.VirtualSize + 1024*1024*1024 - 1) / (1024 * 1024 * 1024)
	if sizeGB < 1 {
		sizeGB = 1
	}
	return strconv.FormatInt(sizeGB, 10) + "G"
}

// assertFreshQcow2DiskOnHypervisor fails unless path is a qcow2 with no backing chain (proof qemu-img create replaced content).
func assertFreshQcow2DiskOnHypervisor(path string, sshConfig *core.SSHConfig, knownHostsPath string) {
	qPath := quotePathForShell(path)
	cmd := fmt.Sprintf("sudo qemu-img info --output=json %s", qPath)
	stdout, stderr, err := core.ExecuteSSHCommand(cmd, sshConfig, knownHostsPath)
	o.Expect(err).To(o.BeNil(), "qemu-img info after disk recreate failed for %s (stderr=%q)", path, strings.TrimSpace(stderr))
	var meta struct {
		Format              string `json:"format"`
		VirtualSize         int64  `json:"virtual-size"`
		BackingFilename     string `json:"backing-filename"`
		FullBackingFilename string `json:"full-backing-filename"`
	}
	o.Expect(json.Unmarshal([]byte(stdout), &meta)).To(o.Succeed(), "parse qemu-img JSON for %s", path)
	o.Expect(strings.ToLower(meta.Format)).To(o.Equal("qcow2"), "expected qcow2 at %s after recreate, got format=%q", path, meta.Format)
	o.Expect(meta.BackingFilename).To(o.BeEmpty(), "fresh qcow2 at %s must not reference backing-filename (would chain old layers)", path)
	o.Expect(meta.FullBackingFilename).To(o.BeEmpty(), "fresh qcow2 at %s must not reference full-backing-filename", path)
	e2e.Logf("[disk replace] %s format=%s virtual-size=%d no_backing", path, meta.Format, meta.VirtualSize)
}

// recreateTargetVM recreates the target VM using backed up configuration
func recreateTargetVM(testConfig *TNFTestConfig, backupDir string) {
	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before recreating VM")
	// Read the backed up XML
	xmlFile := filepath.Join(backupDir, testConfig.TargetNode.VMName+".xml")
	xmlContent, err := os.ReadFile(xmlFile)
	o.Expect(err).To(o.BeNil(), "Expected to read XML backup without error")
	xmlOutput := string(xmlContent)

	// virsh destroy/undefine does not delete disk images on the hypervisor. If we defined+started the VM without
	// replacing those files first, the guest would boot as its old self.
	// Support file and volume-backed disks by resolving volume refs via virsh vol-path.
	diskRefs, err := services.ExtractDiskSourceRefs(xmlOutput)
	o.Expect(err).To(o.BeNil(), "Expected to extract disk refs from VM XML")
	o.Expect(diskRefs).NotTo(o.BeEmpty(),
		"refuse to define/start VM without disk wipe: dumpxml must list disk sources (file or volume), otherwise old OS/OVS identity could persist on hypervisor")

	sshConfig := &testConfig.Hypervisor.Config
	knownHostsPath := testConfig.Hypervisor.KnownHostsPath
	seenDisk := make(map[string]struct{})
	for _, ref := range diskRefs {
		path, pathErr := resolveDiskSourceRefPath(ref, sshConfig, knownHostsPath)
		o.Expect(pathErr).To(o.BeNil(), "resolve disk source path (type=%s pool=%q volume=%q file=%q)", ref.Type, ref.Pool, ref.Volume, ref.FilePath)
		mustNotCollideWithSurvivorDisk(testConfig, path)
		if _, dup := seenDisk[path]; dup {
			e2e.Logf("Skipping duplicate disk path from domain XML: %s", path)
			continue
		}
		seenDisk[path] = struct{}{}

		size := getDiskSizeFromHypervisor(path, defaultFreshDiskSize, sshConfig, knownHostsPath)
		qPath := quotePathForShell(path)
		backupPath := path + backingDiskBackupSuffix
		qBackup := quotePathForShell(backupPath)
		e2e.Logf("[disk replace] %s -> %s then empty qcow2 %s %s", path, backupPath, path, size)
		// If the old image exists, mv must succeed (no "|| true" — a failed rename could leave old content for the guest).
		// If the image file is already absent, skip mv; create still yields an empty qcow2 install target for Ironic.
		moveCmd := fmt.Sprintf("if sudo test -f %s; then sudo mv -f %s %s || exit 1; fi", qPath, qPath, qBackup)
		_, moveStderr, moveErr := core.ExecuteSSHCommand(moveCmd, sshConfig, knownHostsPath)
		o.Expect(moveErr).To(o.BeNil(), "move old disk aside for %s (stderr=%q)", path, strings.TrimSpace(moveStderr))
		createCmd := fmt.Sprintf("sudo qemu-img create -f qcow2 %s %s", qPath, size)
		_, createStderr, createErr := core.ExecuteSSHCommand(createCmd, sshConfig, knownHostsPath)
		o.Expect(createErr).To(o.BeNil(), "create fresh qcow2 at %s size %s (stderr=%q)", path, size, strings.TrimSpace(createStderr))
		assertFreshQcow2DiskOnHypervisor(path, sshConfig, knownHostsPath)
	}
	e2e.Logf("[disk replace] all disk sources (file/volume) resolved and replaced (qemu-img proof per path); guest cannot boot old OS from those paths")

	// Create XML file on the hypervisor using secure method
	xmlPath := fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.CreateRemoteFile(xmlPath, xmlOutput, core.StandardFileMode, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to create XML file on hypervisor without error")

	// Redefine the VM using the backed up XML
	err = services.VirshDefineVM(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to define VM without error")

	// Start the VM with autostart enabled
	err = services.VirshStartVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to start VM without error")

	err = services.VirshAutostartVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to enable autostart for VM without error")

	// Clean up temporary XML file
	err = core.DeleteRemoteTempFile(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to clean up temporary XML file without error")

	err = services.WaitForVMState(testConfig.TargetNode.VMName, services.VMStateRunning, vmLibvirtRunningTimeout, utils.ThirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to wait for VM to start without error")
}

// provisionTargetNodeWithIronic handles the Ironic provisioning process
func provisionTargetNodeWithIronic(testConfig *TNFTestConfig, oc *exutil.CLI) {
	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before provisioning with Ironic")

	// Set flag to indicate we're attempting node provisioning
	testConfig.Execution.HasAttemptedNodeProvisioning = true

	recreateBMCSecret(testConfig, oc)
	newUUID, newMACAddress, err := services.GetVMNetworkInfo(testConfig.TargetNode.VMName, virshProvisioningBridge, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to get VM network info: %v", err)
	updateAndCreateBMH(testConfig, oc, newUUID, newMACAddress)
	waitForBMHProvisioning(testConfig, oc)
	reapplyDetachedAnnotation(testConfig, oc)
	recreateMachine(testConfig, oc)
}

// waitForMachineToHaveNodeRef waits for the Machine to have status.nodeRef.name set to the expected node name
// via the typed cluster client (stable polling without shelling out to oc).
func waitForMachineToHaveNodeRef(testConfig *TNFTestConfig, oc *exutil.CLI, timeout time.Duration) {
	expectedNodeName := testConfig.TargetNode.Name
	machineName := testConfig.TargetNode.MachineName
	e2e.Logf("Waiting for Machine %s to have nodeRef.name=%s (timeout: %v)", machineName, expectedNodeName, timeout)
	err := core.PollUntil(func() (bool, error) {
		st, err := apis.GetMachineStatus(oc, machineName, machineAPINamespace)
		if err != nil {
			e2e.Logf("Machine %s: get failed: %v", machineName, err)
			return false, nil
		}
		if st.NotFound {
			e2e.Logf("Machine %s not found yet, continuing to poll", machineName)
			return false, nil
		}
		if st.NodeRef == expectedNodeName {
			e2e.Logf("Machine %s has nodeRef.name=%s", machineName, st.NodeRef)
			return true, nil
		}
		e2e.Logf("Machine %s nodeRef.name=%q (want %q), continuing to poll", machineName, st.NodeRef, expectedNodeName)
		return false, nil
	}, timeout, machineNodeRefPollInterval, fmt.Sprintf("Machine %s to have nodeRef.name=%s", machineName, expectedNodeName))
	o.Expect(err).To(o.BeNil(), "Expected Machine %s to get nodeRef.name=%s before node-Ready wait", machineName, expectedNodeName)
}

// logMachineStatus logs Machine phase and nodeRef via the cluster API (for diagnostics when waiting for node Ready).
func logMachineStatus(oc *exutil.CLI, machineName string) {
	st, err := apis.GetMachineStatus(oc, machineName, machineAPINamespace)
	if err != nil {
		e2e.Logf("Machine %s: get failed (may not exist yet): %v", machineName, err)
		return
	}
	if st.NotFound {
		e2e.Logf("Machine %s: not found", machineName)
		return
	}
	e2e.Logf("Machine %s: phase=%s nodeRef=%s", machineName, st.Phase, st.NodeRef)
}

// waitForNodeRecovery monitors for the replacement node to become Ready. Call after the node-bootstrapper
// CSR has been approved (e.g. via apis.WaitForAndApproveNodeBootstrapperCSR) so the timeout applies to
// network/container bring-up, not CSR approval.
// While polling, logs Machine status and node CSR approval status so we can see if the machine-approver
// has approved the node's CSRs (required for the node to become Ready).
// While polling Ready, if k8s.ovn.org/node-chassis-id still equals the pre-replacement chassis name, strip it and
// recycle ovnkube-node on the target so OVN-K can converge (same failure mode as recoverClusterFromBackup step 8).
// Returns the time at which the node became Ready (for gating update-setup job waits) and any error.
func waitForNodeRecovery(testConfig *TNFTestConfig, oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration) (time.Time, error) {
	var readyTime time.Time
	var loggedReplacementNodeFirstSighting bool
	err := core.PollUntil(func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		defer cancel()
		_, errGet := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testConfig.TargetNode.Name, metav1.GetOptions{})
		if errGet == nil && !loggedReplacementNodeFirstSighting {
			loggedReplacementNodeFirstSighting = true
			logNodeOVNChassisWebhookDiag(oc, testConfig.TargetNode.Name, "replacement Node first observed in API after re-registration (may be NotReady)")
		}
		if errGet == nil && removePreReplacementChassisFromNodeAndRecycleOVNKubeNode(oc, testConfig.TargetNode.Name, testConfig.Execution.PreReplacementChassisID) {
			e2e.Logf("[OVN chassis recovery] stripped pre-replacement chassis id while waiting for %s to become Ready", testConfig.TargetNode.Name)
		}
		// Check if the target node exists and is Ready
		if utils.IsNodeReady(oc, testConfig.TargetNode.Name) {
			readyTime = time.Now()
			logNodeOVNChassisWebhookDiag(oc, testConfig.TargetNode.Name, "replacement Node when Ready")
			e2e.Logf("Node %s is now Ready at %v", testConfig.TargetNode.Name, readyTime.UTC())
			return true, nil
		}

		// Node doesn't exist or is not Ready yet: log Machine and CSR status for diagnostics
		logMachineStatus(oc, testConfig.TargetNode.MachineName)
		apis.LogNodeCSRStatus(oc, testConfig.TargetNode.Name)
		e2e.Logf("Node %s is not Ready yet, continuing to poll", testConfig.TargetNode.Name)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("node %s to be Ready", testConfig.TargetNode.Name))
	if err != nil {
		return time.Time{}, err
	}
	return readyTime, nil
}

// eastWestCheckName returns the PodNetworkConnectivityCheck name for connectivity from surviving node to target node's network-check-target pod.
func eastWestCheckName(survivingNodeName, targetNodeName string) string {
	return fmt.Sprintf("network-check-source-%s-to-network-check-target-%s", survivingNodeName, targetNodeName)
}

// waitForEastWestConnectivity waits until the PodNetworkConnectivityCheck from the surviving node to the target node's network-check-target reports Reachable=True.
// This verifies that east-west pod-to-pod traffic works after node replacement (OVN SB/NB must have consistent view of both chassis and port_bindings).
func waitForEastWestConnectivity(oc *exutil.CLI, survivingNodeName, targetNodeName string, timeout time.Duration) error {
	checkName := eastWestCheckName(survivingNodeName, targetNodeName)
	e2e.Logf("[east-west] Waiting up to %v for PodNetworkConnectivityCheck %s (surviving=%s -> target=%s)", timeout, checkName, survivingNodeName, targetNodeName)
	return core.PollUntil(func() (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args(
			"podnetworkconnectivitycheck", checkName,
			"-n", networkDiagnosticsNamespace,
			"-o", "jsonpath={.status.conditions[?(@.type==\"Reachable\")].status}",
		).Output()
		if err != nil {
			e2e.Logf("[east-west] Get check %s: %v (may not exist yet)", checkName, err)
			return false, nil
		}
		status := strings.TrimSpace(string(out))
		if status == "True" {
			e2e.Logf("[east-west] Check %s is Reachable=True", checkName)
			return true, nil
		}
		e2e.Logf("[east-west] Check %s status=%q, continuing to poll", checkName, status)
		return false, nil
	}, timeout, eastWestConnectivityPollInterval, fmt.Sprintf("east-west connectivity %s -> %s", survivingNodeName, targetNodeName))
}

// forceStaticPodRevisionBump patches kube-apiserver, kube-controller-manager, and the scheduler operator
// to a new logLevel (Trace) then back to Normal so static pod installers run again on all control-plane nodes.
// Operators may not roll static-pod installers on a replacement control-plane node without a spec change; this forces a new revision.
// Scheduler is exposed as openshiftkubescheduler on some releases and kubescheduler on others; we try both.
func forceStaticPodRevisionBump(oc *exutil.CLI) {
	coreOperators := []string{"kubeapiserver", "kubecontrollermanager"}
	schedulerOperatorCandidates := []string{"openshiftkubescheduler", "kubescheduler"}

	patchLogLevel := func(resource, level string) error {
		_, err := oc.AsAdmin().Run("patch").Args(resource, "cluster", "--type=merge", "-p", fmt.Sprintf(`{"spec":{"logLevel":"%s"}}`, level)).Output()
		return err
	}
	for _, name := range coreOperators {
		if err := patchLogLevel(name, "Trace"); err != nil {
			e2e.Logf("[static-pod revision] Trace patch %s: %v", name, err)
			continue
		}
		e2e.Logf("[static-pod revision] Patched %s to Trace to force new revision", name)
	}
	schedulerBumped := false
	for _, name := range schedulerOperatorCandidates {
		if err := patchLogLevel(name, "Trace"); err != nil {
			e2e.Logf("[static-pod revision] Trace patch %s: %v", name, err)
			continue
		}
		e2e.Logf("[static-pod revision] Patched %s to Trace to force new revision", name)
		schedulerBumped = true
		break
	}
	if !schedulerBumped {
		e2e.Logf("[static-pod revision] WARNING: no scheduler operator accepted Trace patch (tried openshiftkubescheduler, kubescheduler)")
	}
	time.Sleep(staticPodRevisionBumpSettleWait)
	for _, name := range coreOperators {
		if err := patchLogLevel(name, "Normal"); err != nil {
			e2e.Logf("[static-pod revision] Revert %s to Normal: %v", name, err)
			continue
		}
		e2e.Logf("[static-pod revision] Reverted %s to Normal", name)
	}
	for _, name := range schedulerOperatorCandidates {
		if err := patchLogLevel(name, "Normal"); err != nil {
			continue
		}
		e2e.Logf("[static-pod revision] Reverted %s to Normal", name)
	}
}

// controlPlaneStaticPodManifestSSHCheck runs on the node (as core); sudo reads /etc/kubernetes/manifests (root-owned).
// Scheduler: openshift-kube-scheduler-pod.yaml on OCP; kube-scheduler-pod.yaml as fallback.
const controlPlaneStaticPodManifestSSHCheck = "sudo test -f /etc/kubernetes/manifests/kube-apiserver-pod.yaml && " +
	"sudo test -f /etc/kubernetes/manifests/kube-controller-manager-pod.yaml && " +
	"( sudo test -f /etc/kubernetes/manifests/openshift-kube-scheduler-pod.yaml || " +
	"sudo test -f /etc/kubernetes/manifests/kube-scheduler-pod.yaml )"

// waitForControlPlaneStaticPodManifestsOnNode polls until apiserver, KCM, and scheduler manifests exist on the node.
// Uses two-hop SSH (hypervisor → node) like other TNF node operations; requires TargetNode.KnownHostsPath prepared on the hypervisor.
func waitForControlPlaneStaticPodManifestsOnNode(testConfig *TNFTestConfig, nodeName string) error {
	if nodeName != testConfig.TargetNode.Name {
		return fmt.Errorf("static pod manifest SSH check expects replacement target %q, got %q", testConfig.TargetNode.Name, nodeName)
	}
	if testConfig.TargetNode.KnownHostsPath == "" {
		return fmt.Errorf("TargetNode.KnownHostsPath is empty; call PrepareRemoteKnownHostsFile for %s before waitForControlPlaneStaticPodManifestsOnNode", testConfig.TargetNode.IP)
	}
	return core.PollUntil(func() (bool, error) {
		_, stderr, err := core.ExecuteRemoteSSHCommand(
			testConfig.TargetNode.IP,
			controlPlaneStaticPodManifestSSHCheck,
			&testConfig.Hypervisor.Config,
			testConfig.Hypervisor.KnownHostsPath,
			testConfig.TargetNode.KnownHostsPath,
		)
		if err != nil {
			e2e.Logf("[static-pod manifests] SSH to %s (%s): %v; stderr=%q", nodeName, testConfig.TargetNode.IP, err, strings.TrimSpace(stderr))
			return false, nil
		}
		e2e.Logf("[static-pod manifests] node %s: kube-apiserver, kube-controller-manager, and scheduler manifests present", nodeName)
		return true, nil
	}, staticPodManifestsWaitTimeout, staticPodManifestsPollInterval, fmt.Sprintf("control-plane static pod manifests on node %s", nodeName))
}

// evaporateOvnkubeNodePodsOnNode deletes app=ovnkube-node pods on nodeName in openshift-ovn-kubernetes (grace 0).
// Recycling ovnkube-node restarts the sbdb sidecar and refreshes the local SB view without deleting every OVN-K pod.
func evaporateOvnkubeNodePodsOnNode(oc *exutil.CLI, ctx context.Context, nodeName string) error {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=ovnkube-node",
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return fmt.Errorf("list ovnkube-node in %s on node %s: %w", ovnKubernetesNamespace, nodeName, err)
	}
	if len(pods.Items) == 0 {
		e2e.Logf("[OVN-K ovnkube-node recycle] node %s: no ovnkube-node pod in %s (already gone or not scheduled yet)", nodeName, ovnKubernetesNamespace)
		return nil
	}
	grace := int64(0)
	opts := metav1.DeleteOptions{GracePeriodSeconds: &grace}
	e2e.Logf("[OVN-K ovnkube-node recycle] node %s: deleting %d ovnkube-node pod(s) in %s (grace=0)", nodeName, len(pods.Items), ovnKubernetesNamespace)
	for i := range pods.Items {
		pod := pods.Items[i]
		e2e.Logf("[OVN-K ovnkube-node recycle] delete pod %s (node=%s phase=%s)", pod.Name, nodeName, pod.Status.Phase)
		if delErr := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).Delete(ctx, pod.Name, opts); delErr != nil {
			if apierrors.IsNotFound(delErr) {
				continue
			}
			return fmt.Errorf("delete pod %s on %s: %w", pod.Name, nodeName, delErr)
		}
	}
	return nil
}

// recoverOVNKForNodeReplacement restarts ovnkube-node on the replacement (target) first, waits for it to be Ready,
// then restarts ovnkube-node on the survivor and every ovnkube-control-plane pod, then waits for ovnkube-node Ready
// on both nodes. Used from recoverClusterFromBackup step 9 after step 8 when the spec failed; see recoverClusterFromBackup
// header for why. The main replacement flow relies on SB chassis-del during deleteNodeReferences and does not perform
// this full restart after the replacement node is up.
//
// Order: delete target ovnkube-node → wait target Ready → delete survivor ovnkube-node → delete ovnkube-control-plane → wait both Ready.
func recoverOVNKForNodeReplacement(oc *exutil.CLI, survivingNodeName, targetNodeName string) error {
	e2e.Logf("[east-west recovery] OVN-K recovery: ovnkube-node on replacement %s first, wait Ready; then survivor %s + ovnkube-control-plane; wait both Ready", targetNodeName, survivingNodeName)
	ctx, cancel := context.WithTimeout(context.Background(), ovnkubeRecoveryAPITimeout)
	defer cancel()

	deleteOvnkubeNodeOn := func(nodeName string) error {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=ovnkube-node",
			FieldSelector: "spec.nodeName=" + nodeName,
		})
		if err != nil {
			return fmt.Errorf("list ovnkube-node on %s: %w", nodeName, err)
		}
		if len(pods.Items) == 0 {
			e2e.Logf("[east-west recovery] no ovnkube-node pod on %s to delete (may still be scheduling)", nodeName)
			return nil
		}
		for i := range pods.Items {
			podName := pods.Items[i].Name
			e2e.Logf("[east-west recovery] Deleting ovnkube-node pod %s (node %s)", podName, nodeName)
			if delErr := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).Delete(ctx, podName, metav1.DeleteOptions{}); delErr != nil {
				return fmt.Errorf("delete ovnkube-node %s on %s: %w", podName, nodeName, delErr)
			}
		}
		return nil
	}
	if err := deleteOvnkubeNodeOn(targetNodeName); err != nil {
		return err
	}
	e2e.Logf("[east-west recovery] Waiting for ovnkube-node Ready on replacement %s before recycling survivor", targetNodeName)
	if err := waitForOVNKubeNodeReadyOnSingleNode(oc, targetNodeName, ovnkubeNodeAfterRestartWaitTimeout, ovnkubeNodeAfterRestartPollInterval); err != nil {
		return fmt.Errorf("ovnkube-node on replacement %s Ready after recycle: %w", targetNodeName, err)
	}
	if err := deleteOvnkubeNodeOn(survivingNodeName); err != nil {
		return err
	}

	cpPods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=ovnkube-control-plane",
	})
	if err != nil {
		return fmt.Errorf("list ovnkube-control-plane: %w", err)
	}
	for i := range cpPods.Items {
		podName := cpPods.Items[i].Name
		e2e.Logf("[east-west recovery] Deleting ovnkube-control-plane pod %s", podName)
		if delErr := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).Delete(ctx, podName, metav1.DeleteOptions{}); delErr != nil {
			return fmt.Errorf("delete ovnkube-control-plane %s: %w", podName, delErr)
		}
	}

	e2e.Logf("[east-west recovery] Waiting for ovnkube-node Ready on survivor %s and replacement %s", survivingNodeName, targetNodeName)
	if err := waitForOVNKubeNodePodsReadyOnNodes(oc, survivingNodeName, targetNodeName, ovnkubeNodeAfterRestartWaitTimeout, ovnkubeNodeAfterRestartPollInterval); err != nil {
		return err
	}
	e2e.Logf("[east-west recovery] OVN-K ovnkube-node pods are Ready on both nodes; additional settle: %v", ovnkubeRestartSettleWait)
	return nil
}

// waitForOVNKubeNodeReadyOnSingleNode polls until nodeName has a Running ovnkube-node pod with Ready=True.
func waitForOVNKubeNodeReadyOnSingleNode(oc *exutil.CLI, nodeName string, timeout, interval time.Duration) error {
	return core.PollUntil(func() (bool, error) {
		listCtx, listCancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		pods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(listCtx, metav1.ListOptions{
			LabelSelector: "app=ovnkube-node",
			FieldSelector: "spec.nodeName=" + nodeName,
		})
		listCancel()
		if err != nil {
			e2e.Logf("[post-node-delete OVN] list ovnkube-node on %s: %v", nodeName, err)
			return false, nil
		}
		if len(pods.Items) == 0 {
			e2e.Logf("[post-node-delete OVN] no ovnkube-node pod on %s yet", nodeName)
			return false, nil
		}
		pod := pods.Items[0]
		ready := false
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if !ready {
			e2e.Logf("[post-node-delete OVN] ovnkube-node %s on %s not Ready (phase=%s)", pod.Name, nodeName, pod.Status.Phase)
			return false, nil
		}
		e2e.Logf("[post-node-delete OVN] ovnkube-node Ready on %s (%s)", nodeName, pod.Name)
		return true, nil
	}, timeout, interval, fmt.Sprintf("ovnkube-node Ready on %s", nodeName))
}

// waitForOVNKubeNodePodsReadyOnNodes polls until each node has a Running ovnkube-node pod with Ready=True.
func waitForOVNKubeNodePodsReadyOnNodes(oc *exutil.CLI, survivingNodeName, targetNodeName string, timeout, interval time.Duration) error {
	return core.PollUntil(func() (bool, error) {
		for _, nodeName := range []string{survivingNodeName, targetNodeName} {
			listCtx, listCancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
			pods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(listCtx, metav1.ListOptions{
				LabelSelector: "app=ovnkube-node",
				FieldSelector: "spec.nodeName=" + nodeName,
			})
			listCancel()
			if err != nil {
				e2e.Logf("[east-west recovery] list ovnkube-node on %s: %v", nodeName, err)
				return false, nil
			}
			if len(pods.Items) == 0 {
				e2e.Logf("[east-west recovery] no ovnkube-node pod on %s yet", nodeName)
				return false, nil
			}
			pod := pods.Items[0]
			ready := false
			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					ready = true
					break
				}
			}
			if !ready {
				e2e.Logf("[east-west recovery] ovnkube-node %s on %s not Ready (phase=%s)", pod.Name, nodeName, pod.Status.Phase)
				return false, nil
			}
		}
		e2e.Logf("[east-west recovery] ovnkube-node Ready on survivor %s and replacement %s", survivingNodeName, targetNodeName)
		return true, nil
	}, timeout, interval, fmt.Sprintf("ovnkube-node Ready on survivor %s and replacement %s", survivingNodeName, targetNodeName))
}

// queryOVNSBPortBindingsFrom runs ovn-sbctl in fromNodeName's ovnkube-node sbdb container: find chassis for
// peerNodeName (hostname=), then list port_binding rows for that chassis. Used for diagnostics and polling.
func queryOVNSBPortBindingsFrom(oc *exutil.CLI, fromNodeName, peerNodeName string) (chassisUUID string, logicalPorts []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	pods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=ovnkube-node",
		FieldSelector: "spec.nodeName=" + fromNodeName,
	})
	if err != nil {
		return "", nil, fmt.Errorf("list ovnkube-node on %s: %w", fromNodeName, err)
	}
	if len(pods.Items) == 0 {
		return "", nil, fmt.Errorf("no ovnkube-node pod on node %s", fromNodeName)
	}
	var podName string
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning {
			podName = pods.Items[i].Name
			break
		}
	}
	if podName == "" {
		return "", nil, fmt.Errorf("no running ovnkube-node pod on %s", fromNodeName)
	}
	out, err := oc.AsAdmin().Run("exec").Args("-n", ovnKubernetesNamespace, podName, "-c", ovnkubeNodeSBDBContainer, "--",
		"ovn-sbctl", "--columns=_uuid", "--no-headings", "find", "chassis", fmt.Sprintf("hostname=%q", peerNodeName)).Output()
	if err != nil {
		return "", nil, fmt.Errorf("ovn-sbctl find chassis hostname=%s from pod %s: %w", peerNodeName, podName, err)
	}
	chassisUUID = strings.TrimSpace(string(out))
	if chassisUUID == "" {
		return "", nil, fmt.Errorf("chassis for node %s not found in SB view from %s (empty uuid)", peerNodeName, fromNodeName)
	}
	out, err = oc.AsAdmin().Run("exec").Args("-n", ovnKubernetesNamespace, podName, "-c", ovnkubeNodeSBDBContainer, "--",
		"ovn-sbctl", "--columns=logical_port", "--no-headings", "find", "port_binding", "chassis="+chassisUUID).Output()
	if err != nil {
		return "", nil, fmt.Errorf("ovn-sbctl find port_binding chassis=%s from pod %s: %w", chassisUUID, podName, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			logicalPorts = append(logicalPorts, line)
		}
	}
	return chassisUUID, logicalPorts, nil
}

// verifyOVNSBPortBindingsAfterNodeReplacement ensures each node's chassis has at least minPortBindingsForNodeChassis
// port_bindings visible from the other node's SB view (gateway binding minimum). Call after recoverOVNKForNodeReplacement.
// SB programming/replication can lag east-west connectivity; poll with logging before failing.
func verifyOVNSBPortBindingsAfterNodeReplacement(oc *exutil.CLI, survivingNodeName, targetNodeName string) {
	peerPairs := []struct {
		fromNode, peerNode string
	}{
		{survivingNodeName, targetNodeName},
		{targetNodeName, survivingNodeName},
	}
	err := core.PollUntil(func() (bool, error) {
		for _, pair := range peerPairs {
			chassisUUID, logicalPorts, qerr := queryOVNSBPortBindingsFrom(oc, pair.fromNode, pair.peerNode)
			if qerr != nil {
				e2e.Logf("[OVN SB poll] SB view from %s (chassis hostname=%s): %v", pair.fromNode, pair.peerNode, qerr)
				return false, nil
			}
			n := len(logicalPorts)
			e2e.Logf("[OVN SB poll] SB view from %s (chassis hostname=%s): chassis_uuid=%s port_binding_count=%d logical_ports=%v (need >= %d)",
				pair.fromNode, pair.peerNode, chassisUUID, n, logicalPorts, minPortBindingsForNodeChassis)
			if n < minPortBindingsForNodeChassis {
				return false, nil
			}
		}
		e2e.Logf("[OVN SB poll] OK: both peer SB views report >= %d port_bindings per chassis (survivor=%s target=%s)",
			minPortBindingsForNodeChassis, survivingNodeName, targetNodeName)
		return true, nil
	}, ovnSBPortBindingsWaitTimeout, ovnSBPortBindingsPollInterval,
		fmt.Sprintf("OVN SB port_bindings (>=%d) for chassis of %s and %s in each peer's view", minPortBindingsForNodeChassis, survivingNodeName, targetNodeName))
	o.Expect(err).To(o.BeNil(),
		"OVN SB port_bindings did not converge within %v (poll %v): each node's chassis must show >= %d binding(s) from the peer's ovnkube-node SB view (gateway minimum). See [OVN SB poll] logs.",
		ovnSBPortBindingsWaitTimeout, ovnSBPortBindingsPollInterval, minPortBindingsForNodeChassis)
}

// restorePacemakerCluster waits for CEO to restore the pacemaker cluster configuration.
// The critical gate is the update-setup job on the surviving node; that job runs node add/remove,
// fencing, etcd updates, and "pcs cluster start --all". The job on the replacement node only
// exits early (cluster not running on that node). We wait for a run of the survivor's job that
// started after the replacement node was Ready, so we are not gating on an earlier run that
// completed before the new node was up.
