package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/util/image"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// GetClusterNodesByRole returns the cluster nodes by role
func GetClusterNodesByRole(oc *CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// GetFirstCoreOsWorkerNode returns the first CoreOS worker node
func GetFirstCoreOsWorkerNode(oc *CLI) (string, error) {
	return getFirstNodeByOsID(oc, "worker", "rhcos")
}

// getFirstNodeByOsID returns the cluster node by role and os id
func getFirstNodeByOsID(oc *CLI, role string, osID string) (string, error) {
	nodes, err := GetClusterNodesByRole(oc, role)
	for _, node := range nodes {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+node, "-o", "jsonpath=\"{.metadata.labels.node\\.openshift\\.io/os_id}\"").Output()
		if strings.Trim(stdout, "\"") == osID {
			return node, err
		}
	}
	return "", err
}

// DebugNodeRetryWithOptionsAndChroot launches debug container using chroot with options
// and waitPoll to avoid "error: unable to create the debug pod" and do retry
func DebugNodeRetryWithOptionsAndChroot(oc *CLI, nodeName string, debugNodeNamespace string, cmd ...string) (string, error) {
	var (
		cargs  []string
		stdOut string
		err    error
	)
	cargs = []string{"node/" + nodeName, "-n" + debugNodeNamespace, "--", "chroot", "/host"}
	cargs = append(cargs, cmd...)
	wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		stdOut, _, err = oc.AsAdmin().WithoutNamespace().Run("debug").Args(cargs...).Outputs()
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	return stdOut, err
}

func GetClusterNodesBySelector(oc *CLI, selector string) ([]corev1.Node, error) {
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: selector,
		})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

func GetAllClusterNodes(oc *CLI) ([]corev1.Node, error) {
	return GetClusterNodesBySelector(oc, "")
}

func DebugSelectedNodesRetryWithOptionsAndChroot(oc *CLI, selector string, debugNodeNamespace string, cmd ...string) (map[string]string, error) {
	nodes, err := GetClusterNodesBySelector(oc, selector)
	if err != nil {
		return nil, err
	}
	stdOut := make(map[string]string, len(nodes))
	for _, node := range nodes {
		stdOut[node.Name], err = DebugNodeRetryWithOptionsAndChroot(oc, node.Name, debugNodeNamespace, cmd...)
		if err != nil {
			return stdOut, err
		}
	}
	return stdOut, nil
}

func DebugAllNodesRetryWithOptionsAndChroot(oc *CLI, debugNodeNamespace string, cmd ...string) (map[string]string, error) {
	return DebugSelectedNodesRetryWithOptionsAndChroot(oc, "", debugNodeNamespace, cmd...)
}

// TriggerNodeRebootGraceful initiates a graceful node reboot which allows the system to terminate processes cleanly before rebooting.
func TriggerNodeRebootGraceful(kubeClient kubernetes.Interface, nodeName string) error {
	command := "echo 'reboot in 1 minute'; exec chroot /host shutdown -r 1"
	return triggerNodeDisruption(kubeClient, nodeName, 0, false, command)
}

// TriggerNodeRebootUngraceful initiates an ungraceful node reboot which does not allow the system to terminate processes cleanly before rebooting.
func TriggerNodeRebootUngraceful(kubeClient kubernetes.Interface, nodeName string) error {
	command := "echo 'reboot in 1 minute'; exec chroot /host sudo systemd-run sh -c 'sleep 60 && reboot --force --force'"
	return triggerNodeDisruption(kubeClient, nodeName, 0, false, command)
}

// TriggerNetworkDisruption temporarily disables the specified network interface on the target node
func TriggerNetworkDisruption(kubeClient kubernetes.Interface, target, peer corev1.Node, networkInterface string, disruptionDuration time.Duration) (string, error) {
	command_preable := fmt.Sprintf("echo 'temporarily disabling interface %s on %s for %v'; exec chroot /host sh -c ", networkInterface, target.Name, disruptionDuration)
	interface_down_cmd := fmt.Sprintf("ip link set %s down", networkInterface)
	timeout_cmd := fmt.Sprintf("sleep %d", int(disruptionDuration.Seconds()))
	interface_up_cmd := fmt.Sprintf("ip link set %s up", networkInterface)
	command := fmt.Sprintf("%s '%s ; %s ; %s'", command_preable, interface_down_cmd, timeout_cmd, interface_up_cmd)

	return command, triggerNodeDisruption(kubeClient, target.Name, 0, true, command)
}

func triggerNodeDisruption(kubeClient kubernetes.Interface, nodeName string, attempt int, useHostNetwork bool, command string) error {
	isTrue := true
	zero := int64(0)
	name := fmt.Sprintf("disrupt-%s-%d", nodeName, attempt)
	_, err := kubeClient.CoreV1().Pods("kube-system").Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"test.openshift.io/disrupt-target": nodeName,
			},
		},
		Spec: corev1.PodSpec{
			HostNetwork:   useHostNetwork,
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
	if errors.IsAlreadyExists(err) {
		return triggerNodeDisruption(kubeClient, nodeName, attempt+1, useHostNetwork, command)
	}
	return err
}
