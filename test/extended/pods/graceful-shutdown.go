package pods

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	successPodName = "success-grace-period-pod"
	errorPodName   = "error-grace-period-pod"
	namespace      = "openshift-cluster-node-tuning-operator"
	hostPath       = "/var/graceful-shutdown"
)

var (
	rebootLabel  = map[string]string{"reboot-helper": ""}
	testPodLabel = map[string]string{"graceful-restart-helper": ""}
)

// Helper script to catch SIGTERM and force a wait period
const (
	rebootWorkScript = `
#!/bin/bash

write_file() {
	sleepTime=%0.f
	filePath=%s

	echo "SIGTERM received, sleeping for $sleepTime seconds"
	sleep $sleepTime

	if [ ! -f $filePath ]
	then
		echo "writing $filePath"
		echo "finished" >> $filePath
	else
		echo "file exists, skipping"
	fi
	echo "done"
	exit 0
}

# Once a SIGTERM is received, we wait for the specified time before writing.
trap write_file SIGTERM

while true
do
	echo "Busy working, cycling through the ones and zeros"
	sleep 5
done
`
)

var _ = Describe("[sig-node][Disruptive][Suite:openshift/pods/graceful-shutdown]", Ordered, func() {

	var (
		oc   = exutil.NewCLIWithoutNamespace("pod").SetNamespace(namespace)
		node *corev1.Node
		zero = int64(0)
	)

	BeforeAll(func() {
		ctx := context.Background()
		nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="})
		Expect(err).NotTo(HaveOccurred())
		Expect(nodes.Items).NotTo(HaveLen(0))
		node = &nodes.Items[0]

		// Currently hard coded values
		// TODO: Add dynamic support for current existing kubelet config.
		// renderedConfigName := node.Labels["machineconfiguration.openshift.io/currentConfig"]
		successPod := gracefulPodSpec(successPodName, node.Name, time.Minute*2)
		errorPod := gracefulPodSpec(errorPodName, node.Name, time.Minute*10)

		// Create two test pods
		_, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Create(ctx, successPod, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		_, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Create(ctx, errorPod, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		// Wait for pods to be running
		_, err = exutil.WaitForPods(
			oc.AdminKubeClient().CoreV1().Pods(namespace),
			labels.SelectorFromSet(testPodLabel),
			exutil.CheckPodIsRunning, 2, time.Second*60)
		Expect(err).NotTo(HaveOccurred(), "unable to wait for pods")

		// Reboot node
		err = triggerReboot(oc, namespace, node.Name)
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("Kubelet with graceful shutdown configuration", Ordered, func() {
		var outputString = ""

		BeforeAll(func() {
			// Wait for node to be rebooted before examining with debug pod
			err := waitForNode(oc, node.Name)
			Expect(err).NotTo(HaveOccurred(), "unable to watch node for status ready")

			// Run debug pod to get information on the file system
			cmdStr := fmt.Sprintf("exec chroot /host ls %s", hostPath)
			args := []string{"node/" + node.Name, "--to-namespace", namespace, "--", "/bin/bash", "-c", cmdStr}

			// We only need to query the node once and `ls` the desired directory.
			// We can then determine the correct behavior by which file was written.
			err = wait.Poll(time.Second, time.Second*60, func() (done bool, err error) {
				outputString, err = oc.AsAdmin().Run("debug").Args(args...).Output()
				if err != nil && !strings.Contains(outputString, "unable to create the debug pod") {
					return false, err
				}
				return true, nil
			})
			Expect(err).NotTo(HaveOccurred(), "was not able to ls directory %s", hostPath)
		})

		Context("pod with grace period under kubelet range", func() {
			It("should have it's grace period respected", func() {
				Expect(outputString).To(ContainSubstring("%s", successPodName))
			})
		})

		Context("pod with grace period over kubelet range", func() {
			It("should be force terminated", func() {
				Expect(outputString).NotTo(ContainSubstring("%s", errorPodName))
			})
		})
	})

	AfterAll(func() {
		rebootPod := fmt.Sprintf("reboot-%s", node.Name)
		err := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Delete(context.Background(), rebootPod, metav1.DeleteOptions{})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred(), "unable to delete pod (%s)", rebootPod)
		}

		err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).
			Delete(context.Background(), successPodName, metav1.DeleteOptions{GracePeriodSeconds: &zero})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred(), "unable to delete pod (%s)", successPodName)
		}

		err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).
			Delete(context.Background(), errorPodName, metav1.DeleteOptions{GracePeriodSeconds: &zero})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred(), "unable to delete pod (%s)", errorPodName)
		}
	})
})

func filePathForPod(podName string) string {
	return fmt.Sprintf("%s/completed-%s", hostPath, podName)
}

// Wait for the node to come back up after reboot before attempting to validate results
func waitForNode(oc *exutil.CLI, name string) error {
	pollingInterval := time.Second
	timeout := time.Minute * 10
	err := wait.Poll(pollingInterval, timeout, func() (bool, error) {
		node, err := oc.AsAdmin().
			KubeClient().
			CoreV1().
			Nodes().Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	return err
}

// Force a node reboot
func triggerReboot(oc *exutil.CLI, namespace, nodeName string) error {
	isTrue := true
	zero := int64(0)
	podName := fmt.Sprintf("reboot-%s", nodeName)
	command := "exec chroot /host systemctl reboot"
	_, err := oc.AsAdmin().KubeClient().CoreV1().
		Pods(namespace).
		Create(context.Background(), &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   podName,
				Labels: rebootLabel,
			},
			Spec: corev1.PodSpec{
				HostPID:       true,
				RestartPolicy: corev1.RestartPolicyNever,
				NodeName:      nodeName,
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "reboot",
						SecurityContext: &corev1.SecurityContext{
							RunAsUser:  &zero,
							Privileged: &isTrue,
						},
						Image: image.ShellImage(),
						Command: []string{
							"/bin/bash",
							"-c",
							command,
						},
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/host",
								Name:      "host",
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	_, err = exutil.WaitForPods(
		oc.AdminKubeClient().CoreV1().Pods(namespace),
		labels.SelectorFromSet(rebootLabel),
		func(p corev1.Pod) bool {
			return p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded
		}, 1, time.Second*60)
	return err
}

// Generate a pod spec with the termination grace period specified, and busy work lasting a little less
// then the specified grace period
// The pod will write to the node file system if allowed to fully complete
func gracefulPodSpec(name, nodeName string, gracePeriod time.Duration) *corev1.Pod {
	gracePeriodSecond := int64(gracePeriod.Seconds())
	busyWorkTime := gracePeriod - (time.Second * 20)
	hostDirType := corev1.HostPathDirectoryOrCreate
	isTrue := true
	isZero := int64(0)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: testPodLabel,
		},
		Spec: corev1.PodSpec{
			HostPID:           true,
			RestartPolicy:     corev1.RestartPolicyNever,
			PriorityClassName: "system-cluster-critical",
			Containers: []corev1.Container{
				{
					Image: image.ShellImage(),
					Name:  name,
					Command: []string{
						"/bin/bash",
						"-c",
						fmt.Sprintf(rebootWorkScript, busyWorkTime.Seconds(), filePathForPod(name)),
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						RunAsUser:  &isZero,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host",
							MountPath: hostPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &hostDirType,
						},
					},
				},
			},
			TerminationGracePeriodSeconds: &gracePeriodSecond,
			NodeName:                      nodeName,
		},
	}
}
