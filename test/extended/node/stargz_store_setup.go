// This file contains helper functions for stargz-store setup and management.
// It provides utilities for installing, configuring, and verifying stargz-snapshotter
// on OpenShift worker nodes to enable lazy pulling via additionalLayerStores.
package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	stargzStoreNamespace      = "stargz-store"
	stargzStoreDaemonSetName  = "stargz-store-installer"
	stargzStoreServiceAccount = "stargz-store"
	stargzStorePath           = "/var/lib/stargz-store/store" // MCO automatically adds :ref suffix
	// stargzStoreVersion is the stargz-snapshotter release version to install.
	// Update periodically to track upstream releases: https://github.com/containerd/stargz-snapshotter/releases
	// When updating, verify the release includes binaries for all supported architectures (amd64, arm64, s390x, ppc64le).
	// Last updated: 2024-01 (v0.18.2 supports all required architectures)
	stargzStoreVersion = "v0.18.2"
)

// StargzStoreSetup manages the lifecycle of stargz-store on cluster nodes
type StargzStoreSetup struct {
	oc        *exutil.CLI
	namespace string
	deployed  bool
}

// NewStargzStoreSetup creates a new StargzStoreSetup instance
func NewStargzStoreSetup(oc *exutil.CLI) *StargzStoreSetup {
	return &StargzStoreSetup{
		oc:        oc,
		namespace: stargzStoreNamespace,
		deployed:  false,
	}
}

// Deploy installs stargz-store on all worker nodes using a DaemonSet
func (s *StargzStoreSetup) Deploy(ctx context.Context) error {
	framework.Logf("Deploying stargz-store to cluster...")

	// Create namespace
	if err := s.createNamespace(ctx); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Create ServiceAccount
	if err := s.createServiceAccount(ctx); err != nil {
		return fmt.Errorf("failed to create serviceaccount: %w", err)
	}

	// Grant privileged SCC to ServiceAccount
	if err := s.grantPrivilegedSCC(ctx); err != nil {
		return fmt.Errorf("failed to grant privileged SCC: %w", err)
	}

	// Create ConfigMap with stargz-store config and systemd service
	if err := s.createConfigMap(ctx); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	// Create DaemonSet
	if err := s.createDaemonSet(ctx); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	// Mark as deployed immediately after cluster resources created
	// This ensures cleanup runs even if verification fails
	s.deployed = true

	// Wait for DaemonSet to be ready
	if err := s.waitForDaemonSetReady(ctx, 10*time.Minute); err != nil {
		return fmt.Errorf("failed waiting for daemonset: %w", err)
	}

	// Verify stargz-store is running on nodes
	if err := s.verifyStargzStoreRunning(ctx); err != nil {
		return fmt.Errorf("stargz-store verification failed: %w", err)
	}

	framework.Logf("stargz-store deployed successfully")
	return nil
}

// Cleanup removes stargz-store from all nodes
func (s *StargzStoreSetup) Cleanup(ctx context.Context) error {
	if !s.deployed {
		return nil
	}

	framework.Logf("Cleaning up stargz-store...")

	// First, uninstall stargz-store from all worker nodes
	framework.Logf("Uninstalling stargz-store from worker nodes...")
	nodes, err := s.oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker=",
	})
	if err != nil {
		framework.Logf("Warning: failed to list worker nodes: %v", err)
	} else {
		for _, node := range nodes.Items {
			framework.Logf("Uninstalling stargz-store from node: %s", node.Name)

			// Stop stargz-store service
			_, err := ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "stop", "stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to stop stargz-store on %s: %v", node.Name, err)
			}

			// Disable stargz-store service
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "disable", "stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to disable stargz-store on %s: %v", node.Name, err)
			}

			// Remove stargz-store binary
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "rm", "-f", "/usr/local/bin/stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to remove binary on %s: %v", node.Name, err)
			}

			// Remove systemd service file
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "rm", "-f", "/etc/systemd/system/stargz-store.service")
			if err != nil {
				framework.Logf("Warning: failed to remove service file on %s: %v", node.Name, err)
			}

			// Remove config directory
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "rm", "-rf", "/etc/stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to remove config directory on %s: %v", node.Name, err)
			}

			// Remove data directory
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "rm", "-rf", "/var/lib/stargz-store")
			if err != nil {
				framework.Logf("Warning: failed to remove data directory on %s: %v", node.Name, err)
			}

			// Reload systemd daemon
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "daemon-reload")
			if err != nil {
				framework.Logf("Warning: failed to reload systemd on %s: %v", node.Name, err)
			}

			// Restart CRI-O to remove stargz-store from layer stores
			_, err = ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "restart", "crio")
			if err != nil {
				framework.Logf("Warning: failed to restart crio on %s: %v", node.Name, err)
			}

			framework.Logf("Uninstalled stargz-store from node: %s", node.Name)
		}
	}

	// Delete namespace (cascades to all resources)
	err = s.oc.AdminKubeClient().CoreV1().Namespaces().Delete(ctx, s.namespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete namespace %s: %v", s.namespace, err)
	}

	// Wait for namespace to be deleted
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := s.oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, s.namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		framework.Logf("Warning: namespace deletion timed out: %v", err)
	}

	s.deployed = false
	framework.Logf("stargz-store cleanup completed")
	return nil
}

// GetStorePath returns the path to use in ContainerRuntimeConfig for stargz layer store
func (s *StargzStoreSetup) GetStorePath() string {
	return stargzStorePath
}

// IsDeployed returns true if stargz-store has been deployed
func (s *StargzStoreSetup) IsDeployed() bool {
	return s.deployed
}

func (s *StargzStoreSetup) createNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.namespace,
			Labels: map[string]string{
				"app":                                            "stargz-store",
				"pod-security.kubernetes.io/enforce":             "privileged",
				"pod-security.kubernetes.io/audit":               "privileged",
				"pod-security.kubernetes.io/warn":                "privileged",
				"security.openshift.io/scc.podSecurityLabelSync": "false",
			},
		},
	}

	_, err := s.oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	framework.Logf("Namespace %s created/exists", s.namespace)
	return nil
}

func (s *StargzStoreSetup) createServiceAccount(ctx context.Context) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stargzStoreServiceAccount,
			Namespace: s.namespace,
		},
	}

	_, err := s.oc.AdminKubeClient().CoreV1().ServiceAccounts(s.namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	framework.Logf("ServiceAccount %s created/exists", stargzStoreServiceAccount)
	return nil
}

func (s *StargzStoreSetup) grantPrivilegedSCC(ctx context.Context) error {
	// Get the privileged SCC
	scc, err := s.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Get(ctx, "privileged", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get privileged SCC: %w", err)
	}

	// Add ServiceAccount to SCC users list
	saUser := fmt.Sprintf("system:serviceaccount:%s:%s", s.namespace, stargzStoreServiceAccount)

	// Check if already added
	for _, user := range scc.Users {
		if user == saUser {
			framework.Logf("ServiceAccount %s already has privileged SCC", stargzStoreServiceAccount)
			return nil
		}
	}

	// Add to users list
	scc.Users = append(scc.Users, saUser)

	// Update SCC
	_, err = s.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Update(ctx, scc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update privileged SCC: %w", err)
	}

	framework.Logf("Granted privileged SCC to ServiceAccount %s", stargzStoreServiceAccount)
	return nil
}

func (s *StargzStoreSetup) createConfigMap(ctx context.Context) error {
	configToml := `# Stargz-store configuration for CRI-O
# Registry resolver config
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
`

	serviceFile := `[Unit]
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
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stargz-store-config",
			Namespace: s.namespace,
		},
		Data: map[string]string{
			"config.toml":          configToml,
			"stargz-store.service": serviceFile,
		},
	}

	_, err := s.oc.AdminKubeClient().CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	framework.Logf("ConfigMap stargz-store-config created/exists")
	return nil
}

func (s *StargzStoreSetup) createDaemonSet(ctx context.Context) error {
	installerScript := fmt.Sprintf(`set -e

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

# Detect node architecture
NODE_ARCH=$(uname -m)
echo "Detected architecture: $NODE_ARCH"

# Map architecture to release naming convention
# uname -m returns: x86_64, aarch64, s390x, ppc64le
case "$NODE_ARCH" in
  x86_64)
    DOWNLOAD_ARCH="amd64"
    ;;
  aarch64)
    DOWNLOAD_ARCH="arm64"
    ;;
  s390x)
    DOWNLOAD_ARCH="s390x"
    ;;
  ppc64le)
    DOWNLOAD_ARCH="ppc64le"
    ;;
  *)
    echo "ERROR: Unsupported architecture: $NODE_ARCH"
    exit 1
    ;;
esac

echo "Using release architecture: $DOWNLOAD_ARCH"

# Download stargz-snapshotter release
echo "Downloading stargz-snapshotter %s for linux-$DOWNLOAD_ARCH..."
DOWNLOAD_URL="https://github.com/containerd/stargz-snapshotter/releases/download/%s/stargz-snapshotter-%s-linux-$DOWNLOAD_ARCH.tar.gz"
echo "DEBUG: Download URL: $DOWNLOAD_URL"

if ! curl -L -f -o /tmp/stargz.tar.gz "$DOWNLOAD_URL"; then
  echo "ERROR: Failed to download stargz-snapshotter from $DOWNLOAD_URL"
  echo "DEBUG: Checking curl version and network:"
  curl --version
  curl -I https://github.com 2>&1 | head -5
  exit 1
fi

echo "DEBUG: Download successful, file size:"
ls -lh /tmp/stargz.tar.gz

# Extract to host
echo "Extracting binary to /usr/local/bin..."
if ! tar -xzf /tmp/stargz.tar.gz -C /tmp/; then
  echo "ERROR: Failed to extract tarball"
  echo "DEBUG: Tarball contents check:"
  tar -tzf /tmp/stargz.tar.gz | head -20
  exit 1
fi

echo "DEBUG: Extracted files in /tmp:"
ls -la /tmp/stargz* || echo "No stargz files found"

if [ ! -f /tmp/stargz-store ]; then
  echo "ERROR: stargz-store binary not found after extraction"
  echo "DEBUG: All files in /tmp:"
  ls -la /tmp/
  exit 1
fi

cp /tmp/stargz-store /host/usr/local/bin/
chmod +x /host/usr/local/bin/stargz-store

echo "DEBUG: Binary copied, checking on host:"
ls -la /host/usr/local/bin/stargz-store

# Verify binary
echo "Verifying binary..."
if nsenter -t 1 -m -u -i -n -p -- /usr/local/bin/stargz-store --version; then
  echo "DEBUG: Binary verification successful"
else
  echo "ERROR: Binary verification failed (exit code: $?)"
  echo "DEBUG: Trying to get more info about the binary:"
  nsenter -t 1 -m -u -i -n -p -- file /usr/local/bin/stargz-store
  nsenter -t 1 -m -u -i -n -p -- ldd /usr/local/bin/stargz-store 2>&1 | head -10
  exit 1
fi

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
echo "DEBUG: daemon-reload completed (exit code: $?)"

if ! nsenter -t 1 -m -u -i -n -p -- systemctl enable stargz-store; then
  echo "ERROR: Failed to enable stargz-store service (exit code: $?)"
  exit 1
fi
echo "DEBUG: Service enabled successfully"

echo "DEBUG: Starting stargz-store service..."
if ! nsenter -t 1 -m -u -i -n -p -- systemctl start stargz-store; then
  echo "ERROR: Failed to start stargz-store service (exit code: $?)"
  echo "DEBUG: Checking service status after failed start:"
  nsenter -t 1 -m -u -i -n -p -- systemctl status stargz-store --no-pager || true
  echo "DEBUG: Checking recent service logs:"
  nsenter -t 1 -m -u -i -n -p -- journalctl -u stargz-store -n 50 --no-pager || true
  exit 1
fi
echo "DEBUG: systemctl start command completed (exit code: $?)"

# Wait for service to be ready
echo "Waiting for stargz-store to be ready..."
sleep 5

# Verify service is running
echo "Verifying stargz-store service status..."
SERVICE_STATUS=$(nsenter -t 1 -m -u -i -n -p -- systemctl is-active stargz-store)
echo "DEBUG: Service is-active status: $SERVICE_STATUS"

if [ "$SERVICE_STATUS" != "active" ]; then
  echo "ERROR: Service is not active (status: $SERVICE_STATUS)"
  echo "DEBUG: Full service status:"
  nsenter -t 1 -m -u -i -n -p -- systemctl status stargz-store --no-pager || true
  echo "DEBUG: Recent logs:"
  nsenter -t 1 -m -u -i -n -p -- journalctl -u stargz-store -n 100 --no-pager || true
  exit 1
fi

# Verify FUSE mount
echo "Verifying FUSE mount..."
if nsenter -t 1 -m -u -i -n -p -- mount | grep stargz; then
  echo "DEBUG: FUSE mount found"
else
  echo "WARNING: stargz mount not found"
  echo "DEBUG: All mounts:"
  nsenter -t 1 -m -u -i -n -p -- mount | grep -E "fuse|stargz" || echo "No FUSE/stargz mounts"
  echo "DEBUG: Checking /dev/fuse:"
  nsenter -t 1 -m -u -i -n -p -- ls -la /dev/fuse || echo "/dev/fuse not found"
fi

echo "=== Setup complete! ==="
echo "NOTE: CRI-O will be restarted by MCO when ContainerRuntimeConfig is applied"
echo "stargz-store is now running on $(hostname)"
echo ""
echo "To use in ContainerRuntimeConfig, set additionalLayerStores path to:"
echo "  /var/lib/stargz-store/store:ref"
echo ""

# Keep pod running
sleep infinity
`, stargzStoreVersion, stargzStoreVersion, stargzStoreVersion)

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stargzStoreDaemonSetName,
			Namespace: s.namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "stargz-store-installer",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "stargz-store-installer",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: stargzStoreServiceAccount,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
					HostPID:     true,
					HostNetwork: true,
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "installer",
							Image: "registry.access.redhat.com/ubi9/ubi:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
							Command: []string{"/bin/bash", "-c", installerScript},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "host-root",
									MountPath: "/host",
								},
								{
									Name:      "config",
									MountPath: "/config",
									ReadOnly:  true,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resourceMustParse("64Mi"),
									corev1.ResourceCPU:    resourceMustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resourceMustParse("256Mi"),
									corev1.ResourceCPU:    resourceMustParse("500m"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "host-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
									Type: hostPathTypePtr(corev1.HostPathDirectory),
								},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "stargz-store-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := s.oc.AdminKubeClient().AppsV1().DaemonSets(s.namespace).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	framework.Logf("DaemonSet %s created/exists", stargzStoreDaemonSetName)
	return nil
}

func (s *StargzStoreSetup) waitForDaemonSetReady(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for stargz-store DaemonSet to be ready (timeout: %v)...", timeout)

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := s.oc.AdminKubeClient().AppsV1().DaemonSets(s.namespace).Get(ctx, stargzStoreDaemonSetName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("DEBUG: Failed to get DaemonSet: %v", err)
			return false, nil
		}

		framework.Logf("DaemonSet status: desired=%d, ready=%d, available=%d, updated=%d",
			ds.Status.DesiredNumberScheduled, ds.Status.NumberReady, ds.Status.NumberAvailable, ds.Status.UpdatedNumberScheduled)

		if ds.Status.DesiredNumberScheduled == 0 {
			framework.Logf("DEBUG: No pods scheduled yet")
			return false, nil
		}

		// DEBUG: Get pod status if not all ready
		if ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
			pods, err := s.oc.AdminKubeClient().CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=stargz-store-installer",
			})
			if err == nil && len(pods.Items) > 0 {
				framework.Logf("DEBUG: DaemonSet pods status:")
				for _, pod := range pods.Items {
					framework.Logf("  Pod %s on node %s: Phase=%s, Ready=%v",
						pod.Name, pod.Spec.NodeName, pod.Status.Phase,
						isPodReady(&pod))

					// Log container status
					for _, cs := range pod.Status.ContainerStatuses {
						if cs.State.Waiting != nil {
							framework.Logf("    Container %s: Waiting - %s: %s",
								cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
						} else if cs.State.Terminated != nil {
							framework.Logf("    Container %s: Terminated - %s (exit %d): %s",
								cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode,
								cs.State.Terminated.Message)
						} else if cs.State.Running != nil {
							framework.Logf("    Container %s: Running since %v",
								cs.Name, cs.State.Running.StartedAt)
						}
					}
				}
			}
		}

		if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
			ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled {
			framework.Logf("DEBUG: All DaemonSet pods are ready")
			return true, nil
		}

		return false, nil
	})
}

// Helper function to check if pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (s *StargzStoreSetup) verifyStargzStoreRunning(ctx context.Context) error {
	framework.Logf("Verifying stargz-store is running on worker nodes...")

	workerNodes, err := getNodesByLabel(ctx, s.oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return fmt.Errorf("failed to get worker nodes: %w", err)
	}

	pureWorkers := getPureWorkerNodes(workerNodes)
	// Use pureWorkers if available, otherwise use any worker node (SNO support)
	if len(pureWorkers) == 0 {
		pureWorkers = workerNodes
	}

	for _, node := range pureWorkers {
		framework.Logf("=== DEBUG: Verifying stargz-store on node %s ===", node.Name)

		// DEBUG 1: Check if binary exists
		framework.Logf("DEBUG: Checking if stargz-store binary exists...")
		binaryCheck, _ := ExecOnNodeWithChroot(s.oc, node.Name, "ls", "-la", "/usr/local/bin/stargz-store")
		framework.Logf("DEBUG: Binary check output: %s", binaryCheck)

		// DEBUG 2: Try to run binary --version
		framework.Logf("DEBUG: Checking stargz-store version...")
		versionOutput, versionErr := ExecOnNodeWithChroot(s.oc, node.Name, "/usr/local/bin/stargz-store", "--version")
		if versionErr != nil {
			framework.Logf("DEBUG: Version check failed: %v", versionErr)
		} else {
			framework.Logf("DEBUG: stargz-store version: %s", versionOutput)
		}

		// DEBUG 3: Check systemd service file exists
		framework.Logf("DEBUG: Checking if systemd service file exists...")
		serviceFileCheck, _ := ExecOnNodeWithChroot(s.oc, node.Name, "ls", "-la", "/etc/systemd/system/stargz-store.service")
		framework.Logf("DEBUG: Service file check: %s", serviceFileCheck)

		// DEBUG 4: Check systemd service status (detailed)
		framework.Logf("DEBUG: Checking systemd service status...")
		statusOutput, _ := ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "status", "stargz-store", "--no-pager")
		framework.Logf("DEBUG: Service status output:\n%s", statusOutput)

		// DEBUG 5: Check service is-active
		output, err := ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "is-active", "stargz-store")
		framework.Logf("DEBUG: Service is-active output: %s", strings.TrimSpace(output))
		if err != nil {
			framework.Logf("DEBUG: is-active command error: %v", err)
		}

		// DEBUG 6: Get recent service logs
		framework.Logf("DEBUG: Fetching recent stargz-store service logs...")
		logsOutput, _ := ExecOnNodeWithChroot(s.oc, node.Name, "journalctl", "-u", "stargz-store", "-n", "50", "--no-pager")
		framework.Logf("DEBUG: Service logs (last 50 lines):\n%s", logsOutput)

		// DEBUG 7: Check data directory
		framework.Logf("DEBUG: Checking stargz-store data directory...")
		dataDirCheck, _ := ExecOnNodeWithChroot(s.oc, node.Name, "ls", "-la", "/var/lib/stargz-store/")
		framework.Logf("DEBUG: Data directory contents:\n%s", dataDirCheck)

		// DEBUG 8: Check config directory
		framework.Logf("DEBUG: Checking stargz-store config directory...")
		configDirCheck, _ := ExecOnNodeWithChroot(s.oc, node.Name, "ls", "-la", "/etc/stargz-store/")
		framework.Logf("DEBUG: Config directory contents:\n%s", configDirCheck)

		// Now do the actual verification
		if strings.TrimSpace(output) != "active" {
			framework.Logf("ERROR: stargz-store is not active on node %s (status: %s)", node.Name, strings.TrimSpace(output))
			return fmt.Errorf("stargz-store is not active on node %s (status: %s)", node.Name, strings.TrimSpace(output))
		}

		// Check FUSE mount
		framework.Logf("DEBUG: Checking FUSE mount...")
		mountOutput, err := ExecOnNodeWithChroot(s.oc, node.Name, "mount")
		if err != nil {
			framework.Logf("DEBUG: mount command failed: %v", err)
			return fmt.Errorf("failed to check mounts on node %s: %w", node.Name, err)
		}

		framework.Logf("DEBUG: Mount output (grep stargz):")
		for _, line := range strings.Split(mountOutput, "\n") {
			if strings.Contains(line, "stargz") {
				framework.Logf("  %s", line)
			}
		}

		if !strings.Contains(mountOutput, "stargz") {
			framework.Logf("ERROR: stargz FUSE mount not found on node %s", node.Name)
			return fmt.Errorf("stargz FUSE mount not found on node %s", node.Name)
		}

		framework.Logf("Node %s: stargz-store is active and mounted", node.Name)
	}

	return nil
}

// VerifyStorageConfContainsStargz checks if storage.conf contains stargz-store path
func (s *StargzStoreSetup) VerifyStorageConfContainsStargz(ctx context.Context) error {
	workerNodes, err := getNodesByLabel(ctx, s.oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return fmt.Errorf("failed to get worker nodes: %w", err)
	}

	pureWorkers := getPureWorkerNodes(workerNodes)
	// Use pureWorkers if available, otherwise use any worker node (SNO support)
	if len(pureWorkers) == 0 {
		pureWorkers = workerNodes
	}
	for _, node := range pureWorkers {
		output, err := ExecOnNodeWithChroot(s.oc, node.Name, "cat", "/etc/containers/storage.conf")
		if err != nil {
			return fmt.Errorf("failed to read storage.conf on node %s: %w", node.Name, err)
		}

		if !strings.Contains(output, "/var/lib/stargz-store/store") {
			return fmt.Errorf("storage.conf on node %s does not contain stargz-store path", node.Name)
		}

		framework.Logf("Node %s: storage.conf contains stargz-store path", node.Name)
	}

	return nil
}

// Helper functions
func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}

func resourceMustParse(s string) resource.Quantity {
	q := resource.MustParse(s)
	return q
}

// getStargzSnapshotCount returns the number of snapshots in stargz-store
func getStargzSnapshotCount(oc *exutil.CLI, nodeName string) int {
	// List contents of stargz-store to count snapshots/layers
	output, err := ExecOnNodeWithChroot(oc, nodeName, "find", "/var/lib/stargz-store/store", "-type", "d", "-mindepth", "1")
	if err != nil {
		framework.Logf("Warning: failed to count snapshots: %v", err)
		return 0
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	count := 0
	for _, line := range lines {
		if line != "" {
			count++
		}
	}
	return count
}

// createTestPodSpec creates a simple pod spec for testing
func createTestPodSpec(name, namespace, image, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: boolPtr(true),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   image,
					Command: []string{"/bin/sh", "-c", "sleep 3600"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						RunAsNonRoot: boolPtr(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

// deletePodAndWait deletes a pod and waits for it to be gone
func deletePodAndWait(ctx context.Context, oc *exutil.CLI, namespace, podName string) {
	err := oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete pod %s: %v", podName, err)
		return
	}

	// Wait for pod to be deleted
	wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}
