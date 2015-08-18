// +build s2i

package extended

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("OpenShift Ruby S2I image", func() {
	defer GinkgoRecover()
	var (
		oc       = exutil.NewCLI("mysql-create", kubeConfigPath())
		versions = map[string]string{
			"20-centos7": "ruby 2.0.0",
			//"20-rhel7":   "ruby 2.0.0",
			//"22-centos7": "ruby 2.2.2",
			//"22-rhel7":   "ruby 2.2.2",
		}
	)

	for v, expected := range versions {

		Describe(fmt.Sprintf("SCL usage of ruby-%s", v), func() {
			It("should provide usage", func() {
				oc.SetOutputDir(testContext.OutputDir)
				name := exutil.MustGetImageName(oc.REST().ImageStreams("openshift"), "ruby", "latest")

				By("creating a sample pod for that executes the binary directly")
				pod := exutil.GetPodForContainer(kapi.Container{
					Name:  "test",
					Image: name,
				})
				oc.KubeFramework().TestContainerOutput("ruby-"+v, pod, 0, []string{"Sample invocation"})
			})
		})

		Describe(fmt.Sprintf("SCL usage of ruby-%s", v), func() {
			It("should allow various invocations of binary", func() {
				oc.SetOutputDir(testContext.OutputDir)
				name := exutil.MustGetImageName(oc.REST().ImageStreams("openshift"), "ruby", "latest")

				By("creating a sample pod for that executes the binary directly")
				pod := exutil.GetPodForContainer(kapi.Container{
					Image:   name,
					Name:    "test",
					Command: []string{"/bin/bash", "-c", "ruby --version"},
				})

				oc.KubeFramework().TestContainerOutput("ruby-"+v, pod, 0, []string{expected})

				By(fmt.Sprintf("creating a sample pod for %q", name))
				pod = exutil.GetPodForContainer(kapi.Container{
					Image:   name,
					Name:    "test",
					Command: []string{"/usr/bin/sleep", "infinity"},
				})
				_, err := oc.KubeREST().Pods(oc.Namespace()).Create(pod)
				Expect(err).NotTo(HaveOccurred())

				err = oc.KubeFramework().WaitForPodRunning(pod.Name)
				Expect(err).NotTo(HaveOccurred())

				By("calling the binary using 'oc exec /bin/bash -c ruby --version'")
				out, err := oc.Run("exec").Args("-p", pod.Name, "--", "/bin/bash", "-c", "ruby --version").Output()
				Expect(err).NotTo(HaveOccurred())
				Ω(out).Should(ContainSubstring(expected))

				By("calling the binary using 'oc exec /bin/sh -ic ruby --version'")
				out, err = oc.Run("exec").Args("-p", pod.Name, "--", "/bin/sh", "-ic", "ruby --version").Output()
				Expect(err).NotTo(HaveOccurred())
				Ω(out).Should(ContainSubstring(expected))
			})
		})
	}
})
