package operators

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"reflect"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Platform] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("olm", exutil.KubeConfigPath())

	// The list of all available OLM resources
	olmResources := []string{
		"packagemanifests", "catalogsources", "clusterserviceversions",
		"installplans", "operatorgroups", "subscriptions",
	}

	for i := range olmResources {
		g.It("list "+olmResources[i], func() {
			var resourceList map[string]interface{}
			// Get resource yaml and parse
			output, err := oc.AsAdmin().Run("get").Args(olmResources[i], "--all-namespaces", "-o", "yaml").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = yaml.Unmarshal([]byte(output), &resourceList)
			if err != nil {
				e2e.Logf("Unable to parse %s yaml list", olmResources[i])
			}
			// Verify resource items list has at least one item
			o.Expect(isResourceItemsEmpty(resourceList)).To(o.BeFalse(), olmResources[i]+" list should have at least one item")
			e2e.Logf("Successfully list %s", olmResources[i])
		})
	}

	// OCP-24074 Check OLM resources API Version
	// Author: jiazha@redhat.com
	versionMap := map[string]string{
		"operatorgroups":         "v1",
		"packagemanifests":       "v1",
		"catalogsources":         "v1alpha1",
		"subscriptions":          "v1alpha1",
		"installplans":           "v1alpha1",
		"clusterserviceversions": "v1alpha1",
	}

	for k, v := range versionMap {
		g.It("check API version: "+k, func() {
			output, err := oc.AsAdmin().Run("get").Args(k, "-n", "openshift-operator-lifecycle-manager", "-o=jsonpath={.items[0].apiVersion}").Output()
			if err != nil {
				e2e.Failf("Unable to check %s API, errors:%v", k, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.HasSuffix(output, v) {
				e2e.Logf("%s apiVersion is correct.", k)
			} else {
				e2e.Failf("%s apiVersion is incorrect!", k)
			}
		})
		g.It("check OLM's CRDs' version: "+k, func() {
			var output string
			var err error

			if k == "packagemanifests" {
				// packagemanifests is a resource created by Aggregation layer, not CRD.
				output, err = oc.AsAdmin().Run("get").Args("apiservice", "v1.packages.operators.coreos.com", "-o=jsonpath={.spec.version}").Output()
			} else {
				output, err = oc.AsAdmin().Run("get").Args("crd", fmt.Sprintf("%s.operators.coreos.com", k), "-o=jsonpath={.spec.version}").Output()
			}
			if err != nil {
				e2e.Failf("Unable to check %s CRD version, errors:%v", k, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.HasSuffix(output, v) {
				e2e.Logf("%s CRD version is correct.", k)
			} else {
				e2e.Failf("%s CRD version is incorrect!", k)
			}
		})
	}
})

func isResourceItemsEmpty(resourceList map[string]interface{}) bool {
	// Get resource items and check if it is empty
	items, err := resourceList["items"].([]interface{})
	o.Expect(err).To(o.BeTrue(), "Unable to verify items is a slice")

	if reflect.ValueOf(items).Len() > 0 {
		return false
	} else {
		return true
	}
}
