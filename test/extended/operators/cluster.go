package operators

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Platform] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have no crashlooping pods in core namespaces over two minutes", func() {
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		podsWithProblems := make(map[string]*corev1.Pod)
		var lastPending map[string]*corev1.Pod
		wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			allPods, err := c.CoreV1().Pods("").List(metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var pods []*corev1.Pod
			for i := range allPods.Items {
				pod := &allPods.Items[i]
				if !strings.HasPrefix(pod.Namespace, "openshift-") && !strings.HasPrefix(pod.Namespace, "kube-") {
					continue
				}
				pods = append(pods, pod)
			}

			pending := make(map[string]*corev1.Pod)
			for _, pod := range pods {
				if pod.Status.Phase == corev1.PodPending {
					hasInitContainerRunning := false
					for _, initContainerStatus := range pod.Status.InitContainerStatuses {
						if initContainerStatus.State.Running != nil {
							hasInitContainerRunning = true
							break
						}
					}
					if !hasInitContainerRunning {
						pending[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = pod
					}
				}
			}
			lastPending = pending

			var names []string
			for _, pod := range pods {
				if pod.Status.Phase == corev1.PodSucceeded {
					continue
				}
				switch {
				case hasCreateContainerError(pod):
				case hasImagePullError(pod):
				case isCrashLooping(pod):
				case hasExcessiveRestarts(pod):
				case hasFailingContainer(pod):
				default:
					continue
				}
				key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
				names = append(names, key)
				podsWithProblems[key] = pod
			}
			if len(names) > 0 {
				e2e.Logf("Some pods in error: %s", strings.Join(names, ", "))
			}
			return false, nil
		})
		var msg []string
		ns := make(map[string]struct{})
		for _, pod := range podsWithProblems {
			delete(lastPending, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			if _, ok := ns[pod.Namespace]; !ok {
				e2e.DumpAllNamespaceInfo(c, pod.Namespace)
				ns[pod.Namespace] = struct{}{}
			}
			status, _ := json.MarshalIndent(pod.Status, "", "  ")
			e2e.Logf("Pod status %s/%s:\n%s", pod.Namespace, pod.Name, string(status))
			msg = append(msg, fmt.Sprintf("Pod %s/%s is not healthy: %v", pod.Namespace, pod.Name, pod.Status.Message))
		}

		for _, pod := range lastPending {
			if strings.HasPrefix(pod.Name, "must-gather-") {
				e2e.Logf("Pod status %s/%s ignored for being pending", pod.Namespace, pod.Name)
				continue
			}
			if _, ok := ns[pod.Namespace]; !ok {
				e2e.DumpAllNamespaceInfo(c, pod.Namespace)
				ns[pod.Namespace] = struct{}{}
			}
			status, _ := json.MarshalIndent(pod.Status, "", "  ")
			e2e.Logf("Pod status %s/%s:\n%s", pod.Namespace, pod.Name, string(status))
			if len(pod.Status.Message) == 0 {
				pod.Status.Message = "unknown error"
			}
			msg = append(msg, fmt.Sprintf("Pod %s/%s was pending entire time: %v", pod.Namespace, pod.Name, pod.Status.Message))
		}

		o.Expect(msg).To(o.BeEmpty())
	})
})

func hasCreateContainerError(pod *corev1.Pod) bool {
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.State.Waiting != nil {
			if status.State.Waiting.Reason == "CreateContainerError" {
				pod.Status.Message = status.State.Waiting.Message
				if len(pod.Status.Message) == 0 {
					pod.Status.Message = fmt.Sprintf("container %s can't be created", status.Name)
				}
				return true
			}
		}
	}
	return false
}

func hasImagePullError(pod *corev1.Pod) bool {
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.State.Waiting != nil {
			if reason := status.State.Waiting.Reason; reason == "ErrImagePull" || reason == "ImagePullBackOff" {
				pod.Status.Message = status.State.Waiting.Message
				if len(pod.Status.Message) == 0 {
					pod.Status.Message = fmt.Sprintf("container %s can't pull image", status.Name)
				}
				return true
			}
		}
	}
	return false
}

func hasFailingContainer(pod *corev1.Pod) bool {
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			pod.Status.Message = status.State.Terminated.Message
			if len(pod.Status.Message) == 0 {
				pod.Status.Message = fmt.Sprintf("container %s exited with non-zero exit code", status.Name)
			}
			return true
		}
	}
	return pod.Status.Phase == corev1.PodFailed
}

func isCrashLooping(pod *corev1.Pod) bool {
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.State.Waiting != nil {
			if reason := status.State.Waiting.Reason; reason == "CrashLoopBackOff" {
				pod.Status.Message = status.State.Waiting.Message
				if len(pod.Status.Message) == 0 {
					pod.Status.Message = fmt.Sprintf("container %s is crashlooping", status.Name)
				}
				return true
			}
		}
	}
	return false
}

func hasExcessiveRestarts(pod *corev1.Pod) bool {
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.RestartCount > 5 {
			pod.Status.Message = fmt.Sprintf("container %s has restarted more than 5 times", status.Name)
			return true
		}
	}
	return false
}
