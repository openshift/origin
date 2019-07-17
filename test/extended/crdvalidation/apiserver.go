package crdvalidation

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/crdvalidation/apiserver] APIServer CR fields validation", func() {
	var (
		oc              = exutil.NewCLI("cluster-basic-auth", exutil.KubeConfigPath())
		apiServerClient = oc.AdminConfigClient().ConfigV1().APIServers()
	)
	defer g.GinkgoRecover()

	g.It("additionalCORSAllowedOrigins", func() {
		apiServer, err := apiServerClient.Get("cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		apiServer.Spec.AdditionalCORSAllowedOrigins = []string{"no closing (parentheses"}
		_, err = apiServerClient.Update(apiServer)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("additionalCORSAllowedOrigins"))
		o.Expect(err.Error()).To(o.ContainSubstring("not a valid regular expression"))
	})
})
