package pods

import (
	"context"
	"fmt"
	"github.com/openshift/origin/test/extended/util/image"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var containerName = "stressng"

var _ = g.Describe("[sig-node][Serial]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("pod-eviction")
	oc := exutil.NewCLIWithFramework(f)

	// The goal of this test to verify if the OOM inducing pod is the one that is getting evicted
	g.It("should evict the same pod that causes memory crunch", func() {
		g.By("creating the pod that stresses memory")
		kubeClient := oc.AdminKubeClient()

		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
		framework.ExpectNoError(err)
		// Node where memory inducing pod would land. Hardcoding this to pass correct memory configurations to exercise
		// eviction and OOMKill scenarios
		memLimit := getMemoryLimitOntheNode(nodes.Items[1])

		podClient := f.PodClient()
		ns := f.Namespace.Name
		podName := naming.GetPodName("mem-stress-pod", string(uuid.NewUUID()))
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:    containerName,
					Image:   image.ShellImage(),
					Command: []string{"stress-ng"},
					Args:    []string{"--vm", "2", "--vm-bytes", memLimit},
				},
				},
			},
		}
		pod = podClient.Create(pod)

		err = e2epod.WaitForPodNameRunningInNamespace(oc.KubeFramework().ClientSet, pod.Name, oc.Namespace())

		framework.ExpectNoError(err)
		defer func() {
			g.By("deleting the pod")
			podClient.Delete(context.TODO(), pod.Name, *metav1.NewDeleteOptions(0))
		}()

		pod, err = podClient.Get(context.TODO(), pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pod running on node %s", pod.Spec.NodeName)
		// To wait for pod to start hogging memory
		time.Sleep(2 * time.Minute)

		testDuration := fmt.Sprintf("%v", exutil.DurationSinceStartInSeconds().Seconds()) + "s"
		out, err := oc.Run("adm", "node-logs").Args(pod.Spec.NodeName, "--path=journal",
			"--case-sensitive=false", "--grep=Memory cgroup out of memory", "--since=-"+testDuration).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		outputLines := strings.Split(out, "\n")
		totalLines := 0
		misses := 0
		for _, outputLine := range outputLines {
			/* Output Format
			 Logs begin at Thu 2021-05-06 15:07:27 UTC, end at Thu 2021-05-06 20:02:55 UTC. --
			May  6 16:02:55.490: INFO: -- Logs begin at thu 2021-05-06 15:07:27 utc, end at thu 2021-05-06 20:02:55 utc. --
			May  6 16:02:55.490: INFO: -- No entries --
			May  6 16:02:55.490: INFO: -- Logs begin at Thu 2021-05-06 15:07:27 UTC, end at Thu 2021-05-06 20:02:55 UTC. --
			*/
			if strings.EqualFold(outputLine, "-- No entries --") {
				continue
			}
			if strings.Contains(strings.ToLower(outputLine), "end at") { // Logs begin at also contain end at
				continue
			}
			totalLines++
			if !strings.Contains(strings.ToLower(outputLine), "stress-ng") {
				misses++
			}
		}
		if misses != 0 {
			result.Flakef("From Nodelogs: expected oom inducing pod to be oomkilled but other pod oom killed "+
				"%v of times, %v, %v", float64(misses/totalLines), misses, totalLines)
		}
	})
})

var _ = g.Describe("[sig-node][Serial]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("oom-kills-oup")
	oc := exutil.NewCLIWithFramework(f)

	// The goal of this test to verify if the OOM inducing pod is the one that is getting evicted
	g.It("should oom kill the pod with openshift user critical priority class which induced it", func() {
		g.By("creating the pod that stresses memory")
		kubeClient := oc.AdminKubeClient()

		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
		framework.ExpectNoError(err)
		// Node where memory inducing pod would land. Hardcoding this to pass correct memory configurations to exercise
		// eviction and OOMKill scenarios
		memLimit := getMemoryLimitOntheNode(nodes.Items[1])

		podClient := f.PodClient()
		ns := f.Namespace.Name
		podName := naming.GetPodName("mem-stress-pod", string(uuid.NewUUID()))
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
			},
			Spec: v1.PodSpec{
				PriorityClassName: "openshift-user-critical",
				Containers: []v1.Container{{
					Name:    containerName,
					Image:   image.ShellImage(),
					Command: []string{"stress-ng"},
					Args:    []string{"--vm", "2", "--vm-bytes", memLimit},
				},
				},
			},
		}
		pod = podClient.Create(pod)

		err = e2epod.WaitForPodNameRunningInNamespace(oc.KubeFramework().ClientSet, pod.Name, oc.Namespace())

		framework.ExpectNoError(err)
		defer func() {
			g.By("deleting the pod")
			podClient.Delete(context.TODO(), pod.Name, *metav1.NewDeleteOptions(0))
		}()

		pod, err = podClient.Get(context.TODO(), pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pod running on node %s", pod.Spec.NodeName)
		// To wait for pod to start hogging memory
		time.Sleep(2 * time.Minute)

		testDuration := fmt.Sprintf("%v", exutil.DurationSinceStartInSeconds().Seconds()) + "s"
		out, err := oc.Run("adm", "node-logs").Args(pod.Spec.NodeName, "--path=journal",
			"--case-sensitive=false", "--grep=Memory cgroup out of memory", "--since=-"+testDuration).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		outputLines := strings.Split(out, "\n")
		totalLines := 0
		misses := 0
		for _, outputLine := range outputLines {
			/* Output Format
			 Logs begin at Thu 2021-05-06 15:07:27 UTC, end at Thu 2021-05-06 20:02:55 UTC. --
			May  6 16:02:55.490: INFO: -- Logs begin at thu 2021-05-06 15:07:27 utc, end at thu 2021-05-06 20:02:55 utc. --
			May  6 16:02:55.490: INFO: -- No entries --
			May  6 16:02:55.490: INFO: -- Logs begin at Thu 2021-05-06 15:07:27 UTC, end at Thu 2021-05-06 20:02:55 UTC. --
			*/
			if strings.EqualFold(outputLine, "-- No entries --") {
				continue
			}
			if strings.Contains(strings.ToLower(outputLine), "end at") { // Logs begin at also contain end at
				continue
			}
			totalLines++
			if !strings.Contains(strings.ToLower(outputLine), "stress-ng") {
				misses++
			}
		}
		if misses != 0 {
			result.Flakef("From Nodelogs: expected oom inducing pod to be oomkilled but other pod oom killed "+
				"%v of times, %v, %v", float64(misses/totalLines), misses, totalLines)
		}
	})
})

var _ = g.Describe("[sig-node][Serial]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("oom-kills-scp")
	oc := exutil.NewCLIWithFramework(f)

	// The goal of this test to verify if the OOM inducing pod is the one that is getting OOM Killed
	g.It("should oom kill the pod with system critical priority class which induced it", func() {
		g.By("creating the OOM inducing pod")
		podClient := f.PodClient()
		ns := f.Namespace.Name
		podName := naming.GetPodName("oom-inducing-pod-critical-priority", string(uuid.NewUUID()))
		// Collect the events to see if we're getting OOM Killed.
		kubeClient := oc.AdminKubeClient()

		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
		framework.ExpectNoError(err)
		// Node where memory inducing pod would land. Hardcoding this to pass correct memory configurations to exercise
		// eviction and OOMKill scenarios
		memLimit := getMemoryLimitOntheNode(nodes.Items[1])
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:    containerName,
					Image:   image.ShellImage(),
					Command: []string{"stress-ng"},
					Args:    []string{"--vm", "2", "--vm-bytes", memLimit},
				},
				},
				PriorityClassName: "system-cluster-critical",
			},
		}
		pod = podClient.Create(pod)
		err = e2epod.WaitForPodNameRunningInNamespace(oc.KubeFramework().ClientSet, pod.Name, oc.Namespace())
		framework.ExpectNoError(err)
		defer func() {
			g.By("deleting the pod")
			podClient.Delete(context.TODO(), pod.Name, *metav1.NewDeleteOptions(0))
		}()

		pod, err = podClient.Get(context.TODO(), pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pod running on node %s", pod.Spec.NodeName)

		// To wait for pod to start hogging memory
		time.Sleep(2 * time.Minute)

		testDuration := fmt.Sprintf("%v", exutil.DurationSinceStartInSeconds().Seconds()) + "s"
		out, err := oc.Run("adm", "node-logs").Args(pod.Spec.NodeName, "--path=journal",
			"--case-sensitive=false", "--grep=Memory cgroup out of memory", "--since=-"+testDuration).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		outputLines := strings.Split(out, "\n")
		totalLines := 0
		misses := 0
		for _, outputLine := range outputLines {
			/* Output Format
			 Logs begin at Thu 2021-05-06 15:07:27 UTC, end at Thu 2021-05-06 20:02:55 UTC. --
			May  6 16:02:55.490: INFO: -- Logs begin at thu 2021-05-06 15:07:27 utc, end at thu 2021-05-06 20:02:55 utc. --
			May  6 16:02:55.490: INFO: -- No entries --
			May  6 16:02:55.490: INFO: -- Logs begin at Thu 2021-05-06 15:07:27 UTC, end at Thu 2021-05-06 20:02:55 UTC. --
			*/
			if strings.EqualFold(outputLine, "-- No entries --") {
				continue
			}
			if strings.Contains(strings.ToLower(outputLine), "end at") { // Logs begin at also contain end at
				continue
			}
			totalLines++
			if !strings.Contains(strings.ToLower(outputLine), "stress-ng") {
				misses++
			}
		}
		if misses != 0 {
			result.Flakef("From Nodelogs: expected oom inducing pod to be oomkilled but other pod oom killed "+
				"%v of times, %v, %v", float64(misses/totalLines), misses, totalLines)
		}
	})
})

// getMemoryLimitOntheNode gets the memory limit on the node till which we can go to exercise eviction or OOMKill
func getMemoryLimitOntheNode(node v1.Node) string {
	memoryAvailable, ok := node.Status.Allocatable[corev1.ResourceMemory]
	o.Expect(ok).To(o.BeTrue())
	return strconv.Itoa(int(
		memoryAvailable.Value()))

}
