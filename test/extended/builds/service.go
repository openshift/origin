package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Conformance] build can reference a cluster service", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("build-service", exutil.KubeConfigPath())
		testDockerfile = `
FROM centos:7
RUN cat /etc/resolv.conf
RUN curl -vvv hello-openshift:8080
`
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("with a build being created from new-build", func() {
			g.It("should be able to run a build that references a cluster service", func() {
				g.By("standing up a new hello world service")
				err := oc.Run("new-app").Args("docker.io/openshift/hello-openshift").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "hello-openshift", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "hello-openshift")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("calling oc new-build with a Dockerfile")
				err = oc.Run("new-build").Args("-D", "-").InputString(testDockerfile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build is in Complete phase")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "centos-1", nil, nil, nil)
				//debug for failures
				if err != nil {
					exutil.DumpBuildLogs("centos", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})
