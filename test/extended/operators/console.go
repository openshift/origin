package operators

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Console] Console operator should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("console-operator", exutil.KubeConfigPath())

	providedAPIs := []struct {
		fromAPIService bool
		group          string
		version        string
		plural         string
	}{
		{
			group:   "config.openshift.io",
			version: "v1",
			plural:  "consoles",
		},
	}

	for _, api := range providedAPIs {
		g.It(fmt.Sprintf("be installed with %s at version %s", api.plural, api.version), func() {
			// Ensure spec.version matches expected
			raw, err := oc.AsAdmin().Run("get").Args("apiservices", fmt.Sprintf("%s.%s", api.version, api.group), "-o=jsonpath='{.spec.version}'").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(raw).To(o.Equal(fmt.Sprintf("'%s'", api.version)))
		})

	}
})
