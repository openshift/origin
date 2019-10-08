package cli

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[cli]oc rsh[Conformance]", func() {
	defer g.GinkgoRecover()

	var (
		oc                     = exutil.NewCLI("oc-rsh", exutil.KubeConfigPath())
		multiContainersFixture = exutil.FixturePath("testdata", "cli", "pod-with-two-containers.yaml")
		podsLabel              = exutil.ParseLabelsOrDie("name=hello-centos")
	)

	g.Describe("rsh specific flags", func() {
		g.It("should work well when access to a remote shell", func() {
			namespace := oc.Namespace()
			g.By("Creating pods with multi containers")
			err := oc.Run("create").Args("-f", multiContainersFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to be running")
			pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(namespace), podsLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the rsh command without specify container name")
			out, err := oc.Run("rsh").Args(pods[0], "mkdir", "/tmp/test1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Defaulting container name to hello-centos"))

			g.By("running the rsh command with specify container name and shell")
			_, err = oc.Run("rsh").Args("--container=hello-centos-2", "--shell=/bin/sh", pods[0], "mkdir", "/tmp/test3").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
