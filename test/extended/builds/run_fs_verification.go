package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-builds][Feature:Builds] verify /run filesystem contents", func() {
	defer g.GinkgoRecover()
	var (
		oc                                      = exutil.NewCLIWithPodSecurityLevel("verify-run-fs", admissionapi.LevelBaseline)
		testVerityRunFSWriteableBuildConfigYaml = fmt.Sprintf(`
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: verify-run-fs
spec:
  runPolicy: Serial
  source:
    type: Dockerfile
    dockerfile: |-
      FROM %s
      RUN chmod -R uga+rwx /run/secrets
      USER 1001
  strategy:
    dockerStrategy:
      env:
        - name: BUILD_LOGLEVEL
          value: "10"
      imageOptimizationPolicy: SkipLayers
    type: Docker
`, image.LimitedShellImage())
		testVerifyRunFSContentsBuildConfigYaml = fmt.Sprintf(`
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: verify-run-fs
spec:
  runPolicy: Serial
  source:
    type: Dockerfile
    dockerfile: |-
      FROM %s
      RUN ls -R /run/secrets
      USER 1001
  strategy:
    dockerStrategy:
      env:
        - name: BUILD_LOGLEVEL
          value: "10"
      imageOptimizationPolicy: SkipLayers
    type: Docker
`, image.LimitedShellImage())
		lsRSlashRun = `
/run/secrets:
rhsm

/run/secrets/rhsm:
ca

/run/secrets/rhsm/ca:
redhat-entitlement-authority.pem
redhat-uep.pem
`
		lsRSlashRunFIPS = `
/run/secrets:
system-fips
`
		lsRSlashRunOKD = `
/run/secrets:
`
		lsRSlashRunRhel7 = `
/run/secrets:
rhsm

/run/secrets/rhsm:
ca
logging.conf
rhsm.conf
syspurpose

/run/secrets/rhsm/ca:
redhat-uep.pem

/run/secrets/rhsm/syspurpose:
valid_fields.json
`
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("verify-run-fs", oc)
			}
		})

		g.Describe("do not have unexpected content", func() {
			g.It("using a simple Docker Strategy Build [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("calling oc create with yaml")
				err := oc.Run("create").Args("-f", "-").InputString(testVerifyRunFSContentsBuildConfigYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("start and wait for build")
				br, err := exutil.StartBuildAndWait(oc, "verify-run-fs")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				g.By("check build logs for ls -R /run/secrets")
				logs, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				hasRightListing := false
				if strings.Contains(logs, lsRSlashRun) ||
					strings.Contains(logs, lsRSlashRunFIPS) ||
					strings.Contains(logs, lsRSlashRunOKD) ||
					strings.Contains(logs, lsRSlashRunRhel7) {

					hasRightListing = true
				}
				o.Expect(hasRightListing).To(o.BeTrue())
			})
		})

		g.Describe("are writeable", func() {
			g.It("using a simple Docker Strategy Build [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("calling oc create with yaml")
				err := oc.Run("create").Args("-f", "-").InputString(testVerityRunFSWriteableBuildConfigYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("start and wait for build")
				br, err := exutil.StartBuildAndWait(oc, "verify-run-fs")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
			})
		})
	})
})
