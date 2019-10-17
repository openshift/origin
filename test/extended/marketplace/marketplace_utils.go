package marketplace

import (
	"fmt"
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
