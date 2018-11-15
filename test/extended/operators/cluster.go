package operators

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/beorn7/perks/quantile"

	"github.com/beorn7/perks/histogram"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Platform][Suite:openshift/smoke-4] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have no crashlooping pods in core namespaces over two minutes", func() {
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		var lastPodsWithProblems []*corev1.Pod
		var pending map[string]*corev1.Pod
		wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			allPods, err := c.Core().Pods("").List(metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var pods []*corev1.Pod
			for i := range allPods.Items {
				pod := &allPods.Items[i]
				if !strings.HasPrefix(pod.Namespace, "openshift-") && !strings.HasPrefix(pod.Namespace, "kube-") {
					continue
				}
				pods = append(pods, pod)
			}

			if pending == nil {
				pending = make(map[string]*corev1.Pod)
				for _, pod := range pods {
					if pod.Status.Phase == corev1.PodPending {
						pending[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = pod
					}
				}
			} else {
				for _, pod := range pods {
					if pod.Status.Phase != corev1.PodPending {
						delete(pending, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
					}
				}
			}

			var podsWithProblems []*corev1.Pod
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
				names = append(names, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
				podsWithProblems = append(podsWithProblems, pod)
			}
			if len(names) > 0 {
				e2e.Logf("Some pods in error: %s", strings.Join(names, ", "))
			}
			lastPodsWithProblems = podsWithProblems
			return false, nil
		})
		var msg []string
		ns := make(map[string]struct{})
		for _, pod := range lastPodsWithProblems {
			delete(pending, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			if _, ok := ns[pod.Namespace]; !ok {
				e2e.DumpEventsInNamespace(func(opts metav1.ListOptions, ns string) (*corev1.EventList, error) {
					return c.CoreV1().Events(ns).List(opts)
				}, pod.Namespace)
				ns[pod.Namespace] = struct{}{}
			}
			status, _ := json.MarshalIndent(pod.Status, "", "  ")
			e2e.Logf("Pod status %s/%s:\n%s", pod.Namespace, pod.Name, string(status))
			msg = append(msg, fmt.Sprintf("Pod %s/%s is not healthy: %v", pod.Namespace, pod.Name, pod.Status.Message))
		}

		for _, pod := range pending {
			if _, ok := ns[pod.Namespace]; !ok {
				e2e.DumpEventsInNamespace(func(opts metav1.ListOptions, ns string) (*corev1.EventList, error) {
					return c.CoreV1().Events(ns).List(opts)
				}, pod.Namespace)
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

	g.It("respond to 99% of API requests within 1s over 2 minutes with < 0.5% error rate", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		cfg.QPS = 100
		cfg.Burst = 100
		cfg.Timeout = 10 * time.Second
		cfg.Dial = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
		c, err := coreclient.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		for i := 0; i < 3; i++ {
			_, err := c.Namespaces().Get("kube-system", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		h := histogram.New(10)
		q := quantile.NewHighBiased(0.01)
		min, max := float64(1e30), float64(0)
		var errs []string

		start := time.Now()
		for i := 0; ; i++ {
			// create a new connection to get a different load balancer every reasonable chunk
			if (i+1)%20 == 0 {
				cfg.Dial = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
				c, err = coreclient.NewForConfig(cfg)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			startGet := time.Now()
			_, err := c.Namespaces().Get("kube-system", metav1.GetOptions{})
			endGet := time.Now()
			if err != nil {
				errs = append(errs, err.Error())
			} else {
				s := endGet.Sub(startGet).Seconds()
				if s > max {
					max = s
				}
				if s < min {
					min = s
				}
				h.Insert(s)
				q.Insert(s)
			}
			if endGet.Sub(start) > 2*time.Minute {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		for _, b := range h.Bins() {
			e2e.Logf("Bin %2.3f: %3d", b.Mean(), b.Count)
		}
		e2e.Logf("Quantile   0: %2.3f", min)
		for _, quantile := range []float64{0.5, 0.9, 0.99} {
			e2e.Logf("Quantile %3.0f: %2.3f", quantile*100, q.Query(quantile))
		}
		e2e.Logf("Quantile 100: %2.3f", max)
		e2e.Logf("Errors: %d", len(errs))

		o.Expect(q.Query(0.50)).To(o.BeNumerically("<", 0.25))
		o.Expect(q.Query(0.90)).To(o.BeNumerically("<", 0.5))
		o.Expect(q.Query(0.99)).To(o.BeNumerically("<", 1))
		o.Expect(float64(len(errs))/float64(q.Count())).To(o.BeNumerically("<", 0.005), "\n\n"+strings.Join(errs, "\n"))
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
