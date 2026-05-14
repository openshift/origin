package images

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageInfo] Image info", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("image-info", admissionapi.LevelBaseline)

	g.It("should display information about images [apigroup:image.openshift.io]", func() {
		ns = oc.Namespace()
		payloadImage, err := oc.AsAdmin().Run("get").Args("clusterversion", "version", "-o", "jsonpath={.status.desired.image}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get current payload image from clusterversion")
		payloadImage = strings.TrimSpace(payloadImage)
		o.Expect(payloadImage).NotTo(o.BeEmpty())

		var out string
		cleanup, regArgs, prepErr := exutil.PrepareImagePullSecretAndCABundle(oc)
		if cleanup != nil {
			defer cleanup()
		}

		if prepErr == nil {
			args := append([]string{"info", payloadImage}, regArgs...)
			out, err = oc.AsAdmin().Run("image").Args(args...).Output()
		} else {
			err = prepErr
		}
		if err != nil {
			ctx := context.Background()
			isHyperShift, hsErr := exutil.IsHypershift(ctx, oc.AdminConfigClient())
			o.Expect(hsErr).NotTo(o.HaveOccurred())
			if isHyperShift {
				g.Skip("Skipping on HyperShift: runner requires external outbound access to registry")
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "oc image info failed on a standard cluster")
		}
		o.Expect(out).To(o.ContainSubstring("Digest:"))
		o.Expect(out).To(o.MatchRegexp(`Name:\s+.*`))

		//Test the json output
		if prepErr == nil {
			argsJson := append([]string{"info", payloadImage, "-o", "json"}, regArgs...)
			outJson, errJson := oc.AsAdmin().Run("image").Args(argsJson...).Output()
			o.Expect(errJson).NotTo(o.HaveOccurred())
			o.Expect(outJson).To(o.ContainSubstring(`"digest":`))
			o.Expect(outJson).To(o.MatchRegexp(`"name":\s+.*`))
		}
	})
})
