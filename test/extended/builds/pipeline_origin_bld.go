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

// TODO: Remove this test if/when the JenkinsPipeline build strategy is removed.
// JenkinsPipeline builds have been deprecated since OCP 4.3.
var _ = g.Describe("[sig-builds][Feature:JenkinsRHELImagesOnly][Feature:Jenkins][Feature:Builds][sig-devex][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()

	var (
		oc               = exutil.NewCLIWithPodSecurityLevel("jenkins-pipeline", admissionapi.LevelBaseline)
		j                *jenkins.JenkinsRef
		simplePipelineBC = exutil.FixturePath("testdata", "builds", "simple-pipeline-bc.yaml")

		// jenkinsTemplate is an in-tree OpenShift Template to deploy Jenkins. It was initially derived from the openshift/jenkins template for release-4.14.
		// This template should be updated as needed for subsequent OpenShift releases.
		jenkinsTemplate = exutil.FixturePath("testdata", "builds", "jenkins-pipeline", "jenkins-ephemeral.json")

		// jenkinsImageStream is an in-tree ImageStream manifest that imports the official Red Hat Build of Jenkins image.
		// This was derived from the imagestream for release-4.14, and should be updated as needed for subsequent OpenShift releases.
		// Jenkins image tags align with the supported OCP version, though older versions can often run just fine on newer OCP for this test.
		jenkinsImageStream = exutil.FixturePath("testdata", "builds", "jenkins-pipeline", "jenkins-rhel.yaml")

		cleanup = func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}

		}
		setupJenkins = func() {
			exutil.PreTestDump()

			g.By("deploying Jenkins with OpenShift template")

			// Deploy Jenkins using the in-tree template. Parameters are tuned to increase the
			// default memory limit, disable admin monitors, and use the test namespace's Jenkins
			// imagestream.
			newAppArgs := []string{"--file",
				jenkinsTemplate,
				"-p",
				"MEMORY_LIMIT=2Gi",
				"-p",
				"DISABLE_ADMINISTRATIVE_MONITORS=true",
				"-p",
				fmt.Sprintf("NAMESPACE=%s", oc.Namespace()),
			}

			err := oc.Run("new-app").Args(newAppArgs...).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to deploy Jenkins with oc new-app %#v", newAppArgs)

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

			g.BeforeEach(func() {
				// Create the Jenkins imagestream in the test namespace. This ensures the test does
				// not depend on the Samples Operator.
				err := oc.Run("apply").Args("-f", jenkinsImageStream).Execute()
				o.Expect(err).NotTo(o.HaveOccurred(), "error creating the imagestream for Jenkins")
			})

			g.It("using a jenkins instance launched with the ephemeral template [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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
