package builds

import (
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][subscription-content] builds installing subscription content", func() {

	defer g.GinkgoRecover()

	var (
		oc               = exutil.NewCLIWithPodSecurityLevel("build-subscription-content", admissionapi.LevelBaseline)
		baseDir          = exutil.FixturePath("testdata", "builds", "subscription-content")
		secretTemplate   = filepath.Join(baseDir, "secret-template.txt")
		imageStream      = filepath.Join(baseDir, "build-imagestream.yaml")
		rhel7BuildConfig = filepath.Join(baseDir, "buildconfig-subscription-content-rhel7.yaml")
		rhel8BuildConfig = filepath.Join(baseDir, "buildconfig-subscription-content-rhel8.yaml")
		rhel9BuildConfig = filepath.Join(baseDir, "buildconfig-subscription-content-rhel9.yaml")
	)

	g.Context("[apigroup:build.openshift.io]", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func(ctx g.SpecContext) {
			g.By("copying entitlement keys to namespace")
			// The Insights Operator is responsible for retrieving the entitlement keys for the
			// cluster and syncing them to the openshift-config-managed namespace.
			// If this secret is not present, it means the cluster is not a Red Hat subscribed
			// cluster and is not eligible to include entitled RHEL content in builds.
			_, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config-managed").Get(ctx, "etc-pki-entitlement", metav1.GetOptions{})
			if kerrors.IsNotFound(err) {
				g.Skip("cluster entitlements not found")
			}
			// We should not expect an error other than "not found"
			o.Expect(err).NotTo(o.HaveOccurred(), "getting secret openshift-config-managed/etc-pki-entitlement")
			// Without the shared resoruces CSI driver, we must manually copy the entitlement keys
			// to the build namespace.

			// Run oc commands as per the openshift documentation
			stdOut, _, err := oc.AsAdmin().Run("get").Args("secret", "etc-pki-entitlement", "-n", "openshift-config-managed", "-o=go-template-file", "--template", secretTemplate).Outputs()
			o.Expect(err).NotTo(o.HaveOccurred(), "getting secret openshift-config-managed/etc-pki-entitlement")
			err = oc.Run("apply").Args("-f", "-").InputString(stdOut).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "creating secret etc-pki-entitlement")

			g.By("setting up build outputs")
			err = oc.Run("apply").Args("-f", imageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "creating build output imagestream")
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should succeed for RHEL 7 base images", g.Label("Size:L"), func() {
			err := oc.Run("apply").Args("-f", rhel7BuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "creating BuildConfig")
			br, _ := exutil.StartBuildAndWait(oc, "subscription-content-rhel7")
			br.AssertSuccess()
		})

		g.It("should succeed for RHEL 8 base images", g.Label("Size:L"), func() {
			err := oc.Run("apply").Args("-f", rhel8BuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "creating BuildConfig")
			br, _ := exutil.StartBuildAndWait(oc, "subscription-content-rhel8")
			br.AssertSuccess()
		})

		g.It("should succeed for RHEL 9 base images", g.Label("Size:L"), func() {
			err := oc.Run("apply").Args("-f", rhel9BuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "creating BuildConfig")
			br, _ := exutil.StartBuildAndWait(oc, "subscription-content-rhel9")
			br.AssertSuccess()
		})

	})

})
