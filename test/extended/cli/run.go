package cli

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc run", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("oc-run", admissionapi.LevelBaseline)

	g.It("can use --image flag correctly [apigroup:apps.openshift.io]", g.Label("Size:S"), func() {
		_, err := oc.Run("create").Args("deploymentconfig", "newdcforimage", "--image=validimagevalue").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.Run("run").Args("newdcforimage2", "--image=\"InvalidImageValue0192\"").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("error: Invalid image name \"\\\"InvalidImageValue0192\\\"\": invalid reference format"))
	})

	g.It("can use --image flag correctly for deployment", g.Label("Size:S"), func() {
		_, err := oc.Run("create").Args("deployment", "newdcforimage", "--image=validimagevalue").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.Run("run").Args("newdcforimage2", "--image=\"InvalidImageValue0192\"").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("error: Invalid image name \"\\\"InvalidImageValue0192\\\"\": invalid reference format"))
	})
})
