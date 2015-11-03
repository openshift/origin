package job

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exeutil "github.com/openshift/origin/test/extended/util"
	kapiextensions "k8s.io/kubernetes/pkg/apis/extensions"
)

var _ = g.Describe("Job", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exeutil.FixturePath("fixtures", "job-controller.yaml")
		oc         = exeutil.NewCLI("job-controller", exeutil.KubeConfigPath())
	)
	g.Describe("controller", func() {
		g.It("should create and run a job in user project", func() {
			oc.SetOutputDir(exeutil.TestContext.OutputDir)
			g.By(fmt.Sprintf("creating a job from %q", configPath))
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Waiting for pod..."))
			podNames, err := exeutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), exeutil.ParseLabelsOrDie("app=pi"), exeutil.CheckPodIsSucceededFunc, 1, 120*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(podNames)).Should(o.Equal(1))
			podName := podNames[0]

			g.By("retrieving logs from pod " + podName)
			logs, err := oc.Run("logs").Args(podName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(logs).Should(o.Equal("3.141592653589793238462643383279502884197169399375105820974944592307816406286208998628034825342117068"))

			g.By("checking job status")
			jobs, err := oc.KubeREST().Jobs(oc.Namespace()).List(exeutil.ParseLabelsOrDie("app=pi"), nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(len(jobs.Items)).Should(o.Equal(1))
			job := jobs.Items[0]
			o.Expect(len(job.Status.Conditions)).Should(o.Equal(1))
			o.Expect(job.Status.Conditions[0].Type).Should(o.Equal(kapiextensions.JobComplete))
		})
	})
})
