package csi

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

// OpenShiftCSIDriverConfig holds definition test parameters of OpenShift specific CSI test
type OpenShiftCSIDriverConfig struct {
	// Name of the CSI driver.
	Driver string
	// Configuration of the LUN stress test. If nil, the test is skipped.
	LUNStressTest *LUNStressTestConfig
}

// Definition of the LUN stress test parameters.
// The test runs PodsTotal Pods with a single volume each, targeting the same node.
// The pods runs MaxPodsPerNode pods in parallel, or the driver attach limit, whatever is lower.
// The test ensures that a CSI driver can withstand larger nr. of Pod running at the same time
// and also that the larger nr. of volumes attached in sequence does not cause any issues.
// The test has 40 minutes limit by default, it can be higher by openshift-test run --timeout
// parameter.
type LUNStressTestConfig struct {
	// How many pods run in parallel. If this number is higher than the number of volumes the driver
	// can attach to a single node (reported as result of NodeGetInfo), the test will use the
	// attach limit instead.
	MaxPodsPerNode int
	// How many Pods with one volume each to run in total. Set to 0 to disable the test.
	PodsTotal int
}

// runtime.DecodeInto needs a runtime.Object but doesn't do any
// deserialization of it and therefore none of the methods below need
// an implementation.
var _ runtime.Object = &OpenShiftCSIDriverConfig{}

func (d *OpenShiftCSIDriverConfig) DeepCopyObject() runtime.Object {
	return nil
}

func (d *OpenShiftCSIDriverConfig) GetObjectKind() schema.ObjectKind {
	return nil
}

// Register all OCP specific CSI tests into upstream testsuites.CSISuites.
func AddOpenShiftCSITests(filename string) (string, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	// Select sane defaults
	cfg := &OpenShiftCSIDriverConfig{
		LUNStressTest: &LUNStressTestConfig{
			MaxPodsPerNode: 32,
			PodsTotal:      260,
		},
	}
	if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), bytes, cfg); err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}

	testsuites.CSISuites = append(testsuites.CSISuites, initSCSILUNOverflowCSISuite(cfg.LUNStressTest))
	return cfg.Driver, nil
}
