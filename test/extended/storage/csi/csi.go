package csi

import (
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

const (
	// The defaul timeout for the LUN stress test.
	DefaultLUNStressTestTimeout = "40m"
	// The default nr. of Pods to run in the LUN stress test.
	DefaultLUNStressTestPodsTotal = 260
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
// All Pods are created at once. We expect that the CSI driver reports correct
// attach capacity and that the scheduler respects it*.
// Each pod does something very simple, like `ls /the/volume` and exits quickly.
// *) We know the scheduler does not respect the attach limit, see
// https://github.com/kubernetes/kubernetes/issues/126502
type LUNStressTestConfig struct {
	// How many Pods with one volume each to run in total. Set to 0 to disable the test.
	PodsTotal int
	// How long to wait for all Pods to start. 40 minutes by default.
	Timeout time.Duration
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
			PodsTotal: DefaultLUNStressTestPodsTotal,
			Timeout:   DefaultLUNStressTestTimeout,
		},
	}
	// No validation here, just like upstream.
	if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), bytes, cfg); err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}

	// Register this OCP specific test suite in the upstream test framework.
	// In the end, the test suite will be executed as any other upstream storage test.
	// Note: this must be done before external.AddDriverDefinition which actually goes through
	// the registered testsuites and generates ginkgo tests for them.
	testsuites.CSISuites = append(testsuites.CSISuites, initSCSILUNOverflowCSISuite(cfg.LUNStressTest))
	return cfg.Driver, nil
}
