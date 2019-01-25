package operators

import (
	"reflect"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
	e2e "k8s.io/kubernetes/test/e2e/framework"

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
