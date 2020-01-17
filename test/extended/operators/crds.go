package operators

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Platform] Managed cluster should", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("crd-checks")
	)
	defer g.GinkgoRecover()

	g.It("only have CRDs with schemas", func() {
		crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())

		crds, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var failing []string
		for _, crd := range crds.Items {
			for _, version := range crd.Spec.Versions {
				if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil || len(version.Schema.OpenAPIV3Schema.Properties) == 0 {
					failing = append(failing, fmt.Sprintf("CRD %s (%s.%s) version=%s has no schema: %#v", crd.Name, crd.Spec.Names.Plural, crd.Spec.Group, version.Name, version.Schema))
				}
			}
		}
		o.Expect(failing).To(o.BeEmpty(), "Some CRDs are not compliant")
	})
})
