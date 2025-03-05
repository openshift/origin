package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 default Catalogs", func() {
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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 Catalogs /v1/api/all endpoint", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should serve FBC", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		g.By("Testing /api/v1/all endpoint for catalog openshift-community-operators")
		verifyAPIEndpoint(ctx, oc, oc.Namespace(), "openshift-community-operators", "all")
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 Catalogs /v1/api/metas endpoint", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")
	g.It(" should serve the /v1/api/metas API endpoint", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		g.By("Testing api/v1/metas endpoint for catalog openshift-community-operators")
		verifyAPIEndpoint(ctx, oc, oc.Namespace(), "openshift-community-operators", "metas?schema=olm.package")
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 Catalogs API Load Test /v1/api/all endpoint", func() {
	defer g.GinkgoRecover()
	testNamespace := "default"
	oc := exutil.NewCLIWithoutNamespace(testNamespace)

	var (
		baseDir = exutil.FixturePath("testdata", "olmv1")
		catFile = filepath.Join(baseDir, "catalog-server-load-test.yaml")
		jobName = "catalog-server-load-test-all-endpoint"
	)

	g.It("should handle concurrent load with acceptable performance", func(ctx g.SpecContext) {
		checkFeatureCapability(oc)

		// Parameters for the load test
		maxLatencyThresholdMs := 20000 // Maximum acceptable P99 latency in milliseconds
		successRateThreshold := 100.0  // Required success rate percentage

		g.By("Load testing /api/v1/%s endpoint for catalog openshift-community-operators")

		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", catFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the job to complete
		var lastErr error
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("job", jobName, "-o=jsonpath={.status}").Output()
			if err != nil {
				lastErr = err
				g.GinkgoLogr.Info(fmt.Sprintf("Error getting job status: %v", err))
				return false, nil // Continue polling
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
				// Get logs to see why it failed
				podsOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", fmt.Sprintf("job-name=%s", jobName), "-o=jsonpath={.items[0].metadata.name}").Output()
				if err != nil {
					g.GinkgoLogr.Error(nil, fmt.Sprintf("Error finding job pods: %v", err))
					return false, fmt.Errorf("load test job failed and couldn't retrieve logs")
				}

				if podsOutput != "" {
					logs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podsOutput).Output()
					if err == nil {
						g.GinkgoLogr.Error(nil, fmt.Sprintf("Load test job failed. Logs: %s", logs))
					}
				}
				return false, fmt.Errorf("load test job failed")
			}

			g.GinkgoLogr.Info(fmt.Sprintf("Job status: %s", output))
			return false, nil
		})

		if err != nil {
			if lastErr != nil {
				g.GinkgoLogr.Error(nil, fmt.Sprintf("Last error encountered while polling: %v", lastErr))
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "Load test failed or timed out")
		}

		// Get the logs from the job
		podsOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", fmt.Sprintf("job-name=%s", jobName), "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podsOutput).NotTo(o.BeEmpty())

		logsStr, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podsOutput).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.GinkgoLogr.Info(fmt.Sprintf("Load test logs:\n%s", logsStr))

		// Extract key metrics from the logs
		successRateMatch := regexp.MustCompile(`Success rate: (\d+\.\d+)%`).FindStringSubmatch(logsStr)
		latencyMatch := regexp.MustCompile(`99.000%\s+(\d+\.\d+)`).FindStringSubmatch(logsStr)
		requestsPerSecMatch := regexp.MustCompile(`Requests/sec:\s+(\d+\.\d+)`).FindStringSubmatch(logsStr)

		// Verify metrics meet expectations
		if len(successRateMatch) >= 2 {
			successRate, err := strconv.ParseFloat(successRateMatch[1], 64)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(successRate).To(o.BeNumerically(">=", successRateThreshold),
				fmt.Sprintf("Success rate (%.2f%%) below threshold (%.2f%%)", successRate, successRateThreshold))
			g.GinkgoLogr.Info(fmt.Sprintf("Success rate: %.2f%% (threshold: %.2f%%)", successRate, successRateThreshold))
		}

		if len(latencyMatch) >= 2 {
			p99Latency, err := strconv.ParseFloat(latencyMatch[1], 64)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Convert from microseconds to milliseconds
			p99LatencyMs := p99Latency / 1000.0
			o.Expect(p99LatencyMs).To(o.BeNumerically("<=", float64(maxLatencyThresholdMs)),
				fmt.Sprintf("P99 latency (%.2f ms) exceeds threshold (%d ms)", p99LatencyMs, maxLatencyThresholdMs))
			g.GinkgoLogr.Info(fmt.Sprintf("P99 latency: %.2f ms (threshold: %d ms)", p99LatencyMs, maxLatencyThresholdMs))
		}

		if len(requestsPerSecMatch) >= 2 {
			rps, err := strconv.ParseFloat(requestsPerSecMatch[1], 64)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.GinkgoLogr.Info(fmt.Sprintf("Requests per second: %.2f", rps))
		}

		g.By("Successfully completed load test with acceptable performance")
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

		cleanup, ceName := applyClusterExtension(oc, packageName, version, ceFile)
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

		cleanup, ceName := applyClusterExtension(oc, packageName, version, ceFile)
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
			packageName = "elasticsearch-operator"
			version     = "5.8.13"
		)

		cleanup, ceName := applyClusterExtension(oc, packageName, version, ceFile)
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

func applyClusterExtension(oc *exutil.CLI, packageName, version, ceFile string) (func(), string) {
	ns := oc.Namespace()
	g.By(fmt.Sprintf("updating the namespace to: %q", ns))
	ceName := "install-test-ce-" + packageName
	newCeFile := ceFile + "." + packageName
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
	return func() {
		g.By("cleaning the necessary resources")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", newCeFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}, ceName
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
func verifyAPIEndpoint(ctx g.SpecContext, oc *exutil.CLI, namespace, catalogName, endpoint string) {
	jobName := fmt.Sprintf("test-catalog-%s-%s-%s", catalogName, endpoint, rand.String(5))

	baseURL, err := oc.AsAdmin().Run("get").Args(
		"clustercatalogs.olm.operatorframework.io",
		catalogName,
		"-o=jsonpath={.status.urls.base}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(baseURL).NotTo(o.BeEmpty(), fmt.Sprintf("Base URL not found for catalog %s", catalogName))

	serviceURL := fmt.Sprintf("%s/api/v1/%s", baseURL, endpoint)
	g.GinkgoLogr.Info(fmt.Sprintf("Using service URL: %s", serviceURL))

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api-tester",
							Image: "registry.redhat.io/rhel8/httpd-24:latest",
							Command: []string{
								"/bin/bash",
								"-c",
								fmt.Sprintf(`
set -ex
response=$(curl -s -k "%s" || echo "ERROR: Failed to access endpoint")
if [[ "$response" == ERROR* ]]; then
  echo "$response"
  exit 1
fi
echo "Successfully verified API endpoint"
exit 0
`, serviceURL),
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	_, err = oc.AdminKubeClient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if job.Status.Succeeded > 0 {
			return true, nil
		}
		if job.Status.Failed > 0 {
			return false, fmt.Errorf("job failed")
		}

		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pods.Items).NotTo(o.BeEmpty())

	logs, err := oc.AdminKubeClient().CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).DoRaw(context.TODO())
	o.Expect(err).NotTo(o.HaveOccurred())
	g.GinkgoLogr.Info(fmt.Sprintf("Job logs for %s endpoint: %s", endpoint, string(logs)))
}
