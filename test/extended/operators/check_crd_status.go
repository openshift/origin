package operators

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// checkSubresourceStatus returns a list of names of CRDs that have a "status" in the CRD schema
// but no subresource.status defined.
// For now, it ignores the ones that are currently failing.
func checkSubresourceStatus(crdItemList []apiextensionsv1.CustomResourceDefinition) []string {

	// These CRDs, at the time this test was written, do not have a "status" in the CRD schema
	// and subresource.status.
	// These can be skipped for now but we don't want the number to increase.
	// These CRDs should be tidied up over time.
	//
	exceptionsList := sets.NewString(
		"networks.config.openshift.io",
		"networks.operator.openshift.io",
		"operatorpkis.network.operator.openshift.io",
		"profiles.tuned.openshift.io",
		"tuneds.tuned.openshift.io",
		"sharedconfigmaps.sharedresource.openshift.io", // https://bugzilla.redhat.com/show_bug.cgi?id=2088533
		"sharedsecrets.sharedresource.openshift.io",    // https://bugzilla.redhat.com/show_bug.cgi?id=2088533
	)

	failures := []string{}
	for _, crdItem := range crdItemList {

		// This test is interested only in CRDs that end with "openshift.io".
		if !strings.HasSuffix(crdItem.ObjectMeta.Name, "openshift.io") {
			continue
		}

		crdName := crdItem.ObjectMeta.Name

		// Skip CRDs in the exceptions list for now.
		if exceptionsList.Has(crdName) {
			continue
		}

		// Iterate through all versions of the CustomResourceDefinition Spec looking for one with
		// a schema status element,
		foundStatusInSchema := false
		var i int
		for i = 0; i < len(crdItem.Spec.Versions); i++ {
			if _, ok := crdItem.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"]; ok {
				foundStatusInSchema = true
				break
			}
		}

		if foundStatusInSchema {
			if !(crdItem.Spec.Versions[i].Subresources != nil && crdItem.Spec.Versions[i].Subresources.Status != nil) {
				failures = append(failures, fmt.Sprintf("CRD %s has a status in the schema but no subresource.status", crdName))
			}
		}
	}

	return failures
}

var _ = g.Describe("[sig-arch][Early]", func() {

	defer g.GinkgoRecover()

	var (
		crdItemList []apiextensionsv1.CustomResourceDefinition
	)

	oc := exutil.NewCLI("subresource-status-check")

	g.BeforeEach(func() {
		var err error
		crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
		crdItemList, err = getCRDItemList(*crdClient)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("CRDs for openshift.io", func() {
		g.It("should have subresource.status", g.Label("Size:S"), func() {
			failures := checkSubresourceStatus(crdItemList)
			if len(failures) > 0 {
				e2e.Fail(strings.Join(failures, "\n"))
			}
		})
	})
})

func getCRDItemList(crdClient apiextensionsclientset.Clientset) ([]apiextensionsv1.CustomResourceDefinition, error) {

	crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return crdList.Items, err
}
