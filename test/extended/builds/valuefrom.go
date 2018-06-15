package builds

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Conformance][valueFrom] process valueFrom in build strategy environment variables", func() {
	var (
		valueFromBaseDir               = exutil.FixturePath("testdata", "builds", "valuefrom")
		testImageStreamFixture         = filepath.Join(valueFromBaseDir, "test-is.json")
		secretFixture                  = filepath.Join(valueFromBaseDir, "test-secret.yaml")
		configmapFixture               = filepath.Join(valueFromBaseDir, "test-configmap.yaml")
		successfulSTIBuildValueFrom    = filepath.Join(valueFromBaseDir, "successful-sti-build-value-from-config.yaml")
		successfulDockerBuildValueFrom = filepath.Join(valueFromBaseDir, "successful-docker-build-value-from-config.yaml")
		failedSTIBuildValueFrom        = filepath.Join(valueFromBaseDir, "failed-sti-build-value-from-config.yaml")
		failedDockerBuildValueFrom     = filepath.Join(valueFromBaseDir, "failed-docker-build-value-from-config.yaml")
		oc                             = exutil.NewCLI("build-valuefrom", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for openshift namespace imagestreams")
			err = exutil.WaitForOpenShiftNamespaceImageStreams(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test image stream")
			err = oc.Run("create").Args("-f", testImageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test secret")
			err = oc.Run("create").Args("-f", secretFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test configmap")
			err = oc.Run("create").Args("-f", configmapFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

		})

		g.It("should successfully resolve valueFrom in s2i build environment variables", func() {

			g.By("creating test successful build config")
			err := oc.Run("create").Args("-f", successfulSTIBuildValueFrom).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting test build")
			br, err := exutil.StartBuildAndWait(oc, "mys2itest")
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			logs, _ := br.Logs()
			o.Expect(logs).To(o.ContainSubstring("FIELDREF_ENV=mys2itest-1"))
			o.Expect(logs).To(o.ContainSubstring("CONFIGMAPKEYREF_ENV=myvalue"))
			o.Expect(logs).To(o.ContainSubstring("SECRETKEYREF_ENV=developer"))
			o.Expect(logs).To(o.ContainSubstring("FIELDREF_CLONE_ENV=mys2itest-1"))
			o.Expect(logs).To(o.ContainSubstring("FIELDREF_CLONE_CLONE_ENV=mys2itest-1"))
			o.Expect(logs).To(o.ContainSubstring("UNAVAILABLE_ENV=$(SOME_OTHER_ENV"))
			o.Expect(logs).To(o.ContainSubstring("ESCAPED_ENV=$(MY_ESCAPED_VALUE)"))

		})

		g.It("should successfully resolve valueFrom in docker build environment variables", func() {

			g.By("creating test successful build config")
			err := oc.Run("create").Args("-f", successfulDockerBuildValueFrom).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting test build")
			br, err := exutil.StartBuildAndWait(oc, "mydockertest")
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			logs, _ := br.Logs()
			o.Expect(logs).To(o.ContainSubstring("\"FIELDREF_ENV\" \"mydockertest-1\""))
			o.Expect(logs).To(o.ContainSubstring("\"CONFIGMAPKEYREF_ENV\" \"myvalue\""))
			o.Expect(logs).To(o.ContainSubstring("\"SECRETKEYREF_ENV\" \"developer\""))
			o.Expect(logs).To(o.ContainSubstring("\"FIELDREF_CLONE_ENV\" \"mydockertest-1\""))
			o.Expect(logs).To(o.ContainSubstring("\"FIELDREF_CLONE_CLONE_ENV\" \"mydockertest-1\""))
			o.Expect(logs).To(o.ContainSubstring("\"UNAVAILABLE_ENV\" \"$(SOME_OTHER_ENV)\""))
			o.Expect(logs).To(o.ContainSubstring("\"ESCAPED_ENV\" \"$(MY_ESCAPED_VALUE)\""))

		})

		g.It("should fail resolving unresolvable valueFrom in sti build environment variable references", func() {

			g.By("creating test build config")
			err := oc.Run("create").Args("-f", failedSTIBuildValueFrom).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting test build")
			br, _ := exutil.StartBuildAndWait(oc, "mys2itest")
			br.AssertFailure()

			o.Expect(strings.Contains(string(br.Build.Status.Reason), "UnresolvableEnvironmentVariable")).To(o.BeTrue())
			o.Expect(strings.Contains(br.Build.Status.Message, "unsupported fieldPath: metadata.nofield")).To(o.BeTrue())
			o.Expect(strings.Contains(br.Build.Status.Message, "key nokey not found in config map myconfigmap")).To(o.BeTrue())
			o.Expect(strings.Contains(br.Build.Status.Message, "key nousername not found in secret mysecret")).To(o.BeTrue())

		})

		g.It("should fail resolving unresolvable valueFrom in docker build environment variable references", func() {

			g.By("creating test build config")
			err := oc.Run("create").Args("-f", failedDockerBuildValueFrom).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting test build")
			br, _ := exutil.StartBuildAndWait(oc, "mydockertest")
			br.AssertFailure()

			o.Expect(strings.Contains(string(br.Build.Status.Reason), "UnresolvableEnvironmentVariable")).To(o.BeTrue())
			o.Expect(strings.Contains(br.Build.Status.Message, "unsupported fieldPath: metadata.nofield")).To(o.BeTrue())
			o.Expect(strings.Contains(br.Build.Status.Message, "key nokey not found in config map myconfigmap")).To(o.BeTrue())
			o.Expect(strings.Contains(br.Build.Status.Message, "key nousername not found in secret mysecret")).To(o.BeTrue())

		})
	})
})
