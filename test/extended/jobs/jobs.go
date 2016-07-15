package jobs

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exeutil "github.com/openshift/origin/test/extended/util"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
)

var _ = g.Describe("[job][Conformance] openshift can execute jobs", func() {
	defer g.GinkgoRecover()
	oc := exeutil.NewCLI("job-controller", exeutil.KubeConfigPath())

	g.Describe("controller", func() {
		g.It("should create and run a job in user project", func() {
			for _, ver := range []string{"v1beta1", "v1"} {
				oc.SetOutputDir(exeutil.TestContext.OutputDir)
				configPath := exeutil.FixturePath("testdata", "jobs", fmt.Sprintf("%s.yaml", ver))
				name := fmt.Sprintf("simple%s", ver)
				labels := fmt.Sprintf("app=%s", name)

				g.By(fmt.Sprintf("creating a job from %q...", configPath))
				err := oc.Run("create").Args("-f", configPath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a pod...")
				podNames, err := exeutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), exeutil.ParseLabelsOrDie(labels), exeutil.CheckPodIsSucceededFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(podNames)).Should(o.Equal(1))

				g.By("waiting for a job...")
				err = exeutil.WaitForAJob(oc.KubeREST().ExtensionsClient.Jobs(oc.Namespace()), name, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("checking job status...")
				jobs, err := oc.KubeREST().ExtensionsClient.Jobs(oc.Namespace()).List(kapi.ListOptions{LabelSelector: exeutil.ParseLabelsOrDie(labels)})
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(len(jobs.Items)).Should(o.Equal(1))
				job := jobs.Items[0]
				o.Expect(len(job.Status.Conditions)).Should(o.Equal(1))
				o.Expect(job.Status.Conditions[0].Type).Should(o.Equal(batch.JobComplete))

				g.By("removing a job...")
				err = oc.Run("delete").Args(fmt.Sprintf("job/%s", name)).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})
})
