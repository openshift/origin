package builds

import (
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

func verifyStages(stages []buildapi.StageInfo, expectedStages map[string][]string) {
	for _, stage := range stages {
		expectedDurations, ok := expectedStages[string(stage.Name)]
		if !ok {
			o.ExpectWithOffset(1, ok).To(o.BeTrue(), "Unexpected stage %v was encountered", stage.Name)
		}
		if expectedDurations[0] != "" {
			expectedMinDuration, _ := time.ParseDuration(expectedDurations[0])
			o.ExpectWithOffset(1, (stage.DurationMilliseconds < expectedMinDuration.Nanoseconds()/int64(time.Millisecond))).To(o.BeFalse(), "Stage %v ran for %v, expected greater than %v", stage.Name, stage.DurationMilliseconds, expectedMinDuration)
		}
		expectedMaxDuration, _ := time.ParseDuration(expectedDurations[1])
		o.ExpectWithOffset(1, stage.DurationMilliseconds > expectedMaxDuration.Nanoseconds()/int64(time.Millisecond)).To(o.BeFalse(), "Stage %v ran for %v, expected less than %v", stage.Name, stage.DurationMilliseconds, expectedMaxDuration)
	}
}

var _ = g.Describe("[Feature:Builds][timing] capture build stages and durations", func() {
	var (
		buildTimingBaseDir    = exutil.FixturePath("testdata", "builds", "build-timing")
		isFixture             = filepath.Join(buildTimingBaseDir, "test-is.json")
		dockerBuildFixture    = filepath.Join(buildTimingBaseDir, "test-docker-build.json")
		dockerBuildDockerfile = filepath.Join(buildTimingBaseDir, "Dockerfile")
		sourceBuildFixture    = filepath.Join(buildTimingBaseDir, "test-s2i-build.json")
		sourceBuildBinDir     = filepath.Join(buildTimingBaseDir, "s2i-binary-dir")
		oc                    = exutil.NewCLI("build-timing", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should record build stages and durations for s2i", func() {

			expectedBuildStages := make(map[string][]string)
			expectedBuildStages["FetchInputs"] = []string{"", "1000s"}
			expectedBuildStages["CommitContainer"] = []string{"10ms", "1000s"}
			expectedBuildStages["Assemble"] = []string{"10ms", "1000s"}
			expectedBuildStages["PostCommit"] = []string{"", "1000s"}
			expectedBuildStages["PushImage"] = []string{"1s", "1000s"}

			g.By("creating test image stream")
			err := oc.Run("create").Args("-f", isFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test build config")
			err = oc.Run("create").Args("-f", sourceBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the test source build")
			br, _ := exutil.StartBuildAndWait(oc, "test", "--from-dir", sourceBuildBinDir)
			br.AssertSuccess()

			verifyStages(br.Build.Status.Stages, expectedBuildStages)
		})

		g.It("should record build stages and durations for docker", func() {

			expectedBuildStages := make(map[string][]string)
			expectedBuildStages["FetchInputs"] = []string{"", "1000s"}
			expectedBuildStages["CommitContainer"] = []string{"", "1000s"}
			expectedBuildStages["PullImages"] = []string{"", "1000s"}
			expectedBuildStages["Build"] = []string{"10ms", "1000s"}
			expectedBuildStages["PostCommit"] = []string{"", "1000s"}
			expectedBuildStages["PushImage"] = []string{"1s", "1000s"}

			g.By("creating test image stream")
			err := oc.Run("create").Args("-f", isFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test build config")
			err = oc.Run("create").Args("-f", dockerBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the test docker build")
			br, _ := exutil.StartBuildAndWait(oc, "test", "--from-file", dockerBuildDockerfile)
			br.AssertSuccess()

			verifyStages(br.Build.Status.Stages, expectedBuildStages)

		})
	})
})
