package image_ecosystem

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/client/conditions"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	exutil "github.com/openshift/origin/test/extended/util"
)

// Some pods can take much longer to get ready due to volume attach/detach latency.
const slowPodStartTimeout = 15 * time.Minute

func skipArch(oc *exutil.CLI, arches []string) bool {
	allWorkerNodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: nodeLabelSelectorWorker,
	})
	if err != nil {
		e2e.Logf("problem getting nodes for arch check: %s", err)
	}
	for _, node := range allWorkerNodes.Items {
		for _, arch := range arches {
			if node.Status.NodeInfo.Architecture == arch {
				return false
			}
		}
	}
	return true
}

// defineTest will create the gingko test.  This ensures the test
// is created with a local copy of all variables the test will need,
// since the test may not run immediately and may run in parallel with other
// tests, so sharing a variable reference is problematic.  (Sharing the oc client
// is ok for these tests).
func defineTest(name string, t tc, oc *exutil.CLI) {
	g.Describe("returning s2i usage when running the image", func() {
		g.It(fmt.Sprintf("%q should print the usage", t.DockerImageReference), g.Label("Size:S"), func() {
			e2e.Logf("checking %s:%s for architecture compatibility", name, t.Tag)
			if skipArch(oc, t.Arches) {
				e2eskipper.Skipf("skipping %s:%s because not available on cluster architecture", name, t.Tag)
				return
			}
			e2e.Logf("%s:%s passed architecture compatibility", name, t.Tag)
			g.By(fmt.Sprintf("creating a sample pod for %q", t.DockerImageReference))
			container := kapiv1.Container{
				Name:  "test",
				Image: t.DockerImageReference,
			}

			// For .NET 9.0, explicitly run the usage script to work around Terminal Logger issues
			// See: https://developers.redhat.com/articles/2024/11/15/net-9-now-available-rhel-and-openshift
			if name == "dotnet" && strings.Contains(t.Tag, "9.0") {
				e2e.Logf("Setting explicit command for .NET 9.0 to run usage script")
				container.Command = []string{"/usr/libexec/s2i/usage"}
			}

			pod := exutil.GetPodForContainer(container)

			_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the pod to be running")
			err = e2epod.WaitForPodSuccessInNamespaceTimeout(context.TODO(), oc.KubeClient(), pod.Name, oc.Namespace(), slowPodStartTimeout)
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
		g.It(fmt.Sprintf("%q should be SCL enabled", t.DockerImageReference), g.Label("Size:M"), func() {
			e2e.Logf("checking %s:%s for architecture compatibility", name, t.Tag)
			if skipArch(oc, t.Arches) {
				e2eskipper.Skipf("skipping %s:%s because not available on cluster architecture", name, t.Tag)
				return
			}
			e2e.Logf("%s:%s passed architecture compatibility", name, t.Tag)
			g.By(fmt.Sprintf("creating a sample pod for %q with /bin/bash -c command", t.DockerImageReference))
			pod := exutil.GetPodForContainer(kapiv1.Container{
				Image:   t.DockerImageReference,
				Name:    "test",
				Command: []string{"/bin/bash", "-c", t.Cmd},
			})

			_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = e2epod.WaitForPodSuccessInNamespaceTimeout(context.TODO(), oc.KubeClient(), pod.Name, oc.Namespace(), slowPodStartTimeout)
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

			err = e2epod.WaitForPodRunningInNamespaceSlow(context.TODO(), oc.KubeClient(), pod.Name, oc.Namespace())
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
			if g.CurrentSpecReport().Failed() {
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
