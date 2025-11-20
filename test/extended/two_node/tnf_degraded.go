package two_node

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	pdbLabelKey       = "app"
	pdbLabelValue     = "pdb-demo"
	pdbDeploymentName = "pdb-demo-deployment"
	pdbName           = "pdb-demo"
	rebootTestMCName  = "99-master-tnf-degraded-reboot-block-test"
	rebootTestMCFile  = "/etc/tnf-degraded-reboot-block-test"
)

var _ = g.Describe("[sig-apps][OCPFeatureGate:DualReplica][Suite:openshift/two-node] Two Node Fencing behavior in degraded mode", func() {
	oc := exutil.NewCLI("tnf-degraded").AsAdmin()
	ctx := context.Background()
	kubeClient := oc.AdminKubeClient()

	g.BeforeEach(func() {
		ensureTNFDegradedOrSkip(oc)
	})

	g.It("PDB minAvailable=1 should allow a single eviction and block the second in TNF degraded mode [apigroup:policy]", func() {
		ns := oc.Namespace()
		labels := map[string]string{pdbLabelKey: pdbLabelValue}
		selector := fmt.Sprintf("%s=%s", pdbLabelKey, pdbLabelValue)

		// Deployment with 2 pause pods
		deploy, err := createPauseDeployment(ctx, kubeClient, ns, pdbDeploymentName, 2, labels)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForDeploymentAvailable(ctx, kubeClient, ns, deploy.Name, 2, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "deployment did not reach 2 available replicas")

		// PDB minAvailable=1
		pdb, err := createPDBMinAvailable(ctx, kubeClient, ns, pdbName, labels, 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for disruptionsAllowed=1
		err = waitForPDBDisruptionsAllowed(ctx, kubeClient, ns, pdb.Name, 1, 2*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "PDB did not report disruptionsAllowed=1")

		pods, err := kubeClient.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(2), "expected exactly 2 pods before first eviction")

		firstPod := &pods.Items[0]
		secondPod := &pods.Items[1]

		// Evict first pod should succeed  and wait for PDB to consume
		err = evictPod(ctx, kubeClient, firstPod)
		o.Expect(err).NotTo(o.HaveOccurred(), "first eviction should succeed")

		err = waitForPDBDisruptionsAllowed(ctx, kubeClient, ns, pdb.Name, 0, 2*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "PDB did not update disruptionsAllowed=0 after first eviction")

		// Evict second original pod, should be blocked with 429
		err = evictPod(ctx, kubeClient, secondPod)
		o.Expect(err).To(o.HaveOccurred(), "second eviction should be blocked by PDB")

		statusErr, ok := err.(*apierrs.StatusError)
		o.Expect(ok).To(o.BeTrue(), "expected StatusError on blocked eviction")
		o.Expect(statusErr.Status().Code).To(o.Equal(int32(429)), "expected HTTP 429 Too Many Requests for second eviction")

		// PDB disruptionsAllowed must be 0
		currentPDB, err := kubeClient.PolicyV1().PodDisruptionBudgets(ns).Get(ctx, pdb.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(currentPDB.Status.DisruptionsAllowed).To(o.Equal(int32(0)), "expected disruptionsAllowed=0 after second eviction attempt")
	})
	g.It("should block a reboot-required MachineConfig rollout on the remaining master in TNF degraded mode [Serial] [apigroup:machineconfiguration.openshift.io]", func() {
		ns := oc.Namespace()
		mcoClient := machineconfigclient.NewForConfigOrDie(oc.AdminConfig())

		masterNode, err := getReadyMasterNode(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find a Ready master node in TNF degraded mode")

		originalBootID := masterNode.Status.NodeInfo.BootID
		originalUnschedulable := masterNode.Spec.Unschedulable

		// Capture current master MachineConfigPool state so we can assert it never progresses
		masterMCP, err := mcoClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get master MachineConfigPool")

		originalConfigName := masterMCP.Status.Configuration.Name

		// Create a small reboot-required MachineConfig targeting master
		ignFileContents := fmt.Sprintf("TNF degraded reboot-block test namespace=%s", ns)

		testMC := newMasterRebootRequiredMachineConfig(rebootTestMCName, rebootTestMCFile, ignFileContents)

		g.By(fmt.Sprintf("creating reboot-required MachineConfig %q for master pool", rebootTestMCName))
		_, err = mcoClient.MachineconfigurationV1().MachineConfigs().Create(ctx, testMC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create test MachineConfig")

		// Cleanup
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			_ = mcoClient.MachineconfigurationV1().MachineConfigs().Delete(
				cleanupCtx,
				rebootTestMCName,
				metav1.DeleteOptions{},
			)
		}()

		g.By("observing degraded TNF behavior (node safety + MCP blockage)")

		observationWindow := 3 * time.Minute

		err = observeTNFDegradedWindow(
			ctx,
			kubeClient,
			mcoClient,
			masterNode.Name,
			originalBootID,
			originalUnschedulable,
			originalConfigName,
			observationWindow,
		)

		o.Expect(err).NotTo(o.HaveOccurred(), "TNF degraded behavior was not enforced correctly")
	})
},
)

// HELPERS
func createPauseDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	ns, name string,
	replicas int32,
	labels map[string]string,
) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "pause",
							Image: "registry.k8s.io/pause:3.9",
						},
					},
				},
			},
		},
	}

	return client.AppsV1().Deployments(ns).Create(ctx, deploy, metav1.CreateOptions{})
}

func createPDBMinAvailable(
	ctx context.Context,
	client kubernetes.Interface,
	ns, name string,
	labels map[string]string,
	minAvailable int,
) (*policyv1.PodDisruptionBudget, error) {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: intOrStringPtr(intstr.FromInt(minAvailable)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
	return client.PolicyV1().PodDisruptionBudgets(ns).Create(ctx, pdb, metav1.CreateOptions{})
}

func waitForDeploymentAvailable(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
	desiredAvailable int32,
	timeout time.Duration,
) error {
	interval := 2 * time.Second

	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		dep, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return dep.Status.AvailableReplicas >= desiredAvailable, nil
	})
}

func waitForPDBDisruptionsAllowed(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
	expected int32,
	timeout time.Duration,
) error {
	interval := 2 * time.Second

	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		pdb, err := client.PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if pdb.Generation != pdb.Status.ObservedGeneration {
			return false, nil
		}
		return pdb.Status.DisruptionsAllowed == expected, nil
	})
}

func evictPod(
	ctx context.Context,
	client kubernetes.Interface,
	pod *corev1.Pod,
) error {
	eviction := &policyv1.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1",
			Kind:       "Eviction",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}
	return client.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction)
}

func intOrStringPtr(v intstr.IntOrString) *intstr.IntOrString {
	return &v
}

func newMasterRebootRequiredMachineConfig(name, path, contents string) *machineconfigv1.MachineConfig {
	encoded := base64.StdEncoding.EncodeToString([]byte(contents))

	ignJSON := fmt.Sprintf(`{
  "ignition": { "version": "3.2.0" },
  "storage": {
    "files": [{
      "path": "%s",
      "mode": 420,
      "overwrite": true,
      "contents": {
        "source": "data:text/plain;base64,%s"
      }
    }]
  }
}`, path, encoded)

	return &machineconfigv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/role": "master",
			},
		},
		Spec: machineconfigv1.MachineConfigSpec{
			Config: k8sruntime.RawExtension{
				Raw: []byte(ignJSON),
			},
		},
	}
}

// We don't use PollUntilContextTimeout here because it treats timeout as an error.
// we implement our own loop where only real reboot/drain/API/MCP errors fail the test.
func observeTNFDegradedWindow(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	mcoClient machineconfigclient.Interface,
	nodeName, originalBootID string,
	originalUnschedulable bool,
	originalConfigName string,
	duration time.Duration,
) error {
	interval := 10 * time.Second
	deadline := time.Now().Add(duration)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during TNF degraded observation: %w", ctx.Err())
		default:
		}

		if time.Now().After(deadline) {
			return nil // SUCCESS: node safe + MCP blocked
		}

		// NODE SAFETY CHECKS
		node, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node %q during observation: %w", nodeName, err)
		}

		if node.Status.NodeInfo.BootID != originalBootID {
			return fmt.Errorf("node %q reboot detected (BootID changed)", nodeName)
		}

		if node.Spec.Unschedulable && !originalUnschedulable {
			return fmt.Errorf("node %q became unschedulable (drain detected)", nodeName)
		}

		// MCP BLOCKAGE CHECK
		mcp, err := mcoClient.MachineconfigurationV1().
			MachineConfigPools().
			Get(context.Background(), "master", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get master MCP during observation: %w", err)
		}

		cfg := mcp.Status.Configuration.Name
		if cfg != "" && cfg != originalConfigName {
			return fmt.Errorf("master MCP progressed to configuration %q (expected %q while degraded)", cfg, originalConfigName)
		}
		time.Sleep(interval)
	}
}
