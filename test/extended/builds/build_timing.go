package builds

import (
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

func verifyStages(stages []buildv1.StageInfo, expectedStages map[string][]string) {
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
		delete(expectedStages, string(stage.Name))
	}
	o.ExpectWithOffset(1, expectedStages).To(o.BeEmpty())
}

var _ = g.Describe("[sig-builds][Feature:Builds][timing] capture build stages and durations", func() {
	var (
		buildTimingBaseDir    = exutil.FixturePath("testdata", "builds", "build-timing")
		isFixture             = filepath.Join(buildTimingBaseDir, "test-is.json")
		dockerBuildFixture    = filepath.Join(buildTimingBaseDir, "test-docker-build.json")
		dockerBuildDockerfile = filepath.Join(buildTimingBaseDir, "Dockerfile")
		sourceBuildFixture    = filepath.Join(buildTimingBaseDir, "test-s2i-build.json")
		sourceBuildBinDir     = filepath.Join(buildTimingBaseDir, "s2i-binary-dir")
		oc                    = exutil.NewCLIWithPodSecurityLevel("build-timing", admissionapi.LevelBaseline)
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should record build stages and durations for s2i [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			expectedBuildStages := make(map[string][]string)
			expectedBuildStages["PullImages"] = []string{"", "1000s"}
			expectedBuildStages["Build"] = []string{"10ms", "1000s"}
			expectedBuildStages["PushImage"] = []string{"100ms", "1000s"}

			g.By("creating test image stream")
			err := oc.Run("create").Args("-f", isFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test build config")
			err = oc.Run("create").Args("-f", sourceBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the test source build")
			br, _ := exutil.StartBuildAndWait(oc, "test", "--from-dir", sourceBuildBinDir)
			br.AssertSuccess()
			// Bug 1716697 - ensure push spec doesn't include tag, only SHA
			o.Expect(br.Logs()).To(o.MatchRegexp(`pushed image-registry\.openshift-image-registry\.svc:5000/.*/test@sha256:`))

			verifyStages(br.Build.Status.Stages, expectedBuildStages)
		})

		g.It("should record build stages and durations for docker [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			expectedBuildStages := make(map[string][]string)
			expectedBuildStages["PullImages"] = []string{"", "1000s"}
			expectedBuildStages["Build"] = []string{"10ms", "1000s"}
			expectedBuildStages["PushImage"] = []string{"75ms", "1000s"}

			g.By("creating test image stream")
			err := oc.Run("create").Args("-f", isFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test build config")
			err = oc.Run("create").Args("-f", dockerBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the test docker build")
			br, _ := exutil.StartBuildAndWait(oc, "test", "--from-file", dockerBuildDockerfile)
			br.AssertSuccess()
			// Bug 1716697 - ensure push spec doesn't include tag, only SHA
			o.Expect(br.Logs()).To(o.MatchRegexp(`pushed image-registry\.openshift-image-registry\.svc:5000/.*/test@sha256:`))

			verifyStages(br.Build.Status.Stages, expectedBuildStages)

		})
	})
})
