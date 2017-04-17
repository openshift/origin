package builds

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[builds][pruning] prune builds based on settings in the buildconfig", func() {
	var (
		buildPruningBaseDir   = exutil.FixturePath("testdata", "build-pruning")
		isFixture             = filepath.Join(buildPruningBaseDir, "imagestream.yaml")
		successfulBuildConfig = filepath.Join(buildPruningBaseDir, "successful-build-config.yaml")
		failedBuildConfig     = filepath.Join(buildPruningBaseDir, "failed-build-config.yaml")
		oc                    = exutil.NewCLI("build-pruning", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("waiting for openshift namespace imagestreams")
		err = exutil.WaitForOpenShiftNamespaceImageStreams(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should prune completed builds based on the successfulBuildsHistoryLimit setting", func() {

		g.By("creating test image stream")
		err := oc.Run("create").Args("-f", isFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating test successful build config")
		err = oc.Run("create").Args("-f", successfulBuildConfig).Execute()
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

		g.By("creating test image stream")
		err := oc.Run("create").Args("-f", isFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating test successful build config")
		err = oc.Run("create").Args("-f", failedBuildConfig).Execute()
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
})
