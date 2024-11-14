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

	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	olmv1GroupName                       = "olm.operatorframework.io"
	typeIncompatibelOperatorsUpgradeable = "InstalledOLMOperatorsUpgradeable"
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM] OLMv1 CRDs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		// Check for tech preview, if this is not tech preview, bail
		checkTestSkip(ctx, oc)

		// supports multiple versions during transision
		providedAPIs := []struct {
			group   string
			version []string
			plural  string
		}{
			{
				group:   olmv1GroupName,
				version: []string{"v1alpha1", "v1"},
				plural:  "clusterextensions",
			},
			{
				group:   olmv1GroupName,
				version: []string{"v1alpha1", "v1"},
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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM] OLMv1 Catalogs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		// Check for tech preview, if this is not tech preview, bail
		checkTestSkip(ctx, oc)

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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM] OLMv1 operator installation", func() {
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
		const (
			packageName = "quay-operator"
			version     = "3.13.0"
		)
		// Check for tech preview, if this is not tech preview, bail
		checkTestSkip(ctx, oc)

		ns := oc.Namespace()
		g.By(fmt.Sprintf("Updating the namespace to: %q", ns))
		newCeFile = ceFile + "." + ns
		b, err := os.ReadFile(ceFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		s := updateCe(b, ns, packageName, version)
		err = os.WriteFile(newCeFile, []byte(s), 0666)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("applying the necessary resources")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", newCeFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the ClusterExtention to be installed")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return WaitForClusterExtensionReady(oc, "install-test-ce")
			})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("ensuring the cluster is upgradeable when no olm.maxopenshiftversion is specified")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return WaitForCondition(oc, true)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should block cluster upgrades if an incompatible operator is installed", func(ctx g.SpecContext) {
		const (
			packageName = "elasticsearch-operator"
			version     = "5.8.13"
		)
		// Check for tech preview, if this is not tech preview, bail
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("Test only runs in tech-preview")
		}

		ns := oc.Namespace()
		g.By(fmt.Sprintf("Updating the namespace to: %q", ns))
		newCeFile = ceFile + "." + ns
		b, err := os.ReadFile(ceFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		s := updateCe(b, ns, packageName, version)
		err = os.WriteFile(newCeFile, []byte(s), 0666)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("applying the necessary resources")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", newCeFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the ClusterExtention to be installed")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return WaitForClusterExtensionReady(oc, "install-test-ce")
			})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("ensuring the cluster is upgradeable when no olm.maxopenshiftversion is specified")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				return WaitForCondition(oc, false)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func updateCe(b []byte, args ...string) string {
	s := string(b)
	s = strings.ReplaceAll(s, "{REPLACE}", args[0])
	s = strings.ReplaceAll(s, "{PACKAGENAME}", args[1])
	s = strings.ReplaceAll(s, "{VERSION}", args[2])
	return s
}

func WaitForClusterExtensionReady(oc *exutil.CLI, ceName string) (done bool, err error) {
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
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Progressing", metav1.ConditionFalse) {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Installed", metav1.ConditionTrue) {
		return false, nil
	}
	return true, nil
}

func WaitForCondition(oc *exutil.CLI, status bool) (done bool, err error) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterextensions.olm.operatorframework.io", "install-test-ce", "-o=jsonpath={.status.conditions}").Output()
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
	if !meta.IsStatusConditionPresentAndEqual(conditions, "Installed", metav1.ConditionTrue) {
		return false, nil
	}
	return true, nil
}

func checkTestSkip(ctx context.Context, oc *exutil.CLI) {
	if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
		g.Skip("Test only runs in tech-preview")
	}
	cap, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityOperatorLifecycleManagerV1)
	o.Expect(err).NotTo(o.HaveOccurred())
	if !cap {
		g.Skip("Test only runs with OLMv1 capability")
	}
}
