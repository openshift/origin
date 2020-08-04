package builds

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	truePtr  = true
	falsePtr = false
)

func setBuildOverridesForcePull(forcePull *bool, oc *exutil.CLI) {
	buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	buildConfig.Spec.BuildDefaults = configv1.BuildDefaults{}
	buildConfig.Spec.BuildOverrides = configv1.BuildOverrides{ForcePull: forcePull}
	_, err = oc.AdminConfigClient().ConfigV1().Builds().Update(context.Background(), buildConfig, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

var _ = g.Describe("[sig-builds][Feature:Builds][Serial][Disruptive] buildoverrides forcepull should override equivalent option in build", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("buildoverrides")

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
			g.By("waiting for openshift/ruby:latest ImageStreamTag")
			err := exutil.WaitForAnImageStreamTag(oc, "openshift", "ruby", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application build configs")
			fptrueapps := exutil.FixturePath("testdata", "builds", "build-overrides", "forcepull-true.json")
			err = exutil.CreateResource(fptrueapps, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			fpfalseapps := exutil.FixturePath("testdata", "builds", "build-overrides", "forcepull-false.json")
			err = exutil.CreateResource(fpfalseapps, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			fpnilapps := exutil.FixturePath("testdata", "builds", "build-overrides", "forcepull-nil.json")
			err = exutil.CreateResource(fpnilapps, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			g.By("reset cluster build configuration")
			buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			buildConfig.Spec.BuildDefaults = configv1.BuildDefaults{}
			buildConfig.Spec.BuildOverrides = configv1.BuildOverrides{}
			_, err = oc.AdminConfigClient().ConfigV1().Builds().Update(context.Background(), buildConfig, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("Should override the builds forcePull option with true", func() {
			g.By("when BuildOverrides:forcePull is true")
			setBuildOverridesForcePull(&truePtr, oc)

		})

		g.It("Should override the builds forcePull option with false", func() {
			g.By("when BuildOverrides:forcePull is false")
			setBuildOverridesForcePull(&falsePtr, oc)

		})

		g.It("Should NOT override the builds forcePull option", func() {
			g.By("when BuildOverrides:forcePull is nil")

		})
	})
})
