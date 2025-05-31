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
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
)

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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-community-operators Catalog", testCatalogAllEndpoint("openshift-community-operators"))
var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-certified-operators Catalog", testCatalogAllEndpoint("openshift-certified-operators"))
var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-redhat-marketplace Catalog", testCatalogAllEndpoint("openshift-redhat-marketplace"))
var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLM][Skipped:Disconnected] OLMv1 openshift-redhat-operators Catalog", testCatalogAllEndpoint("openshift-redhat-operators"))

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-community-operators Catalog", testCatalogMetasEndpoint("openshift-community-operators"))
var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-certified-operators Catalog", testCatalogMetasEndpoint("openshift-certified-operators"))
var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-redhat-marketplace Catalog", testCatalogMetasEndpoint("openshift-redhat-marketplace"))
var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMCatalogdAPIV1Metas][Skipped:Disconnected] OLMv1 openshift-redhat-operators Catalog", testCatalogMetasEndpoint("openshift-redhat-operators"))

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

func testCatalogAllEndpoint(catalog string) func() {
	return func() {
		defer g.GinkgoRecover()
		oc := exutil.NewCLIWithoutNamespace("default")

		g.It("should serve FBC via the /v1/api/all endpoint", func(ctx g.SpecContext) {
			checkFeatureCapability(oc)

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
	}
}

func testCatalogMetasEndpoint(catalog string) func() {
	return func() {
		defer g.GinkgoRecover()
		oc := exutil.NewCLIWithoutNamespace("default")

		g.It("should serve FBC via the /v1/api/metas endpoint", func(ctx g.SpecContext) {
			checkFeatureCapability(oc)

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

	var lastErr error
	err = wait.PollUntilContextTimeout(ctx, 15*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
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
