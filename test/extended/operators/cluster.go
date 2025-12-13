package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have no crashlooping recycler pods over four minutes", g.Label("Size:M"), func() {
		crashloopingContainerCheck(InCoreNamespaces, recyclerPod)
	})
})

type PodFilter func(pod *corev1.Pod) bool

func InCoreNamespaces(pod *corev1.Pod) bool {
	return strings.HasPrefix(pod.Namespace, "openshift-") || strings.HasPrefix(pod.Namespace, "kube-")
}

func not(filterFn PodFilter) PodFilter {
	return func(pod *corev1.Pod) bool {
		return !filterFn(pod)
	}
}

func recyclerPod(pod *corev1.Pod) bool {
	return strings.HasPrefix(pod.Name, "recycler-for-nfs-")
}

/*var _ = g.Describe("[sig-arch] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have no crashlooping pods in core namespaces over four minutes", func() {
		crashloopingContainerCheck(InCoreNamespaces, not(recyclerPod))
	})
})*/

func crashloopingContainerCheck(podFilters ...PodFilter) {
	c, err := e2e.LoadClientset()
	o.Expect(err).NotTo(o.HaveOccurred())

	restartingContainers := make(map[ContainerName]int)
	podsWithProblems := make(map[string]*corev1.Pod)
	var lastPending map[string]*corev1.Pod
	testStartTime := time.Now()
	wait.PollImmediate(5*time.Second, 4*time.Minute, func() (bool, error) {
		pods := GetPodsWithFilter(c, podFilters)

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
					pending[string(pod.UID)] = pod
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
			case HasExcessiveRestarts(pod, 2, restartingContainers):
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
		delete(lastPending, string(pod.UID))
		if strings.HasPrefix(pod.Name, "samename-") {
			continue
		}
		if _, ok := ns[pod.Namespace]; !ok {
			e2edebug.DumpAllNamespaceInfo(context.TODO(), c, pod.Namespace)
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
		if pod.Status.StartTime != nil && pod.Status.StartTime.After(testStartTime) {
			// At this point lastPending has a list of pods that were pending a few seconds ago, but those pods
			// could have been created a few seconds before that.  What we really want is to know which pods are
			// pending for longer than a predetermined threshold of time.  This test implies that the intent is
			// "find pods which have been pending more than four minutes.
			continue
		}
		if _, ok := ns[pod.Namespace]; !ok {
			e2edebug.DumpAllNamespaceInfo(context.TODO(), c, pod.Namespace)
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
}

func GetPodsWithFilter(c *kubernetes.Clientset, podFilters []PodFilter) []*corev1.Pod {
	allPods, err := c.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	var pods []*corev1.Pod
	for i := range allPods.Items {
		pod := &allPods.Items[i]
		accept := true
		for _, filterFn := range podFilters {
			if !filterFn(pod) {
				accept = false
				break
			}
		}
		if !accept {
			continue
		}
		pods = append(pods, pod)
	}
	return pods
}

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
	// Catalog update pods are deliberately terminated with non-zero exit codes, so this check does not apply to them
	if _, ok := pod.Labels["catalogsource.operators.coreos.com/update"]; ok {
		return false
	}
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			pod.Status.Message = status.State.Terminated.Message
			if len(pod.Status.Message) == 0 {
				pod.Status.Message = fmt.Sprintf("container %s exited with non-zero exit code", status.Name)
			}
			return true
		}
	}
	return false
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

type ContainerName struct {
	namespace string
	name      string
	container string
}

func HasExcessiveRestarts(pod *corev1.Pod, excessiveCount int, counts map[ContainerName]int) bool {
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		name := ContainerName{namespace: pod.Namespace, name: pod.Name, container: status.Name}
		count, ok := counts[name]
		if !ok {
			counts[name] = int(status.RestartCount)
			continue
		}

		current := int(status.RestartCount) - count
		if current >= excessiveCount {
			pod.Status.Message = fmt.Sprintf("container %s has restarted %d times (>= %d) within the allowed interval", status.Name, current, excessiveCount)
			return true
		}
	}
	return false
}
