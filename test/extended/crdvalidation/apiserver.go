package crdvalidation

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery] APIServer CR fields validation", func() {
	var (
		oc = exutil.NewCLI("cluster-basic-auth")
	)
	defer g.GinkgoRecover()

	g.It("additionalCORSAllowedOrigins [apigroup:config.openshift.io]", g.Label("Size:S"), func() {
		apiServerClient := oc.AdminConfigClient().ConfigV1().APIServers()

		apiServer, err := apiServerClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		apiServer.Spec.AdditionalCORSAllowedOrigins = []string{"no closing (parentheses"}
		_, err = apiServerClient.Update(context.Background(), apiServer, metav1.UpdateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("additionalCORSAllowedOrigins"))
		o.Expect(err.Error()).To(o.ContainSubstring("not a valid regular expression"))
	})
})
