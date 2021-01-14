package image_ecosystem

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/client/conditions"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
)

func isNonAMD(oc *exutil.CLI) bool {
	nonAMD := false
	allWorkerNodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker",
	})
	if err != nil {
		e2e.Logf("problem getting nodes for arch check: %s", err)
	}
	for _, node := range allWorkerNodes.Items {
		if node.Status.NodeInfo.Architecture != "amd64" {
			nonAMD = true
			break
		}
	}
	return nonAMD
}

// defineTest will create the gingko test.  This ensures the test
// is created with a local copy of all variables the test will need,
// since the test may not run immediately and may run in parallel with other
// tests, so sharing a variable reference is problematic.  (Sharing the oc client
// is ok for these tests).
func defineTest(name string, t tc, oc *exutil.CLI) {
	g.Describe("returning s2i usage when running the image", func() {
		g.It(fmt.Sprintf("%q should print the usage", t.DockerImageReference), func() {
			e2e.Logf("checking %s/%s for architecture compatibility", name, t.Version)
			if isNonAMD(oc) && !t.NonAMD {
				e2e.Logf("skipping %s/%s because non-amd64 architecture", name, t.Version)
				return
			}
			e2e.Logf("%s/%s passed architecture compatibility", name, t.Version)
			g.By(fmt.Sprintf("creating a sample pod for %q", t.DockerImageReference))
			pod := exutil.GetPodForContainer(kapiv1.Container{
				Name:  "test",
				Image: t.DockerImageReference,
			})
			_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the pod to be running")
			err = e2epod.WaitForPodRunningInNamespaceSlow(oc.KubeClient(), pod.Name, oc.Namespace())
			if err != nil {
				p, e := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pod.Name, metav1.GetOptions{})
				if e != nil {
					e2e.Logf("error %v getting pod", e)
				}
				e2e.Logf("error %v waiting for pod %v: ", err, p)
				o.Expect(err).To(o.Equal(conditions.ErrPodCompleted))
			}

			g.By("checking the log of the pod")
			err = wait.Poll(1*time.Second, 10*time.Second, func() (bool, error) {
				log, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).GetLogs(pod.Name, &kapiv1.PodLogOptions{}).DoRaw(context.Background())
				if err != nil {
					return false, err
				}
				e2e.Logf("got log %v from pod %v", string(log), pod.Name)
				if strings.Contains(string(log), "Sample invocation") {
					return true, nil
				}
				if strings.Contains(string(log), "oc new-app") {
					return true, nil
				}
				if strings.Contains(string(log), "OpenShift") {
					return true, nil
				}
				if strings.Contains(string(log), "Openshift") {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

		})
	})
	g.Describe("using the SCL in s2i images", func() {
		g.It(fmt.Sprintf("%q should be SCL enabled", t.DockerImageReference), func() {
			e2e.Logf("checking %s/%s for architecture compatibility", name, t.Version)
			if isNonAMD(oc) && !t.NonAMD {
				e2e.Logf("skipping %s/%s because non-amd64 architecture", name, t.Version)
				return
			}
			e2e.Logf("%s/%s passed architecture compatibility", name, t.Version)
			g.By(fmt.Sprintf("creating a sample pod for %q with /bin/bash -c command", t.DockerImageReference))
			pod := exutil.GetPodForContainer(kapiv1.Container{
				Image:   t.DockerImageReference,
				Name:    "test",
				Command: []string{"/bin/bash", "-c", t.Cmd},
			})

			_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = e2epod.WaitForPodRunningInNamespaceSlow(oc.KubeClient(), pod.Name, oc.Namespace())
			if err != nil {
				p, e := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pod.Name, metav1.GetOptions{})
				if e != nil {
					e2e.Logf("error %v getting pod", e)
				}
				e2e.Logf("error %v waiting for pod %v: ", err, p)
				o.Expect(err).To(o.Equal(conditions.ErrPodCompleted))
			}

			g.By("checking the log of the pod")
			err = wait.Poll(1*time.Second, 10*time.Second, func() (bool, error) {
				log, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).GetLogs(pod.Name, &kapiv1.PodLogOptions{}).DoRaw(context.Background())
				if err != nil {
					return false, err
				}
				e2e.Logf("got log %v from pod %v", string(log), pod.Name)
				if strings.Contains(string(log), t.Expected) {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("creating a sample pod for %q", t.DockerImageReference))
			pod = exutil.GetPodForContainer(kapiv1.Container{
				Image:   t.DockerImageReference,
				Name:    "test",
				Command: []string{"/usr/bin/sleep", "infinity"},
			})
			_, err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = e2epod.WaitForPodRunningInNamespaceSlow(oc.KubeClient(), pod.Name, oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("calling the binary using 'oc exec /bin/bash -c'")
			out, err := oc.Run("exec").Args(pod.Name, "--", "/bin/bash", "-c", t.Cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).Should(o.ContainSubstring(t.Expected))

			g.By("calling the binary using 'oc exec /bin/sh -ic'")
			out, err = oc.Run("exec").Args(pod.Name, "--", "/bin/sh", "-ic", t.Cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).Should(o.ContainSubstring(t.Expected))
		})
	})
}

var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("s2i-usage")

	g.Context("", func() {
		g.JustBeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		for name, tcs := range GetTestCaseForImages() {
			for _, t := range tcs {
				defineTest(name, t, oc)
			}
		}
	})
})
