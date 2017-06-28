package builds

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][pruning] prune builds based on settings in the buildconfig", func() {
	var (
		buildPruningBaseDir   = exutil.FixturePath("testdata", "build-pruning")
		isFixture             = filepath.Join(buildPruningBaseDir, "imagestream.yaml")
		successfulBuildConfig = filepath.Join(buildPruningBaseDir, "successful-build-config.yaml")
		failedBuildConfig     = filepath.Join(buildPruningBaseDir, "failed-build-config.yaml")
		erroredBuildConfig    = filepath.Join(buildPruningBaseDir, "errored-build-config.yaml")
		legacyBuildConfig     = filepath.Join(buildPruningBaseDir, "default-legacy-build-config.yaml")
		groupBuildConfig      = filepath.Join(buildPruningBaseDir, "default-group-build-config.yaml")
		oc                    = exutil.NewCLI("build-pruning", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for openshift namespace imagestreams")
		err = exutil.WaitForOpenShiftNamespaceImageStreams(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating test image stream")
		err = oc.Run("create").Args("-f", isFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	g.It("should prune completed builds based on the successfulBuildsHistoryLimit setting", func() {

		g.By("creating test successful build config")
		err := oc.Run("create").Args("-f", successfulBuildConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("starting three test builds")
		br, _ := exutil.StartBuildAndWait(oc, "myphp")
		br.AssertSuccess()
		br, _ = exutil.StartBuildAndWait(oc, "myphp")
		br.AssertSuccess()
		br, _ = exutil.StartBuildAndWait(oc, "myphp")
		br.AssertSuccess()

		buildConfig, err := oc.Client().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		builds, err := oc.Client().Builds(oc.Namespace()).List(metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		o.Expect(int32(len(builds.Items))).To(o.Equal(*buildConfig.Spec.SuccessfulBuildsHistoryLimit), "there should be %v completed builds left after pruning, but instead there were %v", *buildConfig.Spec.SuccessfulBuildsHistoryLimit, len(builds.Items))

	})

	g.It("should prune failed builds based on the failedBuildsHistoryLimit setting", func() {

		g.By("creating test failed build config")
		err := oc.Run("create").Args("-f", failedBuildConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("starting three test builds")
		br, _ := exutil.StartBuildAndWait(oc, "myphp")
		br.AssertFailure()
		br, _ = exutil.StartBuildAndWait(oc, "myphp")
		br.AssertFailure()
		br, _ = exutil.StartBuildAndWait(oc, "myphp")
		br.AssertFailure()

		buildConfig, err := oc.Client().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		builds, err := oc.Client().Builds(oc.Namespace()).List(metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		o.Expect(int32(len(builds.Items))).To(o.Equal(*buildConfig.Spec.FailedBuildsHistoryLimit), "there should be %v failed builds left after pruning, but instead there were %v", *buildConfig.Spec.FailedBuildsHistoryLimit, len(builds.Items))

	})

	g.It("should prune canceled builds based on the failedBuildsHistoryLimit setting", func() {

		g.By("creating test successful build config")
		err := oc.Run("create").Args("-f", failedBuildConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("starting and canceling three test builds")
		_, _, _ = exutil.StartBuild(oc, "myphp")
		err = oc.Run("cancel-build").Args("myphp-1").Execute()
		_, _, _ = exutil.StartBuild(oc, "myphp")
		err = oc.Run("cancel-build").Args("myphp-2").Execute()
		_, _, _ = exutil.StartBuild(oc, "myphp")
		err = oc.Run("cancel-build").Args("myphp-3").Execute()

		buildConfig, err := oc.Client().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		builds, err := oc.Client().Builds(oc.Namespace()).List(metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		o.Expect(int32(len(builds.Items))).To(o.Equal(*buildConfig.Spec.FailedBuildsHistoryLimit), "there should be %v canceled builds left after pruning, but instead there were %v", *buildConfig.Spec.FailedBuildsHistoryLimit, len(builds.Items))

	})

	g.It("should prune errored builds based on the failedBuildsHistoryLimit setting", func() {

		g.By("creating test failed build config")
		err := oc.Run("create").Args("-f", erroredBuildConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("starting three test builds")
		br, _ := exutil.StartBuildAndWait(oc, "myphp")
		br.AssertFailure()
		br, _ = exutil.StartBuildAndWait(oc, "myphp")
		br.AssertFailure()
		br, _ = exutil.StartBuildAndWait(oc, "myphp")
		br.AssertFailure()

		buildConfig, err := oc.Client().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		builds, err := oc.Client().Builds(oc.Namespace()).List(metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}

		o.Expect(int32(len(builds.Items))).To(o.Equal(*buildConfig.Spec.FailedBuildsHistoryLimit), "there should be %v failed builds left after pruning, but instead there were %v", *buildConfig.Spec.FailedBuildsHistoryLimit, len(builds.Items))

	})

	g.It("[Conformance] buildconfigs should have a default history limit set when created via the group api", func() {

		g.By("creating a build config with the group api")
		err := oc.Run("create").Args("-f", groupBuildConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		buildConfig, err := oc.Client().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
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

		buildConfig, err := oc.Client().BuildConfigs(oc.Namespace()).Get("myphp", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%v", err)
		}
		o.Expect(buildConfig.Spec.SuccessfulBuildsHistoryLimit).To(o.BeNil(), "the buildconfig should not have the default successful history limit set")
		o.Expect(buildConfig.Spec.FailedBuildsHistoryLimit).To(o.BeNil(), "the buildconfig should not have the default failed history limit set")
	})

})
