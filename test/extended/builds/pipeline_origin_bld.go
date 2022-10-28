package builds

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-builds][Feature:JenkinsRHELImagesOnly][Feature:Jenkins][Feature:Builds][sig-devex][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()

	var (
		oc               = exutil.NewCLIWithPodSecurityLevel("jenkins-pipeline", admissionapi.LevelBaseline)
		j                *jenkins.JenkinsRef
		simplePipelineBC = exutil.FixturePath("testdata", "builds", "simple-pipeline-bc.yaml")

		cleanup = func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}

		}
		setupJenkins = func() {
			exutil.PreTestDump()

			// our pipeline jobs, between jenkins and oc invocations, need more mem than the default
			newAppArgs := []string{"jenkins-ephemeral", "-p", "MEMORY_LIMIT=2Gi", "-p", "DISABLE_ADMINISTRATIVE_MONITORS=true"}

			g.By(fmt.Sprintf("calling oc new-app with %#v", newAppArgs))
			err := oc.Run("new-app").Args(newAppArgs...).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for jenkins deployment")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "jenkins", 1, false, oc)
			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			j = jenkins.NewRef(oc)

			g.By("wait for jenkins to come up")
			resp, err := j.WaitForContent("", 200, 5*time.Minute, "")

			if err != nil {
				e2e.Logf("wait for jenkins to come up got err and resp string %s and err %s, dumping pods", resp, err.Error())
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}

			o.Expect(err).NotTo(o.HaveOccurred())

			// Start capturing logs from this deployment config.
			// This command will terminate if the Jenkins instance crashes. This
			// ensures that even if the Jenkins DC restarts, we should capture
			// logs from the crash.
			_, _, _, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
			o.Expect(err).NotTo(o.HaveOccurred())

		}

		debugAnyJenkinsFailure = func(br *exutil.BuildResult, name string, oc *exutil.CLI, dumpMaster bool) {
			if !br.BuildSuccess {
				br.LogDumper = jenkins.DumpLogs
				fmt.Fprintf(g.GinkgoWriter, "\n\n START debugAnyJenkinsFailure\n\n")
				j := jenkins.NewRef(oc)
				jobLog, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "")
				if err == nil {
					fmt.Fprintf(g.GinkgoWriter, "\n %s job log:\n%s", name, jobLog)
				} else {
					fmt.Fprintf(g.GinkgoWriter, "\n error getting %s job log: %#v", name, err)
				}
				if dumpMaster {
					exutil.DumpApplicationPodLogs("jenkins", oc)
				}
				fmt.Fprintf(g.GinkgoWriter, "\n\n END debugAnyJenkinsFailure\n\n")
			}
		}
	)

	g.Context("", func() {

		g.Describe("jenkins pipeline build config strategy", func() {
			g.It("using a jenkins instance launched with the ephemeral template [apigroup:build.openshift.io]", func() {
				defer cleanup()
				setupJenkins()

				g.By("should build and complete successfully", func() {

					g.By("calling oc create -f to create buildconfig")
					err := oc.Run("create").Args("-f", simplePipelineBC).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("starting the pipeline build and waiting for it to complete")
					// this just does sh "mvn --version"
					br, err := exutil.StartBuildAndWait(oc, "minimalpipeline")
					if err != nil || !br.BuildSuccess {
						debugAnyJenkinsFailure(br, oc.Namespace(), oc, true)
						exutil.DumpBuilds(oc)
					}
					br.AssertSuccess()

					g.By("getting job log, make sure has success message")
					_, err = j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
					o.Expect(err).NotTo(o.HaveOccurred())
				})

			})

		})

	})
})
