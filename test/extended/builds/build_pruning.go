package builds

import (
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

// These build pruning tests create x builds and check that y-z of them are left after pruning.
// This variation in the number of builds that could be left is caused by the HandleBuildPruning
// function using cached information from the buildLister.
var _ = g.Describe("[Feature:Builds][pruning] prune builds based on settings in the buildconfig", func() {
	var (
		buildPruningBaseDir = exutil.FixturePath("testdata", "builds", "build-pruning")
		templatePath        = "https://raw.githubusercontent.com/sclorg/httpd-ex/master/openshift/templates/httpd.json"
		templateName        = "httpd-example"
		legacyBuildConfig   = filepath.Join(buildPruningBaseDir, "default-legacy-build-config.yaml")
		groupBuildConfig    = filepath.Join(buildPruningBaseDir, "default-group-build-config.yaml")
		oc                  = exutil.NewCLI("build-pruning", exutil.KubeConfigPath())
		pollingInterval     = time.Second
		timeout             = 2 * time.Minute
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for openshift namespace imagestreams")
			err = exutil.WaitForOpenShiftNamespaceImageStreams(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("successful, failed, canceled, and errored builds should be pruned based on settings in the build config", func() {

			template := fmt.Sprintf("template/%s", templateName)
			buildConfig := fmt.Sprintf("bc/%s", templateName)

			g.By("creating template for test")
			err := oc.Run("create").Args("-n", oc.Namespace(), "-f", templatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("patching the template to create Parallel builds and include successfulBuildsHistoryLimit and failedBuildsHistoryLimit")
			t, err := oc.TemplateClient().Template().Templates(oc.Namespace()).Get(templateName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			for i := range t.Objects {
				oc.Run("patch").Args(template, "--type", "json", "-p", fmt.Sprintf("[{\"op\": \"test\", \"path\": \"/objects/%d/kind\", \"value\":\"BuildConfig\"},{\"op\": \"add\", \"path\":\"/objects/%d/spec/runPolicy\", \"value\":\"Parallel\"}]", i, i)).Output()
				oc.Run("patch").Args(template, "--type", "json", "-p", fmt.Sprintf("[{\"op\": \"test\", \"path\": \"/objects/%d/kind\", \"value\":\"BuildConfig\"},{\"op\": \"add\", \"path\":\"/objects/%d/spec/successfulBuildsHistoryLimit\", \"value\":\"2\"}]", i, i)).Output()
				oc.Run("patch").Args(template, "--type", "json", "-p", fmt.Sprintf("[{\"op\": \"test\", \"path\": \"/objects/%d/kind\", \"value\":\"BuildConfig\"},{\"op\": \"add\", \"path\":\"/objects/%d/spec/failedBuildsHistoryLimit\", \"value\":\"3\"}]", i, i)).Output()
			}

			g.By("creating a new application from the template")
			err = oc.Run("new-app").Args("--template", templateName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting 3 additional successful builds")
			for i := 0; i < 3; i++ {
				_, _, _ = exutil.StartBuild(oc, templateName)
			}

			g.By("creating 2 canceled builds")
			for i := 5; i < 7; i++ {
				_, _, _ = exutil.StartBuild(oc, templateName)
				err = oc.Run("cancel-build").Args(fmt.Sprintf("httpd-example-%d", i)).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

			}
			g.By("patching the buildconfig to create failed builds")
			err = oc.Run("patch").Args(buildConfig, "-p", `{"spec":{"source":{"git":{"uri":"https://github.com/sclorg/non-working-repo.git"}}}}`).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating 2 failed builds")
			for i := 0; i < 2; i++ {
				_, _, _ = exutil.StartBuild(oc, templateName)
			}

			g.By("patching the build config to create errored builds")
			err = oc.Run("patch").Args(buildConfig, "-p", `{"spec":{"source":{"git":{"uri":"https://github.com/sclorg/httpd-ex.git"}}}}`).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("patch").Args(buildConfig, "--type", "json", "-p", "[{\"op\": \"add\", \"path\":\"/spec/strategy/sourceStrategy/env\", \"value\":[{\"name\":\"FIELDREF_ENV\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.nofield\"}}}]}]").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating 2 errored builds")
			for i := 0; i < 2; i++ {
				_, _, _ = exutil.StartBuild(oc, templateName)
			}

			bc, err := oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get(templateName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var builds *buildapi.BuildList

			g.By("waiting for all builds to complete")
			err = wait.PollImmediate(pollingInterval, timeout, func() (bool, error) {
				builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
					return false, err
				}
				for _, build := range builds.Items {
					if build.Status.Phase == buildapi.BuildPhaseNew || build.Status.Phase == buildapi.BuildPhasePending || build.Status.Phase == buildapi.BuildPhaseRunning {
						return false, nil
					}
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for pruning to complete")
			err = wait.PollImmediate(pollingInterval, timeout, func() (bool, error) {
				builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
					return false, err
				}
				if int32(len(builds.Items)) <= (*bc.Spec.SuccessfulBuildsHistoryLimit + *bc.Spec.FailedBuildsHistoryLimit + 2) {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the correct number of builds exist")
			builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			completedBuilds := 0
			for _, build := range builds.Items {
				e2e.Logf("Build Status: %s/%s", build.Name, build.Status.Phase)
				if build.Status.Phase == buildapi.BuildPhaseComplete {
					completedBuilds++
				}
			}
			passed := false
			if int32(completedBuilds) == *bc.Spec.SuccessfulBuildsHistoryLimit || int32(completedBuilds) == (*bc.Spec.SuccessfulBuildsHistoryLimit+1) {
				passed = true
			}
			o.Expect(passed).To(o.BeTrue(), "there should be %d-%d completed builds left after pruning, but instead there were %v", *bc.Spec.SuccessfulBuildsHistoryLimit, (*bc.Spec.SuccessfulBuildsHistoryLimit + 1), completedBuilds)

			passed = false
			if int32((len(builds.Items)-completedBuilds)) == *bc.Spec.FailedBuildsHistoryLimit || int32((len(builds.Items)-completedBuilds)) == (*bc.Spec.FailedBuildsHistoryLimit+1) {
				passed = true
			}
			o.Expect(passed).To(o.BeTrue(), "there should be %d-%d failed, errored, or canceled builds left after pruning, but instead there were %v", *bc.Spec.FailedBuildsHistoryLimit, (*bc.Spec.FailedBuildsHistoryLimit + 1), (len(builds.Items) - completedBuilds))

			g.By("patching the build config to to leave 1 successful and 1 failed build")
			err = oc.Run("patch").Args(buildConfig, "-p", `{"spec":{"failedBuildsHistoryLimit": 1}}`).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("patch").Args(buildConfig, "-p", `{"spec":{"successfulBuildsHistoryLimit": 1}}`).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			bc, err = oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get(templateName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for pruning to complete")
			err = wait.PollImmediate(pollingInterval, timeout, func() (bool, error) {
				builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
					return false, err
				}
				if int32(len(builds.Items)) <= (*bc.Spec.SuccessfulBuildsHistoryLimit + *bc.Spec.FailedBuildsHistoryLimit + 2) {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the correct number of builds exist")
			builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			completedBuilds = 0
			for _, build := range builds.Items {
				e2e.Logf("Build Status: %s/%s", build.Name, build.Status.Phase)
				if build.Status.Phase == buildapi.BuildPhaseComplete {
					completedBuilds++
				}
			}
			passed = false
			if int32(completedBuilds) == *bc.Spec.SuccessfulBuildsHistoryLimit || int32(completedBuilds) == (*bc.Spec.SuccessfulBuildsHistoryLimit+1) {
				passed = true
			}
			o.Expect(passed).To(o.BeTrue(), "there should be %d-%d completed builds left after pruning, but instead there were %v", *bc.Spec.SuccessfulBuildsHistoryLimit, (*bc.Spec.SuccessfulBuildsHistoryLimit + 1), completedBuilds)

			passed = false
			if int32((len(builds.Items)-completedBuilds)) == *bc.Spec.FailedBuildsHistoryLimit || int32((len(builds.Items)-completedBuilds)) == (*bc.Spec.FailedBuildsHistoryLimit+1) {
				passed = true
			}
			o.Expect(passed).To(o.BeTrue(), "there should be %d-%d failed, errored, or canceled builds left after pruning, but instead there were %v", *bc.Spec.FailedBuildsHistoryLimit, (*bc.Spec.FailedBuildsHistoryLimit + 1), (len(builds.Items) - completedBuilds))

		})

		g.It("[Conformance] buildconfigs should have a default history limit set when created via the group api", func() {

			g.By("creating a build config with the group api")
			err := oc.Run("create").Args("-f", groupBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			buildConfig, err := oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "%v", err)
			}
			o.Expect(buildConfig.Spec.SuccessfulBuildsHistoryLimit).NotTo(o.BeNil(), "the buildconfig should have the default successful history limit set")
			o.Expect(buildConfig.Spec.FailedBuildsHistoryLimit).NotTo(o.BeNil(), "the buildconfig should have the default failed history limit set")
			o.Expect(*buildConfig.Spec.SuccessfulBuildsHistoryLimit).To(o.Equal(buildapi.DefaultSuccessfulBuildsHistoryLimit), "the buildconfig should have the default successful history limit set")
			o.Expect(*buildConfig.Spec.FailedBuildsHistoryLimit).To(o.Equal(buildapi.DefaultFailedBuildsHistoryLimit), "the buildconfig should have the default failed history limit set")
		})

		g.It("[Conformance] buildconfigs should not have a default history limit set when created via the legacy api", func() {

			g.By("creating a build config with the legacy api")
			err := oc.Run("create").Args("-f", legacyBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			buildConfig, err := oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "%v", err)
			}
			o.Expect(buildConfig.Spec.SuccessfulBuildsHistoryLimit).To(o.BeNil(), "the buildconfig should not have the default successful history limit set")
			o.Expect(buildConfig.Spec.FailedBuildsHistoryLimit).To(o.BeNil(), "the buildconfig should not have the default failed history limit set")
		})
	})
})
