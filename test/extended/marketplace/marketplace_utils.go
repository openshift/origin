package marketplace

import (
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"

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

func waitForSource(oc *exutil.CLI, resourcetype string, name string, timeWait int, ns string) error {
	resourceWaitTime := time.Duration(timeWait) * time.Second
	err := wait.Poll(5*time.Second, resourceWaitTime, func() (bool, error) {
		output, err := oc.AsAdmin().Run("get").Args(resourcetype, name, "-o=jsonpath={.status.currentPhase.phase.message}", "-n", ns).Output()
		if err != nil {
			e2e.Failf("Failed to create %s", name)
			return false, err
		}
		if strings.Contains(output, "has been successfully reconciled") {
			return true, nil
		}
		return false, nil
	})
	return err
}
