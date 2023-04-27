package devices

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeletdevicepluginv1beta1 "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

// This test has a very high disruption potential: it wants to bypass the node cordoning et. al.
// and kubernetes management and trigger a reboot straight from the node.
// Additionally, it explicitly considers SNO, where the disruption potential is even higher.
// Because of all the above, it must opted-in explicitly by supplying the `DEVICE_RECOVERY_TARGET_NODE`
// variable pointing to a node in the cluster, SNO or worker node.

const (
	workloadCommand = "devs=$(ls /tmp/ | egrep '^Dev-[0-9]+$'); [ -z $devs ] && { echo MISSING; exit 1; }; { echo devices=$devs; sleep 24h; }" // sleep forever in the test timescale

	setupRegistrationCommandMCD = "mkdir -p /rootfs/var/lib/kubelet/device-plugins/sample && touch /rootfs/var/lib/kubelet/device-plugins/sample/registration"
	// it's just a file plus just a directory. We reverse the step to be on the safer side and minimize the negative effects on bugs.
	// note the file is expected to be gone, so we must tolerate the rm to fail. In any case, rmdir should never fail.
	cleanupRegistrationCommandMCD = "rm /rootfs/var/lib/kubelet/device-plugins/sample/registration; rmdir /rootfs/var/lib/kubelet/device-plugins/sample"
	unblockRegistrationCommandMCD = "rm /rootfs/var/lib/kubelet/device-plugins/sample/registration"
	rebootNodeCommandMCD          = "chroot /rootfs systemctl reboot"

	sampleDeviceResourceName            = "example.com/resource"
	sampleDevicePluginPodName           = "sample-device-plugin"
	sampleDeviceEnvVarNamePluginSockDir = "PLUGIN_SOCK_DIR"

	sampleDeviceEnvVarNameControlFile = "REGISTER_CONTROL_FILE"
	sampleDevicePluginControlFilePath = "/var/lib/kubelet/device-plugins/sample/registration"

	targetNodeEnvVar = "DEVICE_RECOVERY_TARGET_NODE"

	// this is the only timeout tunable because is a hard sleep. Every other long timeout is within a polling loop.
	rebootCooldownEnvVar  = "REBOOT_COOLDOWN_TIME"
	rebootCooldownDefault = 1 * time.Minute
)

var (
	sampleDevicePluginPodLabel = map[string]string{
		"device-recovery-test-pod": "device-plugin",
	}
	workloadPodLabel = map[string]string{
		"device-recovery-test-pod": "workload",
	}
)

var _ = g.Describe("[sig-node][Serial][Slow][Disruptive][Feature:DeviceManager] Device management tests", func() {
	defer g.GinkgoRecover()

	var (
		oc     = exutil.NewCLIWithPodSecurityLevel("devices", admissionapi.LevelPrivileged).AsAdmin()
		fwk    = oc.KubeFramework()
		k8sCli kubernetes.Interface

		rebootCooldownTime = rebootCooldownDefault

		targetNode string
	)

	g.BeforeEach(func() {
		k8sCli = oc.KubeClient()

		targetNode = os.Getenv(targetNodeEnvVar)
		if targetNode == "" {
			e2eskipper.Skipf("Need an explicit target node name, got none")
		}
		framework.Logf("target node name: %q", targetNode)

		var err error
		if val, ok := os.LookupEnv(rebootCooldownEnvVar); ok {
			rebootCooldownTime, err = time.ParseDuration(val)
			o.Expect(err).ToNot(o.HaveOccurred(), "error creating workload pod")
		}
		framework.Logf("reboot cooldown time: %v", rebootCooldownTime)

		node, err := k8sCli.CoreV1().Nodes().Get(context.Background(), targetNode, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error getting the target node %q", targetNode)

		runCommandOnNodeThroughMCDOrFail(oc, node, "setup registration control", setupRegistrationCommandMCD)
		// we use unwrapped exutil.ExecCommandOnMachineConfigDaemon instead of the usual runCommandOnNodeThroughMCDOrFail because:
		// 1. logging and situation awareness will be provided by DeferCleanup - e.g. we will know _when_ we try this operation
		// 2. we let DeferCleanup handle the error, see https://pkg.go.dev/github.com/onsi/ginkgo/v2#DeferCleanup definition #2 and #4
		g.DeferCleanup(exutil.ExecCommandOnMachineConfigDaemon, fwk.ClientSet, oc, node, []string{"sh", "-c", cleanupRegistrationCommandMCD})
	})

	g.It("Verify that pods requesting devices are correctly recovered on node restart", func() {
		namespace := fwk.Namespace.Name // shortcut

		// refresh the targetNode object
		node, err := k8sCli.CoreV1().Nodes().Get(context.Background(), targetNode, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error getting the target node %q", targetNode)

		// phase1: complete the node praparation: run the device plugin pod, make sure it registers, the node exposes the devices
		dpPod := makeSampleDevicePluginPod(namespace)
		dpPod.Spec.NodeName = targetNode

		dpPod, err = k8sCli.CoreV1().Pods(namespace).Create(context.Background(), dpPod, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error creating sample device plugin pod")
		framework.Logf("created sample device plugin pod %s/%s on node %q", dpPod.Namespace, dpPod.Name, node.Name)

		// short timeout: we are on idle cluster
		waitForPodRunningOrFail(k8sCli, dpPod.Namespace, dpPod.Name, 1*time.Minute)
		framework.Logf("pod %s/%s running", dpPod.Namespace, dpPod.Name)

		runCommandOnNodeThroughMCDOrFail(oc, node, "unblock device registration", unblockRegistrationCommandMCD)

		framework.Logf("wait for target node %q to report resources", targetNode)
		var allocatableDevs int64
		o.Eventually(func() (bool, error) {
			node, err := k8sCli.CoreV1().Nodes().Get(context.Background(), targetNode, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			allocatableDevs = countSampleDeviceAllocatable(node)
			return allocatableDevs > 0, nil
		}).WithTimeout(2*time.Minute).WithPolling(2*time.Second).Should(o.BeTrue(), "cannot get the allocatable device status on %q", targetNode)
		framework.Logf("reporting resources from node %q: %v=%d", node.Name, sampleDeviceResourceName, allocatableDevs)

		// phase2: node is prepared, run the test workload and check it gets the device it expected
		wlPod := makeWorkloadPod(namespace, sampleDeviceResourceName, workloadCommand)

		wlPod, err = k8sCli.CoreV1().Pods(namespace).Create(context.Background(), wlPod, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error creating workload pod")

		// short timeout: we are on idle cluster
		wlPod = waitForPodRunningOrFail(k8sCli, wlPod.Namespace, wlPod.Name, 1*time.Minute)
		framework.Logf("pod %s/%s running", wlPod.Namespace, wlPod.Name)

		// phase3: the reboot, which trigger the very scenario we are after

		// now we want the device plugin to NOT register itself on reboot,
		// so we need to rearm the control file before the node restart.
		// This will ensure the order of events we expect after the reboot.
		// We're redoing part of the preparation we did in BeforeEach, so the cleanup
		// routine will take care with no extra work needed.
		runCommandOnNodeThroughMCDOrFail(oc, node, "setup registration control", setupRegistrationCommandMCD)
		// managed, clean restart (e.g. `reboot` command or `systemctl reboot`
		// - details don't matter as long as this is a managed clean restart).
		// Power loss scenarios, aka hard reboot, deferred to another test.
		runCommandOnNodeThroughMCDOrFail(oc, node, "reboot", rebootNodeCommandMCD)
		// this is (likely) a SNO. We need to tolerate connection errors,
		// because the apiserver is going down as well.
		// we intentionally use a generous timeout.
		// On Bare metal reboot can take a while.
		o.Eventually(func() (bool, error) {
			node, err := k8sCli.CoreV1().Nodes().Get(context.Background(), targetNode, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return isNodeReady(*node), nil
		}).WithTimeout(15*time.Minute).WithPolling(3*time.Second).Should(o.BeTrue(), "post reboot: cannot get readiness status after reboot for node %q", targetNode)
		framework.Logf("post reboot: node %q: reported ready again", node.Name)

		// on SNO we have a extra set of challenges after reboot.
		// It's possible (and it was observed when writing the test, OCP 4.13/4.14)
		// that the apiserver will report stale data until the kubelet updates.
		// So we need to tolerate a period on which we receive misleading information,
		// like pods reported running and flap to ContainerCreating after a short while
		// We need a long timeout here because we are in the challenging post-reboot status
		waitForPodRunningOrFail(k8sCli, dpPod.Namespace, dpPod.Name, 15*time.Minute)
		framework.Logf("post reboot: pod %s/%s running", dpPod.Namespace, dpPod.Name)

		// are we really sure? we can't predict if we will have state flapping,
		// we can't predict if pods go back to containercreating and ideally we
		// should have no flapping.
		// Tracking all the state will make the test complex *and fragile*.
		// The best we can do right now is to let the SNO cool down and check again.
		framework.Logf("post reboot: entering cooldown time: %v", rebootCooldownTime)
		time.Sleep(rebootCooldownTime)
		framework.Logf("post reboot: finished cooldown time: %v", rebootCooldownTime)

		// if this passes after the initial check + cooldown, we can only assume the plugin is actually running
		// we still need to be careful so we use a long timeout
		waitForPodRunningOrFail(k8sCli, dpPod.Namespace, dpPod.Name, 10*time.Minute)
		framework.Logf("post reboot: pod %s/%s running (doublecheck, assuming really running)", dpPod.Namespace, dpPod.Name)

		// note now the plugin is running, but it did NOT register itself
		o.Consistently(func() (bool, error) {
			node, err := k8sCli.CoreV1().Nodes().Get(context.Background(), targetNode, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			allocatableDevs = countSampleDeviceAllocatable(node)
			return allocatableDevs == 0, nil
		}).WithTimeout(1*time.Minute).WithPolling(2*time.Second).Should(o.BeTrue(), "post reboot: resource %q unexpectedly available on %q", sampleDeviceResourceName, targetNode)
		framework.Logf("post reboot: resource %q not available on %q (registration not completed)", sampleDeviceResourceName, targetNode)

		runCommandOnNodeThroughMCDOrFail(oc, node, "unblock device registration", unblockRegistrationCommandMCD)

		framework.Logf("post reboot: wait for target node %q to report resources", targetNode)
		var allocatableDevs2 int64
		o.Eventually(func() (bool, error) {
			node, err := k8sCli.CoreV1().Nodes().Get(context.Background(), targetNode, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			allocatableDevs2 = countSampleDeviceAllocatable(node)
			return allocatableDevs2 > 0, nil
		}).WithTimeout(1*time.Minute).WithPolling(2*time.Second).Should(o.BeTrue(), "cannot get the allocatable device status on %q", targetNode)
		framework.Logf("post reboot: reporting resources from node %q: %v=%d", node.Name, sampleDeviceResourceName, allocatableDevs2)

		// what happened to the previous workload?
		rejectedPod, err := k8sCli.CoreV1().Pods(wlPod.Namespace).Get(context.Background(), wlPod.Name, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error getting the initial workload %s/%s", wlPod.Namespace, wlPod.Name)
		o.Expect(isUnexpectedAdmissionError(*rejectedPod)).To(o.BeTrue(), "initial workload pod %s/%s in unexpected status: %s", rejectedPod.Namespace, rejectedPod.Name, extractPodStatus(rejectedPod.Status))
		framework.Logf("post reboot: existing workload pod %s/%s failed admission: %s", rejectedPod.Namespace, rejectedPod.Name, extractPodStatus(rejectedPod.Status))

		// phase4: sanity check that a new workload works as expected
		wlPod2 := makeWorkloadPod(namespace, sampleDeviceResourceName, workloadCommand)

		wlPod2, err = k8sCli.CoreV1().Pods(namespace).Create(context.Background(), wlPod2, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error creating workload pod post reboot")

		// things should be settled now so we can use again a short timeout
		wlPod2 = waitForPodRunningOrFail(k8sCli, wlPod2.Namespace, wlPod2.Name, 1*time.Minute)
		framework.Logf("post reboot: newer workload pod %s/%s admitted: %s", wlPod2.Namespace, wlPod2.Name, extractPodStatus(wlPod2.Status))
	})
})

func makeSampleDevicePluginPod(namespace string) *corev1.Pod {
	isTrue := true
	labels := map[string]string{}
	for key, value := range sampleDevicePluginPodLabel {
		labels[key] = value
	}
	labels["k8s-app"] = "sample-device-plugin"

	podDef := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sampleDevicePluginPodName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			PriorityClassName: "system-node-critical",
			Tolerations: []corev1.Toleration{
				{
					Effect:   "NoExecute",
					Operator: corev1.TolerationOpExists,
				},
				{
					Effect:   "NoSchedule",
					Operator: corev1.TolerationOpExists,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "device-plugin",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/kubelet/device-plugins",
						},
					},
				},
				{
					Name: "plugins-registry-probe-mode",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/kubelet/plugins_registry",
						},
					},
				},
				{
					Name: "dev",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/dev",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "sample-device-plugin",
					Image: "registry.k8s.io/e2e-test-images/sample-device-plugin:1.5",
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
					},
					Env: []corev1.EnvVar{
						{
							Name:  sampleDeviceEnvVarNamePluginSockDir,
							Value: kubeletdevicepluginv1beta1.DevicePluginPath,
						},
						{
							Name:  sampleDeviceEnvVarNameControlFile,
							Value: sampleDevicePluginControlFilePath,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "device-plugin",
							MountPath: "/var/lib/kubelet/device-plugins",
						},
						{
							Name:      "plugins-registry-probe-mode",
							MountPath: "/var/lib/kubelet/plugins_registry",
						},
						{
							Name:      "dev",
							MountPath: "/dev",
						},
					},
				},
			},
		},
	}
	return podDef
}

func makeWorkloadPod(namespace, SampleDeviceResourceName, cmd string) *corev1.Pod {
	zero := int64(0)
	rl := corev1.ResourceList{
		corev1.ResourceName(SampleDeviceResourceName): *resource.NewQuantity(1, resource.DecimalSI),
	}
	podDef := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "sample-device-workload-",
			Namespace:    namespace,
			Labels:       workloadPodLabel,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyAlways,
			TerminationGracePeriodSeconds: &zero,
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{{
				Image: k8simage.GetE2EImage(k8simage.BusyBox),
				Name:  "sample-device-workload",
				Command: []string{
					"sh", "-c", cmd,
				},
				Resources: corev1.ResourceRequirements{
					Limits:   rl,
					Requests: rl,
				},
			}},
		},
	}
	return podDef
}

func waitForPodRunningOrFail(k8sCli kubernetes.Interface, podNamespace, podName string, timeout time.Duration) *corev1.Pod {
	var updatedPod *corev1.Pod
	var err error
	o.EventuallyWithOffset(1, func() (bool, error) {
		updatedPod, err = k8sCli.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// PodIsRunning checks only the phase, is too weak of a check of our purpose
		return exutil.CheckPodIsReady(*updatedPod), nil
	}).WithTimeout(timeout).WithPolling(5*time.Second).Should(o.BeTrue(), "pod %s/%s never got running", podNamespace, podName)
	return updatedPod
}

func runCommandOnNodeThroughMCDOrFail(oc *exutil.CLI, node *corev1.Node, description, command string) {
	framework.Logf("node %q: before %s", node.Name, description)
	fwk := oc.KubeFramework()
	_, err := exutil.ExecCommandOnMachineConfigDaemon(fwk.ClientSet, oc, node, []string{"sh", "-c", command})
	o.ExpectWithOffset(1, err).ToNot(o.HaveOccurred(), "failed to run command=%q on node=%q", command, node.Name)
	framework.Logf("node %q: after %s", node.Name, description)
}

func countSampleDeviceAllocatable(node *corev1.Node) int64 {
	val, ok := node.Status.Allocatable[corev1.ResourceName(sampleDeviceResourceName)]
	if !ok {
		return 0
	}
	return val.Value()
}

func isNodeReady(node corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isUnexpectedAdmissionError(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodFailed {
		return false
	}
	if pod.Status.Reason != "UnexpectedAdmissionError" {
		return false
	}
	if !strings.Contains(pod.Status.Message, "Allocate failed") {
		return false
	}
	if !strings.Contains(pod.Status.Message, "unhealthy devices "+sampleDeviceResourceName) {
		return false
	}
	return true
}

func extractPodStatus(podStatus corev1.PodStatus) string {
	return fmt.Sprintf("phase=%q reason=%q message=%q", podStatus.Phase, podStatus.Reason, podStatus.Message)
}
