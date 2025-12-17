package cli

import (
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc statefulset", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("oc-statefulset", admissionapi.LevelBaseline)

	g.It("creates and deletes statefulsets", g.Label("Size:M"), func() {
		g.By("creating a new service for the statefulset")

		frontendFile, err := writeObjectToFile(newFrontendService())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(frontendFile)

		statefulSetFile, err := writeObjectToFile(newBusyBoxStatefulSet())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(statefulSetFile)

		err = oc.Run("create").Args("-f", frontendFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a new statefulset")
		err = oc.Run("create").Args("-f", statefulSetFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for pods to be ready")
		label := exutil.ParseLabelsOrDie("app=testapp")
		_, err = exutil.WaitForPods(
			oc.KubeClient().CoreV1().Pods(oc.Namespace()),
			label,
			exutil.CheckPodIsReady,
			1,
			5*time.Minute,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("describing pods")
		out, err := oc.Run("describe").Args("statefulset", "testapp").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("app=testapp"))

		g.By("deleting statefulset")
		err = oc.Run("delete").Args("statefulset", "testapp").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deleting service for statefulset")
		err = oc.Run("delete").Args("service", "frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
