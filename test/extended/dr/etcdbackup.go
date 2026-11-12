package dr

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	etcdBackupTestHostPath = "/etc/kubernetes/cluster-backup"
	etcdBackupStagePodName = "etcd-backup-stage-pod"
)

// Use the dynamic client until Origin vendors the new EtcdBackup API.
var etcdBackupGVR = schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "etcdbackups"}

type etcdBackupFiles struct {
	snapshot        string
	staticResources string
}

// supportsEtcdBackupRecoveryAPI checks the live CRD because the feature gate also enabled an older API.
func supportsEtcdBackupRecoveryAPI(oc *exutil.CLI) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	crd, err := oc.AdminAPIExtensionsV1Client().CustomResourceDefinitions().Get(ctx, "etcdbackups.operator.openshift.io", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read EtcdBackup CRD: %w", err)
	}
	return supportsEtcdBackupRecoverySchema(crd)
}

// supportsEtcdBackupRecoverySchema skips the old PVC API but rejects unknown or incomplete schemas.
func supportsEtcdBackupRecoverySchema(crd *apiextensionsv1.CustomResourceDefinition) (bool, error) {
	for _, version := range crd.Spec.Versions {
		if version.Name != "v1alpha1" || !version.Served {
			continue
		}
		if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
			break
		}
		properties := version.Schema.OpenAPIV3Schema.Properties
		spec, hasSpec := properties["spec"]
		status, hasStatus := properties["status"]
		if !hasSpec || !hasStatus {
			break
		}
		_, hasNodeSelector := spec.Properties["nodeSelector"]
		_, hasStorage := spec.Properties["storage"]
		_, hasNodeName := status.Properties["nodeName"]
		_, hasFiles := status.Properties["files"]
		if hasNodeSelector && hasStorage && hasNodeName && hasFiles {
			return true, nil
		}

		_, hasPVCName := spec.Properties["pvcName"]
		_, hasBackupJob := status.Properties["backupJob"]
		if hasPVCName && hasBackupJob && !hasNodeSelector && !hasStorage && !hasNodeName && !hasFiles {
			return false, nil
		}
		break
	}
	return false, fmt.Errorf("EtcdBackup v1alpha1 CRD has neither the legacy PVC API nor the API required for local recovery backups")
}

// createEtcdBackup returns its name even on error so a timed-out request can still be cleaned up.
func createEtcdBackup(oc *exutil.CLI, node *corev1.Node) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	name := fmt.Sprintf("origin-recovery-%d", time.Now().UnixNano())
	backup := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.openshift.io/v1alpha1",
		"kind":       "EtcdBackup",
		"metadata":   map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"nodeSelector": map[string]interface{}{"kubernetes.io/hostname": node.Labels["kubernetes.io/hostname"]},
			"storage": map[string]interface{}{
				"type":  "Local",
				"local": map[string]interface{}{"hostPath": etcdBackupTestHostPath},
			},
		},
	}}
	_, err := oc.AdminDynamicClient().Resource(etcdBackupGVR).Create(ctx, backup, metav1.CreateOptions{})
	if err != nil {
		return name, fmt.Errorf("create EtcdBackup %q: %w", name, err)
	}
	return name, nil
}

// waitForEtcdBackup verifies the selected node and backup files before recovery begins.
func waitForEtcdBackup(oc *exutil.CLI, name string, node *corev1.Node) (etcdBackupFiles, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()

	var files etcdBackupFiles
	err := wait.PollUntilContextCancel(ctx, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		backup, err := oc.AdminDynamicClient().Resource(etcdBackupGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("waiting to read EtcdBackup %q: %v", name, err)
			return false, nil
		}
		conditions, _, err := unstructured.NestedSlice(backup.Object, "status", "conditions")
		if err != nil {
			return false, fmt.Errorf("read EtcdBackup conditions: %w", err)
		}
		completed := false
		for _, value := range conditions {
			condition, ok := value.(map[string]interface{})
			if !ok {
				return false, fmt.Errorf("invalid EtcdBackup condition: %v", value)
			}
			typeName, found, err := unstructured.NestedString(condition, "type")
			if err != nil {
				return false, fmt.Errorf("read EtcdBackup condition type: %w", err)
			}
			if !found {
				return false, fmt.Errorf("EtcdBackup condition has no type")
			}
			status, found, err := unstructured.NestedString(condition, "status")
			if err != nil {
				return false, fmt.Errorf("read EtcdBackup condition status: %w", err)
			}
			if !found {
				return false, fmt.Errorf("EtcdBackup condition has no status")
			}
			if status != "True" {
				continue
			}
			if typeName == "Failed" {
				reason, _, err := unstructured.NestedString(condition, "reason")
				if err != nil {
					return false, fmt.Errorf("read EtcdBackup failure reason: %w", err)
				}
				message, _, err := unstructured.NestedString(condition, "message")
				if err != nil {
					return false, fmt.Errorf("read EtcdBackup failure message: %w", err)
				}
				return false, fmt.Errorf("EtcdBackup %q failed: %s: %s", name, reason, message)
			}
			if typeName == "Completed" {
				completed = true
			}
		}
		if completed {
			actualNode, _, err := unstructured.NestedString(backup.Object, "status", "nodeName")
			if err != nil {
				return false, fmt.Errorf("read EtcdBackup %q nodeName: %w", name, err)
			}
			if actualNode != node.Name {
				return false, fmt.Errorf("EtcdBackup %q ran on an unexpected node", name)
			}
			files, err = readEtcdBackupFiles(backup, name)
			return err == nil, err
		}
		return false, nil
	})
	if err != nil {
		return etcdBackupFiles{}, fmt.Errorf("wait for EtcdBackup %q: %w", name, err)
	}
	return files, nil
}

// readEtcdBackupFiles rejects unexpected paths before using them in recovery shell commands.
func readEtcdBackupFiles(backup *unstructured.Unstructured, name string) (etcdBackupFiles, error) {
	items, found, err := unstructured.NestedSlice(backup.Object, "status", "files")
	if err != nil {
		return etcdBackupFiles{}, fmt.Errorf("read EtcdBackup %q files: %w", name, err)
	}
	if !found || len(items) != 2 {
		return etcdBackupFiles{}, fmt.Errorf("EtcdBackup %q must report two files, got %d", name, len(items))
	}

	expectedDir := filepath.Join(etcdBackupTestHostPath, name)
	var files etcdBackupFiles
	for _, value := range items {
		file, ok := value.(map[string]interface{})
		if !ok {
			return etcdBackupFiles{}, fmt.Errorf("EtcdBackup %q reported invalid file: %v", name, value)
		}
		assetType, _, err := unstructured.NestedString(file, "assetType")
		if err != nil {
			return etcdBackupFiles{}, err
		}
		path, _, err := unstructured.NestedString(file, "path")
		if err != nil || filepath.Dir(path) != expectedDir || filepath.Clean(path) != path {
			return etcdBackupFiles{}, fmt.Errorf("EtcdBackup %q reported unexpected file path %q: %v", name, path, err)
		}
		filename := filepath.Base(path)
		switch assetType {
		case "Snapshot":
			if files.snapshot != "" || !strings.HasPrefix(filename, "snapshot_") || !strings.HasSuffix(filename, ".db") {
				return etcdBackupFiles{}, fmt.Errorf("EtcdBackup %q reported invalid snapshot path %q", name, path)
			}
			files.snapshot = path
		case "StaticKubeResources":
			if files.staticResources != "" || !strings.HasPrefix(filename, "static_kuberesources_") || !strings.HasSuffix(filename, ".tar.gz") {
				return etcdBackupFiles{}, fmt.Errorf("EtcdBackup %q reported invalid static resources path %q", name, path)
			}
			files.staticResources = path
		default:
			return etcdBackupFiles{}, fmt.Errorf("EtcdBackup %q reported unexpected asset type %q", name, assetType)
		}
	}
	return files, nil
}

// stageEtcdBackupFiles copies files before quorum is lost, so copy failures leave the cluster intact.
func stageEtcdBackupFiles(oc *exutil.CLI, node *corev1.Node, files etcdBackupFiles) error {
	stageScript := fmt.Sprintf(`
		#!/bin/bash
		set -euo pipefail
		P_KEY=$(mktemp /tmp/etcd-backup-key.XXXXXX)
		trap 'rm -f "$P_KEY"' EXIT
		cp %s/privKey "$P_KEY"
		chmod 600 "$P_KEY"
		TARGET_NODE_NAME=%s
		ssh -i "$P_KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -q "core@${TARGET_NODE_NAME}" <<'EOF'
			set -eu
			sudo test -s %s
			sudo test -s %s
			sudo rm -rf /home/core/backup
			sudo mkdir -p /home/core/backup
			sudo cp %s %s /home/core/backup/
			sudo chown -R core:core /home/core/backup
EOF
	`, sshPath, internalIP(node), shellQuote(files.snapshot), shellQuote(files.staticResources), shellQuote(files.snapshot), shellQuote(files.staticResources))

	// Host networking lets SSH reach the node despite openshift-etcd's default-deny egress policy.
	podSpec := applycorev1.PodSpec().WithHostNetwork(true).WithRestartPolicy(corev1.RestartPolicyOnFailure)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("stage-backup").
			// Use nobody from the base image so SSH can resolve the local user.
			WithSecurityContext(applycorev1.SecurityContext().
				WithRunAsUser(65534).
				WithRunAsNonRoot(true).
				WithAllowPrivilegeEscalation(false)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(applycorev1.VolumeMount().WithName("keys").WithMountPath(sshPath)).
			WithCommand("/bin/bash", "-c", stageScript),
	}
	podSpec.NodeSelector = map[string]string{"kubernetes.io/hostname": node.Labels["kubernetes.io/hostname"]}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}
	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName("dr-ssh")),
	}
	return runPodAndWaitForSuccess(oc, applycorev1.Pod(etcdBackupStagePodName, openshiftEtcdNamespace).WithSpec(podSpec))
}

// deleteEtcdBackup waits for the resource to disappear before the next recovery test can run.
func deleteEtcdBackup(oc *exutil.CLI, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	backups := oc.AdminDynamicClient().Resource(etcdBackupGVR)
	if err := backups.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete EtcdBackup %q: %w", name, err)
	}
	return wait.PollUntilContextCancel(ctx, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := backups.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			framework.Logf("waiting to read EtcdBackup %q for deletion: %v", name, err)
		}
		return false, nil
	})
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
