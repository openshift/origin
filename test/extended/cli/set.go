package cli

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	appsv1 "github.com/openshift/api/apps/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute
const deploymentChangeTimeout = 30 * time.Second

var _ = g.Describe("[cli]oc set [Conformance]", func() {
	defer g.GinkgoRecover()

	var (
		oc                      = exutil.NewCLI("oc-set", exutil.KubeConfigPath())
		simpleDeploymentFixture = exutil.FixturePath("testdata", "deployments", "deployment-simple.yaml")
		appsLabel               = exutil.ParseLabelsOrDie("name=deployment-simple")
	)

	g.Describe("volume", func() {
		dcName := "deployment-simple"

		g.It("should works well with DC", func() {
			namespace := oc.Namespace()
			dc := exutil.ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			_, err = oc.Run("set", "volume").Args("dc/deployment-simple", "--add", "--type=emptyDir", "--mount-path=/opt1", "--name=v1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), dcName, 2, true, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), appsLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.Run("exec").Args(pods[0], "--", "grep", "opt1", "/proc/mounts").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.Run("set", "volume").Args("dc/deployment-simple", "--add", "--type=emptyDir", "--mount-path=/opt2", "--name=v1", "--overwrite").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), dcName, 3, true, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), appsLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.Run("exec").Args(pods[0], "--", "grep", "opt2", "/proc/mounts").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.Run("set", "volume").Args("dc/deployment-simple", "--remove", "--name=v1", "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), dcName, 4, true, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), appsLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.Run("exec").Args(pods[0], "--", "grep", "opt2", "/proc/mounts").Output()
			o.Expect(err).To(o.HaveOccurred())
		})
	})
})
