package jobs

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exeutil "github.com/openshift/origin/test/extended/util"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[job][Conformance] openshift can execute jobs", func() {
	defer g.GinkgoRecover()
	oc := exeutil.NewCLI("job-controller", exeutil.KubeConfigPath())

	g.Describe("controller", func() {
		g.It("should create and run a job in user project", func() {
			for _, ver := range []string{"v1"} {
				oc.SetOutputDir(exeutil.TestContext.OutputDir)
				configPath := exeutil.FixturePath("testdata", "jobs", fmt.Sprintf("%s.yaml", ver))
				name := fmt.Sprintf("simple%s", ver)
				labels := fmt.Sprintf("app=%s", name)

				g.By(fmt.Sprintf("creating a job from %q...", configPath))
				err := oc.Run("create").Args("-f", configPath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a pod...")
				podNames, err := exeutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), exeutil.ParseLabelsOrDie(labels), exeutil.CheckPodIsSucceeded, 1, 3*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(podNames)).Should(o.Equal(1))

				g.By("waiting for a job...")
				err = exeutil.WaitForAJob(oc.KubeClient().BatchV1().Jobs(oc.Namespace()), name, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("checking job status...")
				jobs, err := oc.KubeClient().BatchV1().Jobs(oc.Namespace()).List(metav1.ListOptions{LabelSelector: exeutil.ParseLabelsOrDie(labels).String()})
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(len(jobs.Items)).Should(o.Equal(1))
				job := jobs.Items[0]
				o.Expect(len(job.Status.Conditions)).Should(o.Equal(1))
				o.Expect(job.Status.Conditions[0].Type).Should(o.Equal(batchv1.JobComplete))

				g.By("removing a job...")
				err = oc.Run("delete").Args(fmt.Sprintf("job/%s", name)).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})
})
