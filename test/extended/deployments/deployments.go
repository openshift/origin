package deployments

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("deployments: parallel: test deployment", func() {
	defer g.GinkgoRecover()
	var (
		deploymentFixture = exutil.FixturePath("..", "extended", "fixtures", "test-deployment-test.yaml")
		oc                = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())
	)

	g.Describe("test deployment", func() {
		g.It("should run a deployment to completion and then scale to zero", func() {
			out, err := oc.Run("create").Args("-f", deploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			out, err = oc.Run("logs").Args("-f", "dc/deployment-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deployment-test-1 to 2"))
			o.Expect(out).To(o.ContainSubstring("Pre hook finished"))
			o.Expect(out).To(o.ContainSubstring("Deployment deployment-test-1 successfully made active"))

			g.By("verifying the deployment is marked complete and scaled to zero")
			err = wait.Poll(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get("deployment-test-1")
				o.Expect(err).NotTo(o.HaveOccurred())
				status := rc.Annotations[deployapi.DeploymentStatusAnnotation]
				if deployapi.DeploymentStatus(status) != deployapi.DeploymentStatusComplete {
					return false, nil
				}
				if rc.Spec.Replicas != 0 {
					return false, nil
				}
				if rc.Status.Replicas != 0 {
					return false, nil
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that scaling does not result in new pods")
			out, err = oc.Run("scale").Args("dc/deployment-test", "--replicas=1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring no scale up of the deployment happens")
			wait.Poll(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get("deployment-test-1")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(rc.Spec.Replicas).Should(o.BeEquivalentTo(0))
				o.Expect(rc.Status.Replicas).Should(o.BeEquivalentTo(0))
				return false, nil
			})

			g.By("verifying the scale is updated on the deployment config")
			config, err := oc.REST().DeploymentConfigs(oc.Namespace()).Get("deployment-test")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(config.Spec.Replicas).Should(o.BeEquivalentTo(1))
			o.Expect(config.Spec.Test).Should(o.BeTrue())

			g.By("deploying a second time")
			out, err = oc.Run("deploy").Args("--latest", "deployment-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			out, err = oc.Run("logs").Args("-f", "dc/deployment-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deployment-test-2 up to 1"))
			o.Expect(out).To(o.ContainSubstring("Pre hook finished"))

			g.By("verifying the second deployment is marked complete and scaled to zero")
			err = wait.Poll(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get("deployment-test-2")
				o.Expect(err).NotTo(o.HaveOccurred())
				status := rc.Annotations[deployapi.DeploymentStatusAnnotation]
				if deployapi.DeploymentStatus(status) != deployapi.DeploymentStatusComplete {
					return false, nil
				}
				if rc.Spec.Replicas != 0 {
					return false, nil
				}
				if rc.Status.Replicas != 0 {
					return false, nil
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
