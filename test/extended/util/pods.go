package util

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	podframework "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	"github.com/openshift/origin/test/extended/util/image"
)

const (
	namespaceMachineConfigOperator = "openshift-machine-config-operator"
	containerMachineConfigDaemon   = "machine-config-daemon"
)


// HasEnvVar checks if a container has an environment variable with a specific value
func HasEnvVar(container *corev1.Container, name, expectedValue string) bool {
	if container == nil {
		return false
	}
	for _, env := range container.Env {
		if env.Name == name {
			return env.Value == expectedValue
		}
	}
	return false
}

// WaitForNoPodsRunning waits until there are no (running) pods in the given namespace.
// (The idling tests use a DeploymentConfig which will leave a "Completed" deploy pod
// after deploying the service; we don't want to count that.)
func WaitForNoPodsRunning(oc *CLI) error {
	return wait.Poll(200*time.Millisecond, 3*time.Minute, func() (bool, error) {
		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				return false, nil
			}
		}
		return true, nil
	})
}

// RemovePodsWithPrefixes deletes pods whose name begins with the
// supplied prefixes
func RemovePodsWithPrefixes(oc *CLI, prefixes ...string) error {
	e2e.Logf("Removing pods from namespace %s with prefix(es): %v", oc.Namespace(), prefixes)
	pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	errs := []error{}
	for _, prefix := range prefixes {
		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, prefix) {
				if err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
					e2e.Logf("unable to remove pod %s/%s", oc.Namespace(), pod.Name)
					errs = append(errs, err)
				}
			}
		}
	}
	if len(errs) > 0 {
		return kutilerrors.NewAggregate(errs)
	}
	return nil
}

// CreateExecPodOrFail creates a pod used as a vessel for kubectl exec commands.
// Pod name is uniquely generated.
// The security context of this pod complies to the "restricted" profile.
// If necessary this can be overriden in tweaks.
func CreateExecPodOrFail(client kubernetes.Interface, ns, name string, tweak ...func(*v1.Pod)) *v1.Pod {
	return podframework.CreateExecPodOrFail(context.TODO(), client, ns, name, func(pod *v1.Pod) {
		pod.Name = name
		pod.GenerateName = ""
		pod.Spec.Containers[0].Image = image.ShellImage()
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "trap exit TERM; while true; do sleep 5; done"}
		pod.Spec.Containers[0].Args = nil
		pod.Spec.Containers[0].SecurityContext = podframework.GetRestrictedContainerSecurityContext()
		pod.Spec.SecurityContext = podframework.GetRestrictedPodSecurityContext()

		for _, fn := range tweak {
			fn(pod)
		}
	})
}

// GetMachineConfigDaemonByNode finds the privileged daemonset from the Machine Config Operator
func GetMachineConfigDaemonByNode(c clientset.Interface, node *corev1.Node) (*corev1.Pod, error) {
	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
		LabelSelector: labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String(),
	}

	mcds, err := c.CoreV1().Pods(namespaceMachineConfigOperator).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}

	if len(mcds.Items) < 1 {
		e2eskipper.Skipf("The cluster machines are not managed by machine api operator")
	}
	return &mcds.Items[0], nil
}

// ExecCommandOnMachineConfigDaemon returns the output of the command execution on the machine-config-daemon pod that runs on the specified node
func ExecCommandOnMachineConfigDaemon(c clientset.Interface, oc *CLI, node *corev1.Node, command []string) (string, error) {
	mcd, err := GetMachineConfigDaemonByNode(c, node)
	if err != nil {
		return "", err
	}

	initialArgs := []string{
		"-n", namespaceMachineConfigOperator,
		"-c", containerMachineConfigDaemon,
		"--request-timeout", "30",
		mcd.Name,
	}
	args := append(initialArgs, command...)
	return oc.AsAdmin().Run("rsh").Args(args...).Output()
}
