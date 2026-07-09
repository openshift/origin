package ginkgo

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// testBucket represents a named group of tests with a specific parallelism level.
type testBucket struct {
	name        string
	tests       []*testCase
	parallelism int
}

// testBucketCreator splits tests into ordered buckets for execution.
// Different implementations can apply different bucketing strategies
// based on job characteristics (e.g., heavy jobs with FIPS + TechPreview).
type testBucketCreator interface {
	createBuckets(tests []*testCase, k8sTestNames map[string]bool, parallelism int) []testBucket
}

// isHeavyJob returns true if the job is both FIPS-enabled and TechPreview,
// which requires CPU-heavy test bucketing to avoid test failures from CPU pressure.
func isHeavyJob() bool {
	jobName := strings.ToLower(os.Getenv("JOB_NAME"))
	return strings.Contains(jobName, "fips") && strings.Contains(jobName, "techpreview")
}

// defaultBucketCreator implements the original test bucketing strategy.
// Tests are split into functional groups with standard parallelism levels.
type defaultBucketCreator struct{}

func (d *defaultBucketCreator) createBuckets(tests []*testCase, k8sTestNames map[string]bool, parallelism int) []testBucket {
	kubeTests, openshiftTests := splitTests(tests, func(t *testCase) bool {
		return k8sTestNames[t.name]
	})

	storageTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-storage]")
	})

	networkK8sTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-network]")
	})

	orderedNamespaceDeletionTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "OrderedNamespaceDeletion")
	})

	networkTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-network]")
	})

	netpolTests, networkK8sTests := splitTests(networkK8sTests, func(t *testCase) bool {
		return strings.Contains(t.name, "Netpol")
	})

	buildsTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-builds]")
	})

	mustGatherTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli] oc adm must-gather")
	})

	return []testBucket{
		{name: "Kubernetes", tests: kubeTests, parallelism: parallelism},
		{name: "Storage", tests: storageTests, parallelism: max(1, parallelism/2)},
		{name: "NetworkK8s", tests: networkK8sTests, parallelism: max(1, parallelism/2)},
		{name: "OrderedNamespaceDeletion", tests: orderedNamespaceDeletionTests, parallelism: 1},
		{name: "Network", tests: networkTests, parallelism: max(1, parallelism/2)},
		{name: "Netpol", tests: netpolTests, parallelism: 2},
		{name: "Builds", tests: buildsTests, parallelism: max(1, parallelism/2)},
		{name: "OpenShift", tests: openshiftTests, parallelism: parallelism},
		{name: "MustGather", tests: mustGatherTests, parallelism: parallelism},
	}
}

// heavyJobBucketCreator implements CPU-aware bucketing for heavy jobs
// (FIPS + TechPreview) that experience high CPU pressure.
// CPU-intensive test groups are separated into dedicated buckets with
// reduced parallelism to prevent API server / etcd overload.
type heavyJobBucketCreator struct{}

func (h *heavyJobBucketCreator) createBuckets(tests []*testCase, k8sTestNames map[string]bool, parallelism int) []testBucket {
	kubeTests, openshiftTests := splitTests(tests, func(t *testCase) bool {
		return k8sTestNames[t.name]
	})

	storageTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-storage]")
	})

	networkK8sTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-network]")
	})

	orderedNamespaceDeletionTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "OrderedNamespaceDeletion")
	})

	kubeCPUHeavyTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[DRA]") ||
			strings.Contains(t.name, "InPlace Resize") ||
			strings.Contains(t.name, "Probing container") || strings.Contains(t.name, "Probing restartable init container") ||
			strings.Contains(t.name, "Pod Generation") ||
			strings.Contains(t.name, "CustomResourcePublishOpenAPI") ||
			strings.Contains(t.name, "CustomResourceValidationRules") ||
			strings.Contains(t.name, "CustomResourceConversionWebhook") ||
			strings.Contains(t.name, "CustomResourceDefinition Watch") ||
			strings.Contains(t.name, "CustomResourceFieldSelectors") ||
			strings.Contains(t.name, "CRDValidationRatcheting") ||
			strings.Contains(t.name, "FieldValidation")
	})

	netpolTests, networkK8sTests := splitTests(networkK8sTests, func(t *testCase) bool {
		return strings.Contains(t.name, "Netpol")
	})

	buildsTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-builds]")
	})

	mustGatherTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli] oc adm must-gather")
	})

	// Tier 1: heaviest per-test CPU impact — infrastructure-intensive operations
	openshiftCPUHeavyTier1Tests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-olmv1]") ||
			(strings.Contains(t.name, "[sig-network-edge]") &&
				(strings.Contains(t.name, "Router") || strings.Contains(t.name, "GatewayAPI"))) ||
			strings.Contains(t.name, "cloud-provider-aws") ||
			strings.Contains(t.name, "[sig-network]")
	})

	// Tier 2: sustained load but less per-test impact
	openshiftCPUHeavyTier2Tests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[Feature:DeploymentConfig]") ||
			strings.Contains(t.name, "poddisruptionbudgets") ||
			strings.Contains(t.name, "[sig-node] [Jira:Node/Kubelet]") ||
			strings.Contains(t.name, "[sig-node] Probe configuration [OTP]") ||
			strings.Contains(t.name, "[sig-cli] Workloads ")
	})

	return []testBucket{
		{name: "Kubernetes", tests: kubeTests, parallelism: parallelism},
		{name: "KubeCPUHeavy", tests: kubeCPUHeavyTests, parallelism: max(1, parallelism/2)},
		{name: "Storage", tests: storageTests, parallelism: max(1, parallelism*2/3)},
		{name: "NetworkK8s", tests: networkK8sTests, parallelism: max(1, parallelism*2/3)},
		{name: "OrderedNamespaceDeletion", tests: orderedNamespaceDeletionTests, parallelism: 1},
		{name: "Netpol", tests: netpolTests, parallelism: 2},
		{name: "Builds", tests: buildsTests, parallelism: max(1, parallelism*2/3)},
		{name: "OpenShift", tests: openshiftTests, parallelism: parallelism},
		{name: "OpenshiftCPUHeavyTier1", tests: openshiftCPUHeavyTier1Tests, parallelism: max(1, parallelism/3)},
		{name: "OpenshiftCPUHeavyTier2", tests: openshiftCPUHeavyTier2Tests, parallelism: max(1, parallelism*2/3)},
		{name: "MustGather", tests: mustGatherTests, parallelism: parallelism},
	}
}

// selectBucketCreator returns the appropriate bucket creator based on job characteristics.
func selectBucketCreator() testBucketCreator {
	jobName := os.Getenv("JOB_NAME")
	if isHeavyJob() {
		logrus.Infof("Heavy job detected (JOB_NAME=%s) - using CPU-heavy bucketing strategy", jobName)
		return &heavyJobBucketCreator{}
	}
	logrus.Infof("Standard job (JOB_NAME=%s) - using default bucketing strategy", jobName)
	return &defaultBucketCreator{}
}
