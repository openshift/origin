package images

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry] Image --dry-run", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("image-dry-run", admissionapi.LevelBaseline)
	)

	g.It("should not delete resources [apigroup:image.openshift.io]", func() {
		g.By("preparing the image stream where the test image will be pushed")
		_, err := oc.Run("create").Args("imagestreamtag", "test:latest", "--from=tools:latest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "test", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("triggering delete operation of istag with --dry-run=server")
		err = oc.Run("delete").Args("istag/test:latest", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("obtaining the test:latest image name")
		_, err = oc.Run("get").Args("istag", "test:latest", "-o", "jsonpath={.image.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("triggering delete operation of imagestream with --dry-run=server")
		err = oc.Run("delete").Args("imagestream/test", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("obtaining the test imagestream")
		_, err = oc.Run("get").Args("imagestream", "test", "-o", "jsonpath={.image.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
