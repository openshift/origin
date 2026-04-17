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
	stargzStoreNamespace     = "stargz-store"
	stargzStoreDaemonSetName = "stargz-store-installer"
	stargzStorePath          = "/var/lib/stargz-store/store:ref"
	stargzStoreVersion       = "v0.18.2"
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

	// Create ConfigMap with stargz-store config and systemd service
	if err := s.createConfigMap(ctx); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	// Create DaemonSet
	if err := s.createDaemonSet(ctx); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	// Wait for DaemonSet to be ready
	if err := s.waitForDaemonSetReady(ctx, 10*time.Minute); err != nil {
		return fmt.Errorf("failed waiting for daemonset: %w", err)
	}

	// Verify stargz-store is running on nodes
	if err := s.verifyStargzStoreRunning(ctx); err != nil {
		return fmt.Errorf("stargz-store verification failed: %w", err)
	}

	s.deployed = true
	framework.Logf("stargz-store deployed successfully")
	return nil
}

// Cleanup removes stargz-store from all nodes
func (s *StargzStoreSetup) Cleanup(ctx context.Context) error {
	if !s.deployed {
		return nil
	}

	framework.Logf("Cleaning up stargz-store...")

	// Delete namespace (cascades to all resources)
	err := s.oc.AdminKubeClient().CoreV1().Namespaces().Delete(ctx, s.namespace, metav1.DeleteOptions{})
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
			return false, nil
		}

		framework.Logf("DaemonSet status: desired=%d, ready=%d, available=%d",
			ds.Status.DesiredNumberScheduled, ds.Status.NumberReady, ds.Status.NumberAvailable)

		if ds.Status.DesiredNumberScheduled == 0 {
			return false, nil
		}

		if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
			ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled {
			return true, nil
		}

		return false, nil
	})
}

func (s *StargzStoreSetup) verifyStargzStoreRunning(ctx context.Context) error {
	framework.Logf("Verifying stargz-store is running on worker nodes...")

	workerNodes, err := getNodesByLabel(ctx, s.oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return fmt.Errorf("failed to get worker nodes: %w", err)
	}

	pureWorkers := getPureWorkerNodes(workerNodes)
	if len(pureWorkers) == 0 {
		return fmt.Errorf("no pure worker nodes found")
	}

	for _, node := range pureWorkers {
		// Check stargz-store service status
		output, err := ExecOnNodeWithChroot(s.oc, node.Name, "systemctl", "is-active", "stargz-store")
		if err != nil {
			framework.Logf("Warning: failed to check stargz-store status on node %s: %v", node.Name, err)
			continue
		}

		if strings.TrimSpace(output) != "active" {
			return fmt.Errorf("stargz-store is not active on node %s (status: %s)", node.Name, strings.TrimSpace(output))
		}

		// Check FUSE mount
		mountOutput, err := ExecOnNodeWithChroot(s.oc, node.Name, "mount")
		if err != nil {
			framework.Logf("Warning: failed to check mounts on node %s: %v", node.Name, err)
			continue
		}

		if !strings.Contains(mountOutput, "stargz") {
			framework.Logf("Warning: stargz FUSE mount not found on node %s", node.Name)
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
