package storage

import (
	"context"
	"fmt"
	"slices"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/ghodss/yaml"
	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

const (
	// controllerManagerConfigFeatureGate gates the ControllerManager config API and
	// the KCM operator logic that consumes volumeForceDetach.
	controllerManagerConfigFeatureGate = "ControllerManagerConfig"
	// kcmOperandNamespace holds the rendered kube-controller-manager static pod config.
	kcmOperandNamespace = "openshift-kube-controller-manager"
	// kcmClusterOperator is the ClusterOperator that reports KCM rollout progress.
	kcmClusterOperator = "kube-controller-manager"
	// controllerManagerSingletonName is the required name of the ControllerManager CR.
	controllerManagerSingletonName = "cluster"

	// kcmRolloutTimeout is a per-rollout safety cap for a single KCM rollout
	// triggered by changing volumeForceDetach; it only fires if a rollout stalls
	// (normal rollouts take ~8-10m, so the cap leaves a safe margin).
	kcmRolloutTimeout = 20 * time.Minute
	// kubeletDownDuration is how long kubelet stays stopped before the node
	// self-recovers. It must comfortably exceed the time to detect NotReady
	// (nodeNotReadyTimeout) plus the longest observation window
	// (forceDetachObserveTimeout) so a kubelet restart never confounds the
	// force-detach assertions.
	kubeletDownDuration = 15 * time.Minute
	// kubeletStopDelay is a head start before the detached unit actually stops
	// kubelet, so the `oc debug` invocation can detach and remove its own debug pod
	// (which needs a live kubelet) before kubelet goes down. Mirrors the sleep used
	// by TriggerNodeRebootUngraceful in test/extended/util/nodes.go.
	kubeletStopDelay = 30 * time.Second
	// nodeNotReadyTimeout bounds how long we wait for the node to be seen NotReady
	// after kubelet is stopped.
	nodeNotReadyTimeout = 3 * time.Minute
	// forceDetachObserveTimeout bounds how long we wait for a force detach.
	// The attach/detach controller force detaches ~6 minutes after the pod is gone.
	forceDetachObserveTimeout = 8 * time.Minute
	// noForceDetachObserveDuration must exceed the 6-minute force-detach timeout to
	// prove a volume is NOT force detached.
	noForceDetachObserveDuration = 7 * time.Minute
	// kcmConfigConvergeTimeout bounds how long we wait for the rendered KCM operand
	// config to reflect an expected disable-force-detach-on-timeout value.
	kcmConfigConvergeTimeout = 10 * time.Minute
	// podRunningTimeout bounds how long we wait for the test pod to reach Running
	// (scheduling, dynamic PV provisioning, attach, mount, and image pull).
	podRunningTimeout = 5 * time.Minute
	// volumeAttachTimeout bounds how long we wait for the test volume to attach to
	// the node after the pod is running.
	volumeAttachTimeout = 2 * time.Minute
	// nodeRecoveryMargin is extra time beyond kubeletDownDuration to wait for the
	// node to become Ready again after kubelet auto-restarts.
	nodeRecoveryMargin = 5 * time.Minute
)

// forceDetachFlagPath is the location of the kube-controller-manager
// --disable-force-detach-on-timeout flag in the rendered operand config. It
// mirrors library-go's observer (ObserveVolumeForceDetach).
var forceDetachFlagPath = []string{"extendedArguments", "disable-force-detach-on-timeout"}

// isControllerManagerConfigEnabled reports whether the ControllerManagerConfig
// feature gate is enabled on the cluster.
func isControllerManagerConfigEnabled(ctx context.Context, oc *exutil.CLI) (bool, string) {
	fgs, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get FeatureGate resource")
	for _, fg := range fgs.Status.FeatureGates {
		for _, enabled := range fg.Enabled {
			if string(enabled.Name) == controllerManagerConfigFeatureGate {
				return true, ""
			}
		}
	}
	return false, fmt.Sprintf("%s feature gate is not enabled", controllerManagerConfigFeatureGate)
}

// getKCMForceDetachFlag reads the disable-force-detach-on-timeout flag value from
// the rendered kube-controller-manager operand config. found is false when the
// flag is absent entirely.
func getKCMForceDetachFlag(ctx context.Context, oc *exutil.CLI) (value []string, found bool, err error) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(kcmOperandNamespace).Get(ctx, "config", metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	var observed map[string]interface{}
	if err := yaml.Unmarshal([]byte(cm.Data["config.yaml"]), &observed); err != nil {
		return nil, false, err
	}
	return unstructured.NestedStringSlice(observed, forceDetachFlagPath...)
}

// expectKCMForceDetachFlag waits until the rendered KCM config matches the
// expectation. Pass expected == nil to assert the flag is absent.
func expectKCMForceDetachFlag(ctx context.Context, oc *exutil.CLI, expected []string) {
	o.Eventually(func() error {
		value, found, err := getKCMForceDetachFlag(ctx, oc)
		if err != nil {
			return err
		}
		if expected == nil {
			if found {
				return fmt.Errorf("expected disable-force-detach-on-timeout to be absent, got %v", value)
			}
			return nil
		}
		if !found {
			return fmt.Errorf("expected disable-force-detach-on-timeout=%v, but it is absent", expected)
		}
		if !slices.Equal(value, expected) {
			return fmt.Errorf("expected disable-force-detach-on-timeout=%v, got %v", expected, value)
		}
		return nil
	}, kcmConfigConvergeTimeout, 10*time.Second).Should(o.Succeed(),
		"kube-controller-manager config did not converge on the expected force-detach flag")
}

// expectedForceDetachFlag maps a volumeForceDetach policy to the KCM flag value it
// produces. An empty policy (no opinion) produces no flag at all (nil).
func expectedForceDetachFlag(policy configv1alpha1.VolumeForceDetachPolicy) []string {
	switch policy {
	case configv1alpha1.VolumeForceDetachOnOutOfServiceTaintOnly:
		return []string{"true"}
	case configv1alpha1.VolumeForceDetachOnUnmountTimeout:
		return []string{"false"}
	default:
		return nil
	}
}

// withKCMRollout applies mutate and, when it changes the rendered KCM force-detach
// flag, waits for the resulting kube-controller-manager rollout to complete before
// asserting the flag converged on desired. The rollout watch is established before
// the mutation so a fast rollout is not missed. When mutate does not change the
// flag, no rollout is expected and we only assert the flag.
func withKCMRollout(ctx context.Context, oc *exutil.CLI, desired []string, mutate func()) {
	current, _, err := getKCMForceDetachFlag(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read current KCM force-detach flag")

	if slices.Equal(current, desired) {
		mutate()
		expectKCMForceDetachFlag(ctx, oc, desired)
		return
	}

	rolloutCtx, cancel := context.WithTimeout(ctx, kcmRolloutTimeout)
	defer cancel()
	status := exutil.WaitForOperatorToRollout(rolloutCtx, oc.AdminConfigClient(), kcmClusterOperator)
	<-status.StableBeforeStarting()
	mutate()
	<-status.Done()
	o.Expect(status.Err()).NotTo(o.HaveOccurred(), "kube-controller-manager rollout did not complete")
	expectKCMForceDetachFlag(ctx, oc, desired)
}

// setVolumeForceDetach creates or updates the ControllerManager singleton with the
// given policy. An empty policy expresses "no opinion" (field unset).
func setVolumeForceDetach(ctx context.Context, oc *exutil.CLI, policy configv1alpha1.VolumeForceDetachPolicy) {
	client := oc.AdminConfigClient().ConfigV1alpha1().ControllerManagers()
	// Retry on Conflict (lost update) and AlreadyExists (lost create race).
	retriable := func(err error) bool { return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) }
	err := retry.OnError(retry.DefaultRetry, retriable, func() error {
		cm, err := client.Get(ctx, controllerManagerSingletonName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = client.Create(ctx, &configv1alpha1.ControllerManager{
				ObjectMeta: metav1.ObjectMeta{Name: controllerManagerSingletonName},
				Spec:       &configv1alpha1.ControllerManagerSpec{VolumeForceDetach: policy},
			}, metav1.CreateOptions{})
			return err
		}
		if err != nil {
			return err
		}
		if cm.Spec == nil {
			cm.Spec = &configv1alpha1.ControllerManagerSpec{}
		}
		cm.Spec.VolumeForceDetach = policy
		_, err = client.Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to set volumeForceDetach=%q", policy)
}

// originalDesiredFlag returns the KCM flag value expected after restoring the
// captured ControllerManager state.
func originalDesiredFlag(original *configv1alpha1.ControllerManager, existed bool) []string {
	if !existed || original.Spec == nil {
		return nil
	}
	return expectedForceDetachFlag(original.Spec.VolumeForceDetach)
}

// captureControllerManager returns a deep copy of the current ControllerManager
// singleton, or (nil, false) when it does not exist.
func captureControllerManager(ctx context.Context, oc *exutil.CLI) (*configv1alpha1.ControllerManager, bool) {
	cm, err := oc.AdminConfigClient().ConfigV1alpha1().ControllerManagers().Get(ctx, controllerManagerSingletonName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ControllerManager")
	return cm.DeepCopy(), true
}

// restoreControllerManager restores the singleton to the captured original state.
// The cluster ControllerManager is a protected required singleton that cannot be
// deleted, so when it did not originally exist we restore the "no opinion" state
// (empty volumeForceDetach), which is behaviorally identical to absent.
func restoreControllerManager(ctx context.Context, oc *exutil.CLI, original *configv1alpha1.ControllerManager, existed bool) {
	if !existed {
		setVolumeForceDetach(ctx, oc, "")
		return
	}
	client := oc.AdminConfigClient().ConfigV1alpha1().ControllerManagers()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm, err := client.Get(ctx, controllerManagerSingletonName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		cm.Spec = original.Spec.DeepCopy()
		_, err = client.Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore ControllerManager during cleanup")
}

// restoreControllerManagerAndSettle restores the captured original state and waits
// for any resulting KCM rollout to finish, so one test does not bleed into the next.
// Shared by both Describe blocks' AfterEach.
func restoreControllerManagerAndSettle(ctx context.Context, oc *exutil.CLI, original *configv1alpha1.ControllerManager, existed bool) {
	withKCMRollout(ctx, oc, originalDesiredFlag(original, existed), func() {
		restoreControllerManager(ctx, oc, original, existed)
	})
}

// [Timeout:45m] overrides the default 15m per-test timeout. A mutating scenario
// does two KCM rollouts within the per-test budget (the It applies the policy,
// then AfterEach restores it), each bounded by kcmRolloutTimeout.
var _ = g.Describe("[sig-storage][OCPFeatureGate:ControllerManagerConfig][Serial][apigroup:config.openshift.io][Timeout:45m] ControllerManager volumeForceDetach config", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLIWithoutNamespace("volume-force-detach-config")
		original       *configv1alpha1.ControllerManager
		originalExists bool
		captured       bool
	)

	g.BeforeEach(func(ctx context.Context) {
		// Reset before the skip checks: AfterEach still runs after a BeforeEach skip,
		// and must not touch the ControllerManager CR if capture never happened.
		captured = false
		if enabled, reason := isControllerManagerConfigEnabled(ctx, oc); !enabled {
			g.Skip(reason)
		}
		original, originalExists = captureControllerManager(ctx, oc)
		captured = true
	})

	g.AfterEach(func(ctx context.Context) {
		if !captured {
			return
		}
		restoreControllerManagerAndSettle(ctx, oc, original, originalExists)
	})

	g.It("should leave force detach on timeout enabled by default when volumeForceDetach is unset", func(ctx context.Context) {
		// No opinion: the operator emits nothing and the flag stays absent.
		withKCMRollout(ctx, oc, nil, func() {
			setVolumeForceDetach(ctx, oc, "")
		})
	})

	g.It("should disable force detach on timeout when volumeForceDetach is OnOutOfServiceTaintOnly", func(ctx context.Context) {
		policy := configv1alpha1.VolumeForceDetachOnOutOfServiceTaintOnly
		withKCMRollout(ctx, oc, expectedForceDetachFlag(policy), func() {
			setVolumeForceDetach(ctx, oc, policy)
		})
	})

	g.It("should enable force detach on timeout when volumeForceDetach is OnUnmountTimeout", func(ctx context.Context) {
		policy := configv1alpha1.VolumeForceDetachOnUnmountTimeout
		withKCMRollout(ctx, oc, expectedForceDetachFlag(policy), func() {
			setVolumeForceDetach(ctx, oc, policy)
		})
	})
})

// pickSchedulableWorker returns a ready, schedulable worker node (never a
// control-plane node), or ok=false when none exists. Stopping kubelet on a
// control-plane or single-node cluster would break the cluster, so such nodes are
// excluded and the caller should skip.
func pickSchedulableWorker(ctx context.Context, oc *exutil.CLI) (v1.Node, bool) {
	nodes, err := e2enode.GetReadySchedulableNodes(ctx, oc.AdminKubeClient())
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list ready schedulable nodes")
	for _, node := range nodes.Items {
		if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
			continue
		}
		if _, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]; isControlPlane {
			continue
		}
		return node, true
	}
	return v1.Node{}, false
}

// volumeAttachmentExistsForPV reports whether an attached VolumeAttachment for the
// given node and PV currently exists.
func volumeAttachmentExistsForPV(ctx context.Context, oc *exutil.CLI, nodeName, pvName string) (bool, error) {
	vaList, err := oc.AdminKubeClient().StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, va := range vaList.Items {
		if va.Spec.NodeName != nodeName {
			continue
		}
		if va.Spec.Source.PersistentVolumeName != nil && *va.Spec.Source.PersistentVolumeName == pvName && va.Status.Attached {
			return true, nil
		}
	}
	return false, nil
}

// createVolumePodOnNode creates a long-running pod on the given node (by hostname)
// that mounts the PVC, and waits for it to be Running. It is platform-agnostic: it
// relies on the PVC's (default) StorageClass and a generic image, so it works on
// any cloud provider.
func createVolumePodOnNode(ctx context.Context, oc *exutil.CLI, namespace, pvcName, hostname string) (*v1.Pod, error) {
	allowPrivEsc := false
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "force-detach-pod",
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			NodeSelector: map[string]string{"kubernetes.io/hostname": hostname},
			Containers: []v1.Container{
				{
					Name:    "test",
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: []string{"sleep", "86400"},
					VolumeMounts: []v1.VolumeMount{
						{Name: "pvc-data", MountPath: "/mnt"},
					},
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivEsc,
						SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
						Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "pvc-data",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
					},
				},
			},
		},
	}
	pod, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	if err := e2epod.WaitTimeoutForPodRunningInNamespace(ctx, oc.AdminKubeClient(), pod.Name, namespace, podRunningTimeout); err != nil {
		return pod, err
	}
	return oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
}

// stopKubeletWithAutoRecovery stops kubelet on the node using a detached transient
// systemd unit that restarts it after kubeletDownDuration. Because the unit is
// detached, it keeps running after the debug pod goes away and after the node goes
// NotReady, so the node recovers without a second node exec (which would be
// impossible while kubelet is down).
//
// The unit sleeps kubeletStopDelay before stopping kubelet so the `oc debug`
// invocation can detach and remove its own debug pod (which requires a live
// kubelet) before kubelet goes down; otherwise `oc debug` times out cleaning up.
// No explicit --unit name is given so a retry cannot collide with an existing unit.
func stopKubeletWithAutoRecovery(oc *exutil.CLI, nodeName string) {
	script := fmt.Sprintf("sleep %d; systemctl stop kubelet; sleep %d; systemctl start kubelet",
		int(kubeletStopDelay.Seconds()), int(kubeletDownDuration.Seconds()))
	_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, oc.Namespace(),
		"systemd-run", "--collect", "/bin/bash", "-c", script)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to stop kubelet on node %s", nodeName)
}

// waitForNodeReadiness waits until the node's Ready condition matches wantReady.
func waitForNodeReadiness(ctx context.Context, oc *exutil.CLI, nodeName string, wantReady bool, timeout time.Duration) {
	ok := e2enode.WaitConditionToBe(ctx, oc.AdminKubeClient(), nodeName, v1.NodeReady, wantReady, timeout)
	o.Expect(ok).To(o.BeTrue(), "node %s did not reach Ready=%v within %s", nodeName, wantReady, timeout)
}

// [Suite:openshift/disruptive-longrunning] routes these to the disruptive suite,
// which runs under Disruptive cluster-stability so the intentional kubelet outage
// does not trip availability/health monitors that would false-fail under Stable.
// [Timeout:75m] overrides the default 15m per-test timeout and gives margin over
// the serial stages a scenario runs even on a slow cluster: the KCM rollout to
// apply the policy (~8-10m), pod startup, the ~7m force-detach observation window,
// node recovery (gated by the 15m kubeletDownDuration), and a second KCM rollout to
// restore in AfterEach — each bounded by kcmRolloutTimeout, which the budget must
// exceed for graceful failures. (Realistic end-to-end runtime is ~38m healthy,
// ~50-58m loaded; 75m leaves headroom without budgeting for the broken-cluster
// case where every cap fires at once and restore could not complete anyway.)
var _ = g.Describe("[sig-storage][Suite:openshift/disruptive-longrunning][OCPFeatureGate:ControllerManagerConfig][Serial][Disruptive][apigroup:config.openshift.io][Skipped:SingleReplicaTopology][Timeout:75m] ControllerManager volumeForceDetach behavior", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLIWithPodSecurityLevel("volume-force-detach-behavior", admissionapi.LevelPrivileged)
		original       *configv1alpha1.ControllerManager
		originalExists bool
		captured       bool
		nodeName       string
		pvcName        string
	)

	g.BeforeEach(func(ctx context.Context) {
		// Reset before the skip checks: AfterEach still runs after a BeforeEach skip,
		// and must not touch the ControllerManager CR / node if capture never happened.
		captured = false
		nodeName = ""
		pvcName = ""
		if enabled, reason := isControllerManagerConfigEnabled(ctx, oc); !enabled {
			g.Skip(reason)
		}
		if FindDefaultStorageClass(oc) == nil {
			g.Skip("No default StorageClass found")
		}
		original, originalExists = captureControllerManager(ctx, oc)
		captured = true
	})

	g.AfterEach(func(ctx context.Context) {
		if !captured {
			return
		}
		// Register restore and PVC deletion as cleanups up front so a slow/failed
		// node recovery (the wait below) cannot skip them. DeferCleanup runs LIFO, so
		// registering restore first makes it run last: node wait -> PVC delete -> restore.
		g.DeferCleanup(func(ctx context.Context) {
			restoreControllerManagerAndSettle(ctx, oc, original, originalExists)
		})
		if pvcName != "" {
			g.DeferCleanup(func(ctx context.Context) {
				// The pod is already force-deleted; delete the PVC so a leaked
				// PV/VolumeAttachment does not linger past namespace teardown.
				err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, pvcName, metav1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete PVC during cleanup")
				}
			})
		}
		// Wait for the node to recover (the detached systemd unit restarts kubelet)
		// so the volume is released and the namespace can be torn down cleanly.
		if nodeName != "" {
			waitForNodeReadiness(ctx, oc, nodeName, true, kubeletDownDuration+nodeRecoveryMargin)
		}
	})

	// runScenario sets the policy, pins a volume pod to a node, makes the node
	// NotReady, force-deletes the pod, and returns (nodeName, pvName) for the
	// caller to assert force-detach behavior on.
	runScenario := func(ctx context.Context, policy configv1alpha1.VolumeForceDetachPolicy) (string, string) {
		g.By("selecting a schedulable worker node")
		node, ok := pickSchedulableWorker(ctx, oc)
		if !ok {
			g.Skip("no schedulable worker node available (cannot safely stop kubelet)")
		}

		g.By(fmt.Sprintf("setting volumeForceDetach=%s and waiting for KCM to converge", policy))
		withKCMRollout(ctx, oc, expectedForceDetachFlag(policy), func() {
			setVolumeForceDetach(ctx, oc, policy)
		})

		nodeName = node.Name
		hostname := node.Labels["kubernetes.io/hostname"]
		if hostname == "" {
			hostname = node.Name
		}

		g.By("creating a PVC and a pod pinned to the node")
		pvc, err := createTestPVC(ctx, oc, oc.Namespace(), "force-detach-pvc", "1Gi")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create PVC %q", "force-detach-pvc")
		pvcName = pvc.Name
		pod, err := createVolumePodOnNode(ctx, oc, oc.Namespace(), pvc.Name, hostname)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod %q", "force-detach-pod")

		g.By("resolving the bound PV and confirming the volume is attached")
		pvc, err = oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, pvc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to retrieve PVC %q", pvcName)
		pvName := pvc.Spec.VolumeName
		o.Expect(pvName).NotTo(o.BeEmpty(), "PVC is not bound to a PV")
		o.Eventually(func() bool {
			exists, err := volumeAttachmentExistsForPV(ctx, oc, nodeName, pvName)
			if err != nil {
				e2e.Logf("listing VolumeAttachments failed (will retry): %v", err)
				return false
			}
			return exists
		}, volumeAttachTimeout, 5*time.Second).Should(o.BeTrue(), "volume was never attached to the node")

		g.By("stopping kubelet to make the node NotReady (auto-recovers)")
		stopKubeletWithAutoRecovery(oc, nodeName)
		waitForNodeReadiness(ctx, oc, nodeName, false, nodeNotReadyTimeout)

		g.By("force-deleting the pod")
		err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: ptr.To[int64](0),
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to force-delete pod %q", pod.Name)

		return nodeName, pvName
	}

	g.It("should NOT force detach a volume after timeout when volumeForceDetach is OnOutOfServiceTaintOnly", func(ctx context.Context) {
		node, pvName := runScenario(ctx, configv1alpha1.VolumeForceDetachOnOutOfServiceTaintOnly)

		g.By("verifying the volume remains attached past the force-detach timeout")
		var lastListErr error
		o.Consistently(func() bool {
			exists, err := volumeAttachmentExistsForPV(ctx, oc, node, pvName)
			lastListErr = err
			if err != nil {
				// Tolerate a transient API error mid-window: treat it as "still
				// attached" so a blip is not mistaken for a (wrong) force detach.
				e2e.Logf("listing VolumeAttachments failed (treating as still attached): %v", err)
				return true
			}
			return exists
		}, noForceDetachObserveDuration, 30*time.Second).Should(o.BeTrue(),
			"volume was force detached even though force detach on timeout is disabled")
		// The tolerance above could let the window end on an unsuccessful list, which
		// would pass without confirming persistence; require the final poll to have
		// actually listed the attachment state.
		o.Expect(lastListErr).NotTo(o.HaveOccurred(),
			"could not confirm the volume remained attached at the end of the observation window")
	})

	g.It("should force detach a volume after timeout when volumeForceDetach is OnUnmountTimeout", func(ctx context.Context) {
		node, pvName := runScenario(ctx, configv1alpha1.VolumeForceDetachOnUnmountTimeout)

		g.By("verifying the volume is force detached after the timeout")
		o.Eventually(func() bool {
			exists, err := volumeAttachmentExistsForPV(ctx, oc, node, pvName)
			if err != nil {
				// Treat a transient API error as "still attached" so a force detach
				// is only concluded from a successful list, not from a blip.
				e2e.Logf("listing VolumeAttachments failed (treating as still attached): %v", err)
				return true
			}
			return exists
		}, forceDetachObserveTimeout, 15*time.Second).Should(o.BeFalse(),
			"volume was not force detached even though force detach on timeout is enabled")
	})
})
