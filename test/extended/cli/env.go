package cli

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc env", func() {
	defer g.GinkgoRecover()

	var (
		file            = exutil.FixturePath("testdata", "test-deployment.yaml")
		buildConfigFile = exutil.FixturePath("testdata", "test-buildcli.json")
		oc              = exutil.NewCLI("oc-env")
	)

	g.It("can set environment variables [apigroup:image.openshift.io][apigroup:build.openshift.io]", func() {
		g.By("creating a test-deployment deployment")
		err := oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.Run("delete").Args("-f", file).Execute()

		g.By("create a test-buildcli buildconfig")
		err = oc.Run("create").Args("-f", buildConfigFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.Run("delete").Args("-f", buildConfigFile).Execute()

		g.By("setting environment variables for deployment")
		deploymentName := "deployment/test-deployment"

		out, err := oc.Run("set").Args("env", deploymentName, "FOO=1st").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("env", deploymentName, "FOO=2nd").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("env", deploymentName, "FOO=bar", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("env", deploymentName, "FOO=zee", "--overwrite=false").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("already has a value"))

		out, err = oc.Run("set").Args("env", deploymentName, "--list").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("FOO=bar"))

		out, err = oc.Run("set").Args("env", deploymentName, "FOO-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		err = oc.Run("create").Args("secret", "generic", "mysecret", "--from-literal=foo.bar=secret").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("set").Args("env", "--from=secret/mysecret", "--prefix=PREFIX_", deploymentName, "FOO-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("env", deploymentName, "--list").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("PREFIX_FOO_BAR from secret mysecret, key foo.bar"))

		out, err = oc.Run("set").Args("env", deploymentName, "--list", "--resolve").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("PREFIX_FOO_BAR=secret"))

		err = oc.Run("delete").Args("secret", "mysecret").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("set").Args("env", deploymentName, "--list", "--resolve").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error retrieving reference for PREFIX_FOO_BAR"))

		g.By("setting environment variables for buildconfigs")
		out, err = oc.Run("set").Args("env", "bc", "--all", "FOO=bar").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("env", "bc", "--all", "--list").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("FOO=bar"))

		out, err = oc.Run("set").Args("env", "bc", "--all", "FOO-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))
	})
})
