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
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	olmv1GroupName                       = "olm.operatorframework.io"
	typeIncompatibleOperatorsUpgradeable = "InstalledOLMOperatorsUpgradeable"
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM] OLMv1 CRDs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		checkFeatureCapability(ctx, oc)

		providedAPIs := []struct {
			group   string
			version []string
			plural  string
		}{
			{
				group:   olmv1GroupName,
				version: []string{"v1"},
				plural:  "clusterextensions",
			},
			{
				group:   olmv1GroupName,
				version: []string{"v1"},
				plural:  "clustercatalogs",
			},
		}

		for _, api := range providedAPIs {
			g.By(fmt.Sprintf("checking %s at version %s [apigroup:%s]", api.plural, api.version, api.group))
			// Ensure expected version exists in spec.versions and is both served and stored
			var err error
			var raw string
			for _, ver := range api.version {
				raw, err = oc.AsAdmin().Run("get").Args("crds", fmt.Sprintf("%s.%s", api.plural, api.group), fmt.Sprintf("-o=jsonpath={.spec.versions[?(@.name==%q)]}", ver)).Output()
				if err == nil {
					break
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(raw).To(o.MatchRegexp(`served.?:true`))
			o.Expect(raw).To(o.MatchRegexp(`storage.?:true`))
		}
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 Catalogs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		checkFeatureCapability(ctx, oc)

		providedCatalogs := []string{
			"openshift-certified-operators",
			"openshift-community-operators",
			"openshift-redhat-marketplace",
			"openshift-redhat-operators",
		}
		for _, cat := range providedCatalogs {
			g.By(fmt.Sprintf("checking that %q exists", cat))
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clustercatalogs.olm.operatorframework.io", cat, "-o=jsonpath={.status.conditions}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).NotTo(o.BeEmpty())

			g.By(fmt.Sprintf("checking that %q is serving", cat))
			var conditions []metav1.Condition
			err = json.Unmarshal([]byte(output), &conditions)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(meta.IsStatusConditionPresentAndEqual(conditions, "Serving", metav1.ConditionTrue)).To(o.BeTrue())
		}
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 New Catalog Install", func() {
	defer g.GinkgoRecover()

	var (
		baseDir = exutil.FixturePath("testdata", "olmv1")
		catFile = filepath.Join(baseDir, "install-catalog.yaml")
		catName = "bad-catalog"
	)

	oc := exutil.NewCLIWithoutNamespace("default")

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", catFile).Execute()
		os.Remove(catFile)
	})

	g.It("should fail to install if it has an invalid reference", func(ctx g.SpecContext) {
		checkFeatureCapability(ctx, oc)

		g.By("applying the necessary resources")
		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", catFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("checking that %q is not serving", catName))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return waitForCatalogFailure(oc, catName)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Serial][Skipped:Disconnected] OLMv1 operator installation", func() {
	defer g.GinkgoRecover()

	var (
		baseDir   = exutil.FixturePath("testdata", "olmv1")
		ceFile    = filepath.Join(baseDir, "install-operator.yaml")
		newCeFile string
	)
	oc := exutil.NewCLI("openshift-operator-controller")

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", newCeFile).Execute()
		os.Remove(newCeFile)
	})

	g.It("should install a cluster extension", func(ctx g.SpecContext) {
		checkFeatureCapability(ctx, oc)

		const (
			packageName = "quay-operator"
			version     = "3.13.0"
		)

		newCeFile = applyClusterExtension(oc, packageName, version, ceFile)

		g.By("waiting for the ClusterExtention to be installed")
		err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return waitForClusterExtensionReady(oc, "install-test-ce")
			})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("ensuring the cluster is upgradeable when no olm.maxopenshiftversion is specified")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return waitForUpgradableCondition(oc, true)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should fail to install a non-existing cluster extension", func(ctx g.SpecContext) {
		checkFeatureCapability(ctx, oc)

		const (
			packageName = "does-not-exist"
			version     = "99.99.99"
		)

		newCeFile = applyClusterExtension(oc, packageName, version, ceFile)

		g.By("waiting for the ClusterExtention to report failure")
		err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return waitForClusterExtensionFailure(oc, "install-test-ce")
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should block cluster upgrades if an incompatible operator is installed", func(ctx g.SpecContext) {
		g.Skip("This test is broken: need to verify OCP max version behavior")
		checkFeatureCapability(ctx, oc)

		const (
			packageName = "elasticsearch-operator"
			version     = "5.8.13"
		)

		newCeFile = applyClusterExtension(oc, packageName, version, ceFile)

		g.By("waiting for the ClusterExtention to be installed")
		err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return waitForClusterExtensionReady(oc, "install-test-ce")
			})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("ensuring the cluster is not upgradeable when olm.maxopenshiftversion is specified")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return waitForUpgradableCondition(oc, false)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func applyClusterExtension(oc *exutil.CLI, packageName, version, ceFile string) string {
	ns := oc.Namespace()
	g.By(fmt.Sprintf("updating the namespace to: %q", ns))
	newCeFile := ceFile + "." + ns
	b, err := os.ReadFile(ceFile)
	o.Expect(err).NotTo(o.HaveOccurred())
	s := string(b)
	s = strings.ReplaceAll(s, "{NAMESPACE}", ns)
	s = strings.ReplaceAll(s, "{PACKAGENAME}", packageName)
	s = strings.ReplaceAll(s, "{VERSION}", version)
	err = os.WriteFile(newCeFile, []byte(s), 0666)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("applying the necessary resources")
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", newCeFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	return newCeFile
}

func waitForClusterExtensionReady(oc *exutil.CLI, ceName string) (done bool, err error) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterextensions.olm.operatorframework.io", ceName, "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err
	}
	// no data yet, so try again
	if output == "" {
		return false, nil
	}
	err = json.Unmarshal([]byte(output), &conditions)
	if err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err)
	}
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Progressing", metav1.ConditionTrue) {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Installed", metav1.ConditionTrue) {
		return false, nil
	}
	return true, nil
}

func waitForClusterExtensionFailure(oc *exutil.CLI, ceName string) (done bool, err error) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterextensions.olm.operatorframework.io", ceName, "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err
	}
	// no data yet, so try again
	if output == "" {
		return false, nil
	}
	err = json.Unmarshal([]byte(output), &conditions)
	if err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err)
	}
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Progressing", metav1.ConditionTrue) {
		return false, nil
	}
	c := meta.FindStatusCondition(conditions, "Progressing")
	if c == nil {
		return false, fmt.Errorf("Progressing condtion should not be nil")
	}
	if !strings.HasPrefix(c.Message, "no bundles found") {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Installed", metav1.ConditionFalse) {
		return false, nil
	}
	return true, nil
}

func waitForUpgradableCondition(oc *exutil.CLI, status bool) (bool, error) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("olms.operator.openshift.io", "cluster", "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err
	}
	// no data yet, so try again
	if output == "" {
		return false, nil
	}
	err = json.Unmarshal([]byte(output), &conditions)
	if err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err)
	}
	if status {
		return meta.IsStatusConditionTrue(conditions, typeIncompatibleOperatorsUpgradeable), nil
	}
	return meta.IsStatusConditionFalse(conditions, typeIncompatibleOperatorsUpgradeable), nil
}

func waitForCatalogFailure(oc *exutil.CLI, name string) (bool, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clustercatalogs.olm.operatorframework.io", name, "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err
	}
	// no data yet, so try again
	if output == "" {
		return false, nil
	}
	var conditions []metav1.Condition
	err = json.Unmarshal([]byte(output), &conditions)
	if err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err)
	}
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Progressing", metav1.ConditionTrue) {
		return false, nil
	}
	c := meta.FindStatusCondition(conditions, "Progressing")
	if c == nil {
		return false, fmt.Errorf("Progressing condtion should not be nil")
	}
	if c.Reason != "Retrying" {
		return false, nil
	}
	if !strings.Contains(c.Message, "error creating image source") {
		return false, nil
	}
	return true, nil
}

func checkFeatureCapability(ctx context.Context, oc *exutil.CLI) {
	// Hardcoded until openshift/api is updated:
	// import (	configv1 "github.com/openshift/api/config/v1" )
	// configv1.ClusterVersionCapabilityOperatorLifecycleManagerV1
	cap, err := exutil.IsCapabilityEnabled(oc, "OperatorLifecycleManagerV1")
	o.Expect(err).NotTo(o.HaveOccurred())
	if !cap {
		g.Skip("Test only runs with OperatorLifecycleManagerV1 capability")
	}
}
