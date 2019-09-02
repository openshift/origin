package marketplace

import (
	"fmt"
	"reflect"
	"strings"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

//create objects by yaml in the cluster
func createResources(oc *exutil.CLI, yamlfile string) error {
	yaml := fmt.Sprint(yamlfile)
	e2e.Logf("Start to create Resource: %s", yaml)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", yaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		e2e.Failf("Unable to create file:%s", yaml)
		return err
	}
	return nil
}

// delete objects in the cluster
func clearResources(oc *exutil.CLI, resourcetype string, name string, ns string) error {
	msg, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ns, resourcetype, name).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") {
			return nil
		}
		return err
	}
	return nil
}

//check the resource exist or not
func existResources(oc *exutil.CLI, resourcetype string, name string, ns string) (b bool, e error) {
	msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, resourcetype, name).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") {
			return false, nil
		}

		e2e.Failf("Can't get resource:%s", name)
		return false, err
	}
	return true, nil
}

func isResourceItemsEmpty(resourceList map[string]interface{}) bool {
	// Get resource items and check if it is empty
	items, err := resourceList["items"].([]interface{})
	o.Expect(err).To(o.BeTrue(), "Unable to verify items is a slice:%v", items)

	if reflect.ValueOf(items).Len() > 0 {
		return false
	} else {
		return true
	}
}

func getResourceByPath(oc *exutil.CLI, resourcetype string, name string, path string, ns string) (msg string, e error) {
	msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, resourcetype, name, path).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") {
			return msg, nil
		}

		e2e.Failf("Can't get resource:%s", name)
		return msg, err
	}
	return msg, nil
}