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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM] OLMv1 CRDs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

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
		checkFeatureCapability(oc)

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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-community-operators Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/all endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-community-operators"
		endpoint := "all"

		g.By(fmt.Sprintf("Testing api/v1/all endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s", baseURL, endpoint)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-certified-operators Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/all endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-certified-operators"
		endpoint := "all"

		g.By(fmt.Sprintf("Testing api/v1/all endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s", baseURL, endpoint)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-redhat-marketplace Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/all endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-redhat-marketplace"
		endpoint := "all"

		g.By(fmt.Sprintf("Testing api/v1/all endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s", baseURL, endpoint)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-redhat-operators Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/all endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-redhat-operators"
		endpoint := "all"

		g.By(fmt.Sprintf("Testing api/v1/all endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s", baseURL, endpoint)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-community-operators Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/metas endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-community-operators"
		endpoint := "metas"
		query := "schema=olm.package"

		g.By(fmt.Sprintf("Testing api/v1/metas endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s?%s", baseURL, endpoint, query)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-certified-operators Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/metas endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-certified-operators"
		endpoint := "metas"
		query := "schema=olm.package"

		g.By(fmt.Sprintf("Testing api/v1/metas endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s?%s", baseURL, endpoint, query)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-redhat-marketplace Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/metas endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-redhat-marketplace"
		endpoint := "metas"
		query := "schema=olm.package"

		g.By(fmt.Sprintf("Testing api/v1/metas endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s?%s", baseURL, endpoint, query)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-redhat-operators Catalog", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC via the /v1/api/metas endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		catalog := "openshift-redhat-operators"
		endpoint := "metas"
		query := "schema=olm.package"

		g.By(fmt.Sprintf("Testing api/v1/metas endpoint for catalog %q", catalog))
		baseURL, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clustercatalogs.olm.operatorframework.io",
			catalog,
			"-o=jsonpath={.status.urls.base}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalog))

		serviceURL := fmt.Sprintf("%s/api/v1/%s?%s", baseURL, endpoint, query)
		g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

		verifyAPIEndpoint(ctx, oc, serviceURL)
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
		checkFeatureCapability(oc)

		g.By("applying the necessary resources")
		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", catFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("checking that %q is not serving", catName))
		var lastReason string
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				b, err, s := waitForCatalogFailure(oc, catName)
				if lastReason != s {
					g.GinkgoLogr.Info(fmt.Sprintf("waitForCatalogFailure: %q", s))
					lastReason = s
				}
				return b, err
			})
		o.Expect(lastReason).To(o.BeEmpty())
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

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

func waitForCatalogFailure(oc *exutil.CLI, name string) (bool, error, string) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clustercatalogs.olm.operatorframework.io", name, "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err, ""
	}
	// no data yet, so try again
	if output == "" {
		return false, nil, "no output"
	}
	var conditions []metav1.Condition
	if err := json.Unmarshal([]byte(output), &conditions); err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err), ""
	}
	c := meta.FindStatusCondition(conditions, typeProgressing)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not pesent: %q", typeProgressing)
	}
	if c.Status != metav1.ConditionTrue {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
	}
	if c.Reason != reasonRetrying {
		return false, nil, fmt.Sprintf("expected reason to be %q: %+v", reasonRetrying, c)
	}
	if !strings.Contains(c.Message, "error creating image source") {
		return false, nil, fmt.Sprintf("expected message to contain %q: %+v", "error creating image source", c)
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

// verifyAPIEndpoint runs a job to validate the given service endpoint of a ClusterCatalog
func verifyAPIEndpoint(ctx g.SpecContext, oc *exutil.CLI, serviceURL string) {
	startTime := time.Now()

	jobName := fmt.Sprintf("test-catalog-endpoint-%s", rand.String(5))

	jobYAML := fmt.Sprintf(`
apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: api-tester
        image: registry.redhat.io/rhel8/httpd-24:latest
        resources:
          requests:
            cpu: "10m"
            memory: "50Mi"
        command:
        - /bin/bash
        - -c
        - |
          set -ex
          curl -v -k "%s" 
          if [ $? -ne 0 ]; then
            echo "Failed to access endpoint"
            exit 1
          fi
          echo "Successfully verified API endpoint"
          exit 0
      restartPolicy: Never
  backoffLimit: 2
`, jobName, "default", serviceURL)

	tempFile, err := os.CreateTemp("", "api-test-job-*.yaml")
	o.Expect(err).NotTo(o.HaveOccurred())
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	err = os.WriteFile(tempFile.Name(), []byte(jobYAML), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", tempFile.Name()).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("Creating the API endpoint verification job: %s at %v", jobName, startTime.Format(time.RFC3339)))

	// Wait for job completion
	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.5,
		Jitter:   0.1,
		Steps:    10,
		Cap:      1 * time.Minute,
	}

	var lastErr error
	err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"job", jobName, "-n", "default", "-o=jsonpath={.status}").Output()
		if err != nil {
			lastErr = err
			g.GinkgoLogr.Info(fmt.Sprintf("error getting job status: %v (will retry)", err))
			return false, nil
		}

		if output == "" {
			return false, nil // Job status not available yet
		}

		// Parse job status
		var status struct {
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
		}

		if err := json.Unmarshal([]byte(output), &status); err != nil {
			g.GinkgoLogr.Info(fmt.Sprintf("Error parsing job status: %v", err))
			return false, nil
		}

		if status.Succeeded > 0 {
			return true, nil
		}

		if status.Failed > 0 {
			return false, fmt.Errorf("job failed")
		}

		return false, nil
	})

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if err != nil {
		if lastErr != nil {
			g.GinkgoLogr.Error(nil, fmt.Sprintf("Last error encountered while polling: %v", lastErr))
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "Job failed or timed out in %v", duration)
	}
	g.GinkgoLogr.Info(fmt.Sprintf("Job completed successfully in: %v", duration))
}
