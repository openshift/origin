package operators

import (
	"encoding/json"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"os"
	"strings"
)

const (
	typeInstalled   = "Installed"
	typeProgressing = "Progressing"

	reasonRetrying = "Retrying"
)

// Use the supplied |unique| value if provided, otherwise generate a unique string. The unique string is returned.
// |unique| is used to combine common test elements and to avoid duplicate names, which can occur if, for instance,
// the packageName is used.
// If this is called multiple times, pass the unique value from the first invocation to subsequent invocations.
func applyResourceFile(oc *exutil.CLI, packageName, version, unique, ceFile string) (func(), string) {
	ns := oc.Namespace()
	if unique == "" {
		unique = rand.String(8)
	}
	g.By(fmt.Sprintf("updating the namespace to: %q", ns))
	newCeFile := ceFile + "." + unique
	b, err := os.ReadFile(ceFile)
	o.Expect(err).NotTo(o.HaveOccurred())
	s := string(b)
	s = strings.ReplaceAll(s, "{NAMESPACE}", ns)
	s = strings.ReplaceAll(s, "{PACKAGENAME}", packageName)
	s = strings.ReplaceAll(s, "{VERSION}", version)
	s = strings.ReplaceAll(s, "{UNIQUE}", unique)
	err = os.WriteFile(newCeFile, []byte(s), 0666)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("applying the necessary %q resources", unique))
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", newCeFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	return func() {
		g.By(fmt.Sprintf("cleaning the necessary %q resources", unique))
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", newCeFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}, unique
}

func waitForClusterExtensionReady(oc *exutil.CLI, ceName string) (bool, error, string) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterextensions.olm.operatorframework.io", ceName, "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err, ""
	}
	// no data yet, so try again
	if output == "" {
		return false, nil, "no output"
	}
	if err := json.Unmarshal([]byte(output), &conditions); err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err), ""
	}
	c := meta.FindStatusCondition(conditions, typeProgressing)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", typeProgressing)
	}
	if c.Status != metav1.ConditionTrue {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
	}
	c = meta.FindStatusCondition(conditions, typeInstalled)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", typeInstalled)
	}
	if c.Status != metav1.ConditionTrue {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
	}
	return true, nil, ""
}

func checkFeatureCapability(oc *exutil.CLI) {
	cap, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityOperatorLifecycleManagerV1)
	o.Expect(err).NotTo(o.HaveOccurred())
	if !cap {
		g.Skip("Test only runs with OperatorLifecycleManagerV1 capability")
	}
}
