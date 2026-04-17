package node

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	stargzStoreNamespace      = "stargz-store"
	stargzStoreDaemonSetName  = "stargz-store-installer"
	stargzStoreServiceAccount = "stargz-store-installer"
	stargzStoreVersion        = "v0.18.2"
	stargzStorePath           = "/var/lib/stargz-store/store"
)

// StargzStoreSetup manages the lifecycle of stargz-store on cluster nodes
type StargzStoreSetup struct {
	oc              *exutil.CLI
	namespace       string
	deployed        bool
	targetNodeLabel string // Node label selector for DaemonSet (e.g., "node-role.kubernetes.io/worker")
}

// NewStargzStoreSetup creates a new StargzStoreSetup instance
// targetNodeLabel specifies which nodes to deploy stargz-store to (e.g., "node-role.kubernetes.io/worker")
func NewStargzStoreSetup(oc *exutil.CLI, targetNodeLabel string) *StargzStoreSetup {
	return &StargzStoreSetup{
		oc:              oc,
		namespace:       stargzStoreNamespace,
		deployed:        false,
		targetNodeLabel: targetNodeLabel,
	}
}

// Deploy installs stargz-store on target nodes using YAML
func (s *StargzStoreSetup) Deploy(ctx context.Context) error {
	framework.Logf("Deploying stargz-store using YAML approach...")

	// Generate YAML with dynamic node label
	yaml := s.generateYAML()

	// Write YAML to temp file
	tmpFile, err := os.CreateTemp("", "stargz-store-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		return fmt.Errorf("failed to write YAML: %w", err)
	}
	tmpFile.Close()

	framework.Logf("Created YAML file: %s", tmpFile.Name())

	// Apply YAML using oc
	cmd := exec.Command("oc", "apply", "-f", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply YAML: %w\nOutput: %s", err, string(output))
	}
	framework.Logf("Applied YAML successfully:\n%s", string(output))

	// Grant privileged SCC (YAML can't do this automatically)
	framework.Logf("Granting privileged SCC to serviceaccount...")
	if err := s.grantPrivilegedSCC(ctx); err != nil {
		return fmt.Errorf("failed to grant privileged SCC: %w", err)
	}

	// Mark as deployed
	s.deployed = true

	// Wait for DaemonSet to be ready
	framework.Logf("Waiting for DaemonSet to be ready...")
	if err := s.waitForDaemonSetReady(ctx, 10*time.Minute); err != nil {
		return fmt.Errorf("daemonset not ready: %w", err)
	}

	framework.Logf("stargz-store deployed successfully")
	return nil
}

// generateYAML creates the stargz-store YAML with dynamic node label
func (s *StargzStoreSetup) generateYAML() string {
	// This YAML is based on the manual setup script that works
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    app: stargz-store
    pod-security.kubernetes.io/enforce: privileged
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/warn: privileged
    security.openshift.io/scc.podSecurityLabelSync: "false"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: %s
  namespace: %s
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: stargz-store-config
  namespace: %s
data:
  config.toml: |
    # Stargz-store configuration for CRI-O
    [[resolver.host."quay.io".mirrors]]
    host = "quay.io"

    [[resolver.host."docker.io".mirrors]]
    host = "registry-1.docker.io"

    [[resolver.host."gcr.io".mirrors]]
    host = "gcr.io"

    [[resolver.host."ghcr.io".mirrors]]
    host = "ghcr.io"

    [[resolver.host."registry.redhat.io".mirrors]]
    host = "registry.redhat.io"

  stargz-store.service: |
    [Unit]
    Description=stargz store
    After=network.target
    Before=crio.service

    [Service]
    Type=notify
    Environment=HOME=/root
    ExecStart=/usr/local/bin/stargz-store --log-level=debug --config=/etc/stargz-store/config.toml /var/lib/stargz-store/store
    ExecStopPost=umount /var/lib/stargz-store/store
    Restart=always
    RestartSec=1

    [Install]
    WantedBy=multi-user.target
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    matchLabels:
      app: stargz-store-installer
  template:
    metadata:
      labels:
        app: stargz-store-installer
    spec:
      serviceAccountName: %s
      nodeSelector:
        %s: ""
      hostPID: true
      hostNetwork: true
      tolerations:
      - operator: Exists
      containers:
      - name: installer
        image: registry.access.redhat.com/ubi9/ubi:latest
        securityContext:
          privileged: true
        command:
        - /bin/bash
        - -c
        - |
          set -e

          echo "=== Stargz-store Installer ==="
          echo "Node: $(hostname)"

          # Check if already installed and running
          if nsenter -t 1 -m -u -i -n -p -- systemctl is-active stargz-store &>/dev/null; then
            echo "stargz-store already running on this node"
            nsenter -t 1 -m -u -i -n -p -- mount | grep stargz || true
            echo "Sleeping to keep pod running..."
            sleep infinity
          fi

          echo "Installing stargz-store..."

          # Unlock ostree for modifications
          echo "Unlocking ostree..."
          nsenter -t 1 -m -u -i -n -p -- ostree admin unlock --hotfix || echo "ostree unlock failed or already unlocked"

          # Download stargz-snapshotter release
          echo "Downloading stargz-snapshotter %s..."
          curl -L -o /tmp/stargz.tar.gz \
            https://github.com/containerd/stargz-snapshotter/releases/download/%s/stargz-snapshotter-%s-linux-amd64.tar.gz

          # Extract to host
          echo "Extracting binary to /usr/local/bin..."
          tar -xzf /tmp/stargz.tar.gz -C /tmp/
          cp /tmp/stargz-store /host/usr/local/bin/
          chmod +x /host/usr/local/bin/stargz-store

          # Verify binary
          echo "Verifying binary..."
          nsenter -t 1 -m -u -i -n -p -- /usr/local/bin/stargz-store --version || echo "Version check skipped"

          # Create directories
          echo "Creating directories..."
          mkdir -p /host/etc/stargz-store
          mkdir -p /host/var/lib/stargz-store/store

          # Copy config file
          echo "Copying config.toml..."
          cp /config/config.toml /host/etc/stargz-store/config.toml

          # Copy service file
          echo "Copying systemd service..."
          cp /config/stargz-store.service /host/etc/systemd/system/stargz-store.service

          # Reload systemd and enable service
          echo "Enabling stargz-store service..."
          nsenter -t 1 -m -u -i -n -p -- systemctl daemon-reload
          nsenter -t 1 -m -u -i -n -p -- systemctl enable stargz-store
          nsenter -t 1 -m -u -i -n -p -- systemctl start stargz-store

          # Wait for service to be ready
          echo "Waiting for stargz-store to be ready..."
          sleep 5

          # Verify service is running
          echo "Verifying stargz-store service..."
          nsenter -t 1 -m -u -i -n -p -- systemctl status stargz-store --no-pager || true

          # Verify FUSE mount
          echo "Verifying FUSE mount..."
          nsenter -t 1 -m -u -i -n -p -- mount | grep stargz || echo "WARNING: stargz mount not found"

          # Restart CRI-O to pick up the new layer store
          echo "Restarting CRI-O..."
          nsenter -t 1 -m -u -i -n -p -- systemctl restart crio

          echo "=== Setup complete! ==="
          echo "stargz-store is now running on $(hostname)"

          # Keep pod running
          sleep infinity
        volumeMounts:
        - name: host-root
          mountPath: /host
        - name: config
          mountPath: /config
          readOnly: true
      volumes:
      - name: host-root
        hostPath:
          path: /
          type: Directory
      - name: config
        configMap:
          name: stargz-store-config
`, s.namespace, stargzStoreServiceAccount, s.namespace, s.namespace,
		stargzStoreDaemonSetName, s.namespace, stargzStoreServiceAccount,
		s.targetNodeLabel, stargzStoreVersion, stargzStoreVersion, stargzStoreVersion)

	return yaml
}

// grantPrivilegedSCC grants privileged SCC to the stargz-store serviceaccount
func (s *StargzStoreSetup) grantPrivilegedSCC(ctx context.Context) error {
	// Get the privileged SCC
	scc, err := s.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Get(ctx, "privileged", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get privileged SCC: %w", err)
	}

	// Check if serviceaccount already has access
	saName := fmt.Sprintf("system:serviceaccount:%s:%s", s.namespace, stargzStoreServiceAccount)
	for _, user := range scc.Users {
		if user == saName {
			framework.Logf("ServiceAccount %s already has privileged SCC", saName)
			return nil
		}
	}

	// Add serviceaccount to privileged SCC
	scc.Users = append(scc.Users, saName)
	_, err = s.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Update(ctx, scc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update privileged SCC: %w", err)
	}

	framework.Logf("Granted privileged SCC to %s", saName)
	return nil
}

// waitForDaemonSetReady waits for the DaemonSet to have all pods ready
func (s *StargzStoreSetup) waitForDaemonSetReady(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for stargz-store DaemonSet to be ready (timeout: %v)...", timeout)

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := s.oc.AdminKubeClient().AppsV1().DaemonSets(s.namespace).Get(ctx, stargzStoreDaemonSetName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to get DaemonSet: %v", err)
			return false, nil
		}

		desired := ds.Status.DesiredNumberScheduled
		ready := ds.Status.NumberReady
		available := ds.Status.NumberAvailable
		updated := ds.Status.UpdatedNumberScheduled

		framework.Logf("DaemonSet status: desired=%d, ready=%d, available=%d, updated=%d",
			desired, ready, available, updated)

		// Check if all pods are ready
		if desired > 0 && ready == desired && available == desired {
			framework.Logf("DaemonSet is ready: %d/%d pods ready", ready, desired)
			return true, nil
		}

		// Get pod details for debugging
		pods, err := s.oc.AdminKubeClient().CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=stargz-store-installer",
		})
		if err == nil && len(pods.Items) > 0 {
			framework.Logf("DEBUG: DaemonSet pods status:")
			for _, pod := range pods.Items {
				framework.Logf("  Pod %s on node %s: Phase=%s, Ready=%v",
					pod.Name, pod.Spec.NodeName, pod.Status.Phase, isPodReady(&pod))

				// Show container status for debugging
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil {
						framework.Logf("    Container %s: Waiting - %s: %s",
							cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
					} else if cs.State.Terminated != nil {
						framework.Logf("    Container %s: Terminated - %s (exit %d): %s",
							cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
					} else if cs.State.Running != nil {
						framework.Logf("    Container %s: Running, Ready=%v", cs.Name, cs.Ready)
					}
				}
			}
		}

		return false, nil
	})
}

// isPodReady returns true if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// GetStorePath returns the stargz-store path for ContainerRuntimeConfig
func (s *StargzStoreSetup) GetStorePath() string {
	return stargzStorePath
}

// IsDeployed returns true if stargz-store has been deployed
func (s *StargzStoreSetup) IsDeployed() bool {
	return s.deployed
}

// Cleanup removes stargz-store from the cluster
func (s *StargzStoreSetup) Cleanup(ctx context.Context) error {
	if !s.deployed {
		framework.Logf("stargz-store was not deployed, skipping cleanup")
		return nil
	}

	framework.Logf("Cleaning up stargz-store...")

	// Delete namespace (this cascades to all resources)
	cmd := exec.Command("oc", "delete", "namespace", s.namespace, "--ignore-not-found=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		framework.Logf("Warning: failed to delete namespace: %v\nOutput: %s", err, string(output))
	} else {
		framework.Logf("Deleted namespace %s", s.namespace)

		// Wait for namespace to be fully deleted
		framework.Logf("Waiting for namespace %s to be fully deleted...", s.namespace)
		wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := s.oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, s.namespace, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})
	}

	// Remove from privileged SCC
	saName := fmt.Sprintf("system:serviceaccount:%s:%s", s.namespace, stargzStoreServiceAccount)
	scc, err := s.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Get(ctx, "privileged", metav1.GetOptions{})
	if err == nil {
		var newUsers []string
		removed := false
		for _, user := range scc.Users {
			if user != saName {
				newUsers = append(newUsers, user)
			} else {
				removed = true
			}
		}
		if removed {
			scc.Users = newUsers
			_, err = s.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Update(ctx, scc, metav1.UpdateOptions{})
			if err != nil {
				framework.Logf("Warning: failed to remove SA from privileged SCC: %v", err)
			} else {
				framework.Logf("Removed %s from privileged SCC", saName)
			}
		}
	}

	// Stop and disable stargz-store service on nodes
	nodes, err := getNodesByLabel(ctx, s.oc, s.targetNodeLabel)
	if err == nil {
		for _, node := range nodes {
			framework.Logf("Stopping stargz-store on node %s", node.Name)
			_, err := ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "stop", "stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to stop stargz-store on %s: %v", node.Name, err)
			}

			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "disable", "stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to disable stargz-store on %s: %v", node.Name, err)
			}

			// Unmount any stale FUSE mounts
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "umount", "-l", "/var/lib/stargz-store/store")
			if err != nil {
				framework.Logf("Info: no stargz-store mount to unmount on %s (expected if already cleaned)", node.Name)
			} else {
				framework.Logf("Unmounted stargz-store FUSE mount on %s", node.Name)
			}

			// Clean up snapshot metadata (but not the binary/config - those persist)
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "rm", "-rf", "/var/lib/stargz-store/store/*")
			if err != nil {
				framework.Logf("Warning: failed to clean stargz-store snapshots on %s: %v", node.Name, err)
			}

			// Restart CRI-O to remove stargz-store from layer stores
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "restart", "crio")
			if err != nil {
				framework.Logf("Warning: failed to restart crio on %s: %v", node.Name, err)
			}
		}
	}

	s.deployed = false
	framework.Logf("stargz-store cleanup complete")
	return nil
}

// getStargzSnapshotCount counts the number of snapshots in stargz-store
func getStargzSnapshotCount(oc *exutil.CLI, nodeName string) (int, error) {
	// List contents of stargz-store to count snapshots/layers
	output, err := ExecOnNodeWithChroot(oc, nodeName, "find", "/var/lib/stargz-store/store", "-type", "d", "-mindepth", "1")
	if err != nil {
		return 0, fmt.Errorf("failed to count snapshots: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	count := 0
	for _, line := range lines {
		if line != "" {
			count++
		}
	}
	return count, nil
}

// deletePodAndWait deletes a pod and waits for it to be gone
func deletePodAndWait(ctx context.Context, oc *exutil.CLI, namespace, podName string) {
	err := oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete pod %s/%s: %v", namespace, podName, err)
		return
	}
	if apierrors.IsNotFound(err) {
		framework.Logf("Pod %s/%s already deleted", namespace, podName)
		return
	}
	framework.Logf("Deleting pod %s/%s", namespace, podName)

	// Wait for pod to be deleted
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		framework.Logf("Warning: timeout waiting for pod %s/%s deletion: %v", namespace, podName, err)
	} else {
		framework.Logf("Pod %s/%s deleted successfully", namespace, podName)
	}
}
