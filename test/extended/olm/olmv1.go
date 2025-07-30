package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	olmv1GroupName                       = "olm.operatorframework.io"
	typeIncompatibleOperatorsUpgradeable = "InstalledOLMOperatorsUpgradeable"
	reasonIncompatibleOperatorsInstalled = "IncompatibleOperatorsInstalled"

	typeInstalled   = "Installed"
	typeProgressing = "Progressing"

	reasonRetrying = "Retrying"
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 operator installation", func() {
	defer g.GinkgoRecover()

	var (
		baseDir = exutil.FixturePath("testdata", "olmv1")
		ceFile  = filepath.Join(baseDir, "install-operator.yaml")
	)
	oc := exutil.NewCLI("openshift-operator-controller")

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should install a cluster extension", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		const (
			packageName = "quay-operator"
			version     = "3.13.0"
		)

		cleanup, unique := applyResourceFile(oc, packageName, version, "", ceFile)
		ceName := "install-test-ce-" + unique
		g.DeferCleanup(cleanup)

		g.By("waiting for the ClusterExtention to be installed")
		var lastReason string
		err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				b, err, s := waitForClusterExtensionReady(oc, ceName)
				if lastReason != s {
					g.GinkgoLogr.Info(fmt.Sprintf("waitForClusterExtensionReady: %q", s))
					lastReason = s
				}
				return b, err
			})
		o.Expect(lastReason).To(o.BeEmpty())
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should fail to install a non-existing cluster extension", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		const (
			packageName = "does-not-exist"
			version     = "99.99.99"
		)

		cleanup, unique := applyResourceFile(oc, packageName, version, "", ceFile)
		ceName := "install-test-ce-" + unique
		g.DeferCleanup(cleanup)

		g.By("waiting for the ClusterExtention to report failure")
		var lastReason string
		err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				b, err, s := waitForClusterExtensionFailure(oc, ceName)
				if lastReason != s {
					g.GinkgoLogr.Info(fmt.Sprintf("waitForClusterExtensionFailure: %q", s))
					lastReason = s
				}
				return b, err
			})
		o.Expect(lastReason).To(o.BeEmpty())
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should block cluster upgrades if an incompatible operator is installed", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		const (
			packageName = "cluster-logging"
			version     = "6.2.2"
		)

		cleanup, unique := applyResourceFile(oc, packageName, version, "", ceFile)
		ceName := "install-test-ce-" + unique
		g.DeferCleanup(cleanup)

		g.By("waiting for the ClusterExtention to be installed")
		var lastReason string
		err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				b, err, s := waitForClusterExtensionReady(oc, ceName)
				if lastReason != s {
					g.GinkgoLogr.Info(fmt.Sprintf("waitForClusterExtensionReady: %q", s))
					lastReason = s
				}
				return b, err
			})
		o.Expect(lastReason).To(o.BeEmpty())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("ensuring the cluster is not upgradeable when olm.maxopenshiftversion is specified")
		lastReason = ""
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				b, err, s := waitForUpgradableCondition(oc, false, ceName)
				if lastReason != s {
					g.GinkgoLogr.Info(fmt.Sprintf("waitForUpgradableCondition: %q", s))
					lastReason = s
				}
				return b, err
			})
		o.Expect(lastReason).To(o.BeEmpty())
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

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

func waitForClusterExtensionFailure(oc *exutil.CLI, ceName string) (bool, error, string) {
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
	if !strings.HasPrefix(c.Message, "no bundles found") {
		return false, nil, fmt.Sprintf("expected message to contain %q: %+v", "no bundles found", c)
	}
	c = meta.FindStatusCondition(conditions, typeInstalled)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", typeInstalled)
	}
	if c.Status != metav1.ConditionFalse {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionFalse, c)
	}
	return true, nil, ""
}

func waitForUpgradableCondition(oc *exutil.CLI, status bool, ceName string) (bool, error, string) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("olms.operator.openshift.io", "cluster", "-o=jsonpath={.status.conditions}").Output()
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
	c := meta.FindStatusCondition(conditions, typeIncompatibleOperatorsUpgradeable)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", typeIncompatibleOperatorsUpgradeable)
	}
	if status {
		if c.Status != metav1.ConditionTrue {
			return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
		}
		return true, nil, ""
	}
	if c.Status != metav1.ConditionFalse {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionFalse, c)
	}
	if c.Reason != reasonIncompatibleOperatorsInstalled {
		return false, nil, fmt.Sprintf("expected reason to be %q: %+v", reasonIncompatibleOperatorsInstalled, c)
	}
	// Message should include "bundle %q for ClusterExtension %q"
	if !strings.Contains(c.Message, ceName) {
		return false, nil, fmt.Sprintf("expected message to contain %q: %+v", ceName, c)
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
