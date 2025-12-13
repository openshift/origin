package cli

import (
	"os"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc probe", func() {
	defer g.GinkgoRecover()

	var (
		deploymentConfig = exutil.FixturePath("testdata", "test-deployment-config.yaml")
		oc               = exutil.NewCLIWithPodSecurityLevel("oc-probe", admissionapi.LevelBaseline)
	)

	g.It("can ensure the probe command is functioning as expected on pods", g.Label("Size:M"), func() {
		g.By("creating a hello-openshift pod")
		file, err := writeObjectToFile(newHelloPod())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("describe").Args("pod", "hello-openshift").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for expected failure conditions")
		out, err := oc.Run("set").Args("probe").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: one or more resources"))

		out, err = oc.Run("set").Args("probe", "-f", file).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: you must specify"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--get-url=https://127.0.0.1/path").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("port must be specified as part of a url"))

		g.By("checking for expected success conditions")
		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("livenessProbe: {}"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--initial-delay-seconds=10").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("livenessProbe:"))
		o.Expect(out).To(o.ContainSubstring("initialDelaySeconds: 10"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--", "echo", "test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("livenessProbe:"))
		o.Expect(out).To(o.ContainSubstring("exec:"))
		o.Expect(out).To(o.ContainSubstring("- echo"))
		o.Expect(out).To(o.ContainSubstring("- test"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--readiness", "--", "echo", "test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("readinessProbe:"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--open-tcp=3306").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("tcpSocket:"))
		o.Expect(out).To(o.ContainSubstring("port: 3306"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--open-tcp=port").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("port: port"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--get-url=https://127.0.0.1:8080/path").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("port: 8080"))
		o.Expect(out).To(o.ContainSubstring("path: /path"))
		o.Expect(out).To(o.ContainSubstring("scheme: HTTPS"))
		o.Expect(out).To(o.ContainSubstring("host: 127.0.0.1"))

		out, err = oc.Run("set").Args("probe", "-f", file, "--local", "-o", "yaml", "--liveness", "--get-url=http://127.0.0.1:8080/path").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("scheme: HTTP"))

		oc.Run("delete").Args("-f", file).Execute()
	})

	g.It("can ensure the probe command is functioning as expected on deploymentconfigs [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
		g.By("creating a test-deployment-config deploymentconfig")
		err := oc.Run("create").Args("-f", deploymentConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		dc := "dc/test-deployment-config"
		defer oc.Run("delete").Args("-f", deploymentConfig).Execute()

		g.By("checking for expected failure conditions")
		out, err := oc.Run("set").Args("probe", dc, "--liveness").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Required value: must specify a handler type"))

		g.By("checking for expected success conditions")
		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--open-tcp=8080").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--open-tcp=8080", "--v=1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("was not changed"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("livenessProbe:"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--initial-delay-seconds=10").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("initialDelaySeconds: 10"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--initial-delay-seconds=20").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("initialDelaySeconds: 20"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--failure-threshold=2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("initialDelaySeconds: 20"))
		o.Expect(out).To(o.ContainSubstring("failureThreshold: 2"))

		out, err = oc.Run("set").Args("probe", dc, "--readiness", "--success-threshold=4", "--", "echo", "test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("initialDelaySeconds: 20"))
		o.Expect(out).To(o.ContainSubstring("successThreshold: 4"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--period-seconds=5").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("periodSeconds: 5"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--timeout-seconds=6").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("timeoutSeconds: 6"))

		out, err = oc.Run("set").Args("probe", "dc", "--all", "--liveness", "--timeout-seconds=7").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("timeoutSeconds: 7"))

		out, err = oc.Run("set").Args("probe", dc, "--liveness", "--remove").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args(dc, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("livenessProbe:"))
	})
})
